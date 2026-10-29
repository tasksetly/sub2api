package service

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"
)

// UpstreamRechargeOrder is a normalized order, never a local payment order.
// CreditUSD is nil when a NewAPI payment plugin uses an undocumented unit.
type UpstreamRechargeOrder struct {
	ID        string   `json:"id"`
	CreditUSD *float64 `json:"credit_usd"`
	Paid      *float64 `json:"paid"`
	Currency  string   `json:"currency"`
	Valid     bool     `json:"valid"`
}

// UpstreamFinance retains observations across NewAPI's rolling history window.
// It contains no credentials. Only the explicit summary DTO is sent to clients.
type UpstreamFinance struct {
	ProviderType   string                  `json:"provider_type"`
	BaseURL        string                  `json:"base_url"`
	UserID         int64                   `json:"user_id"`
	Orders         []UpstreamRechargeOrder `json:"orders"`
	OrdersSyncedAt *time.Time              `json:"orders_synced_at"`
	OrdersError    string                  `json:"orders_error"`
	// Coverage is deliberately not called lifetime for NewAPI.
	Coverage      string     `json:"coverage"`
	UsedUSD       *float64   `json:"used_usd"`
	UsageSyncedAt *time.Time `json:"usage_synced_at"`
	UsageError    string     `json:"usage_error"`
	SyncedAt      time.Time  `json:"synced_at"`
}

type UpstreamFinanceSummary struct {
	OrderCount          int                `json:"order_count"`
	RechargeUSD         float64            `json:"recharge_usd"`
	UnknownCreditOrders int                `json:"unknown_credit_orders"`
	UnknownPaidOrders   int                `json:"unknown_paid_orders"`
	PaidAmounts         map[string]float64 `json:"paid_amounts"`
	OrdersSyncedAt      *time.Time         `json:"orders_synced_at"`
	OrdersError         string             `json:"orders_error,omitempty"`
	Coverage            string             `json:"coverage"`
	UsedUSD             *float64           `json:"used_usd"`
	UsageSyncedAt       *time.Time         `json:"usage_synced_at"`
	UsageError          string             `json:"usage_error,omitempty"`
	// Costs use the administrator's existing recharge conversion, not FX guesses.
	ConsumedCostUSD   *float64 `json:"consumed_cost_usd"`
	GrossProfitUSD    *float64 `json:"gross_profit_usd"`
	CostDifferenceUSD *float64 `json:"cost_difference_usd"`
}

func (p *UpstreamProviderWithStats) FinanceSummary() *UpstreamFinanceSummary {
	f := p.Finance
	if f == nil {
		return nil
	}
	summary := &UpstreamFinanceSummary{
		PaidAmounts: map[string]float64{}, Coverage: f.Coverage,
		OrdersSyncedAt: f.OrdersSyncedAt, OrdersError: f.OrdersError,
		UsedUSD: f.UsedUSD, UsageSyncedAt: f.UsageSyncedAt, UsageError: f.UsageError,
	}
	for _, order := range f.Orders {
		if !order.Valid {
			continue
		}
		summary.OrderCount++
		if order.CreditUSD != nil {
			summary.RechargeUSD += *order.CreditUSD
		} else {
			summary.UnknownCreditOrders++
		}
		if order.Paid != nil {
			summary.PaidAmounts[order.Currency] += *order.Paid
		} else {
			summary.UnknownPaidOrders++
		}
	}
	// A failed refresh keeps the last good figures visible but cannot produce
	// a current profit or reconciliation result. Zero consumption is valid.
	if f.UsedUSD != nil && f.UsageError == "" && p.LastSyncError == "" {
		cost := *f.UsedUSD * NormalizeRateCorrection(p.RateCorrection)
		profit := p.LocalUserBilledUSD - cost
		difference := p.LocalCostUSD - cost
		summary.ConsumedCostUSD, summary.GrossProfitUSD, summary.CostDifferenceUSD = &cost, &profit, &difference
	}
	return summary
}

// syncFinance is optional: old upstream versions or a PAT without billing scope
// must not prevent balance/group synchronization. Each failure is persisted.
func (s *UpstreamProviderService) syncFinance(ctx context.Context, p *UpstreamProvider, token string, profile *UpstreamProfile) *UpstreamFinance {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	f := UpstreamFinance{ProviderType: p.ProviderType, BaseURL: p.BaseURL, UserID: profile.UserID, SyncedAt: time.Now()}
	if old := p.Finance; old != nil && old.ProviderType == f.ProviderType && old.BaseURL == f.BaseURL && old.UserID == f.UserID {
		f = *old
		f.SyncedAt = time.Now()
	}
	f.OrdersError, f.UsageError = "", ""
	f.Coverage = "upstream_history"
	if p.ProviderType == UpstreamProviderTypeNewAPI {
		f.Coverage = "observed_history"
		if !isNewAPIDashboardToken(token) {
			f.OrdersError = "NewAPI recharge orders require a dashboard PAT with wallet:read; relay keys cannot read account orders"
			f.UsageError = "NewAPI relay key usage is not account-wide consumption; use a dashboard PAT"
			return &f
		}
	}
	if profile.UserID <= 0 {
		f.OrdersError = "upstream did not report a user identity; recharge history cannot be associated safely"
		f.UsageError = f.OrdersError
		return &f
	}
	orders, err := s.client.ListRechargeOrders(ctx, p.BaseURL, token, p.ProviderType)
	if err != nil {
		f.OrdersError = err.Error()
	} else {
		if p.ProviderType == UpstreamProviderTypeNewAPI {
			// Upsert by remote ID, including invalid statuses: repeated syncs
			// cannot double count, and a changed order replaces its old state.
			byID := make(map[string]UpstreamRechargeOrder, len(f.Orders)+len(orders))
			for _, order := range f.Orders {
				byID[order.ID] = order
			}
			for _, order := range orders {
				byID[order.ID] = order
			}
			orders = make([]UpstreamRechargeOrder, 0, len(byID))
			for _, order := range byID {
				orders = append(orders, order)
			}
			sort.Slice(orders, func(i, j int) bool { return orders[i].ID < orders[j].ID })
		}
		f.Orders = orders
		now := time.Now()
		f.OrdersSyncedAt = &now
	}
	var used *float64
	if p.ProviderType == UpstreamProviderTypeNewAPI {
		used = profile.UsedUSD
	} else {
		used, err = s.client.GetConsumedUSD(ctx, p.BaseURL, token)
	}
	if err != nil && p.ProviderType != UpstreamProviderTypeNewAPI {
		f.UsageError = err.Error()
	} else if used == nil {
		f.UsageError = "upstream did not report account-wide consumption"
	} else {
		f.UsedUSD = used
		now := time.Now()
		f.UsageSyncedAt = &now
	}
	return &f
}

func (c *UpstreamProviderClient) GetConsumedUSD(ctx context.Context, baseURL, token string) (*float64, error) {
	// The ordinary /usage/stats endpoint defaults to a recent date range.
	// Dashboard total_actual_cost is the account-wide cumulative deduction.
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/v1/usage/dashboard/stats", token, nil)
	if err != nil {
		return nil, err
	}
	var stats struct {
		Total *float64 `json:"total_actual_cost"`
	}
	if err := json.Unmarshal(body, &stats); err != nil {
		return nil, fmt.Errorf("parse upstream consumption: %w", err)
	}
	if stats.Total == nil || !validUpstreamMoney(*stats.Total) {
		return nil, fmt.Errorf("upstream did not report valid cumulative consumption")
	}
	return stats.Total, nil
}

func validUpstreamMoney(v float64) bool { return v >= 0 && !math.IsNaN(v) && !math.IsInf(v, 0) }

// ListRechargeOrders reads every available page before returning any result.
// Caps and pagination inconsistencies are errors, never a partial zero total.
func (c *UpstreamProviderClient) ListRechargeOrders(ctx context.Context, baseURL, token, providerType string) ([]UpstreamRechargeOrder, error) {
	orders := make([]UpstreamRechargeOrder, 0)
	seen := map[string]bool{}
	expectedTotal := -1
	for page := 1; page <= 100; page++ {
		path := fmt.Sprintf("/api/v1/payment/orders/my?page=%d&page_size=100&order_type=balance", page)
		if providerType == UpstreamProviderTypeNewAPI {
			path = fmt.Sprintf("/api/user/topup/self?p=%d&page_size=100&size=100", page)
		}
		body, err := c.do(ctx, baseURL, http.MethodGet, path, token, nil)
		if err != nil {
			return nil, fmt.Errorf("fetch recharge orders: %w", err)
		}
		var result struct {
			Items []json.RawMessage `json:"items"`
			Total *int              `json:"total"`
		}
		if err := json.Unmarshal(body, &result); err != nil {
			return nil, fmt.Errorf("parse recharge orders: %w", err)
		}
		if result.Total == nil || *result.Total < 0 || result.Items == nil {
			return nil, fmt.Errorf("upstream recharge pagination is missing items or total")
		}
		if expectedTotal < 0 {
			expectedTotal = *result.Total
		}
		if expectedTotal != *result.Total {
			return nil, fmt.Errorf("upstream recharge history changed during pagination; retry synchronization")
		}
		for _, raw := range result.Items {
			order, err := parseUpstreamRechargeOrder(raw, providerType)
			if err != nil {
				return nil, err
			}
			if seen[order.ID] {
				return nil, fmt.Errorf("upstream recharge pagination repeated an order; retry synchronization")
			}
			seen[order.ID] = true
			orders = append(orders, order)
		}
		if len(orders) == expectedTotal {
			return orders, nil
		}
		if len(result.Items) == 0 || len(orders) > expectedTotal {
			return nil, fmt.Errorf("upstream recharge pagination is incomplete")
		}
	}
	return nil, fmt.Errorf("upstream recharge history exceeds 100 pages")
}

func parseUpstreamRechargeOrder(raw json.RawMessage, providerType string) (UpstreamRechargeOrder, error) {
	var row struct {
		ID              json.Number `json:"id"`
		Amount          *float64    `json:"amount"`
		PayAmount       *float64    `json:"pay_amount"`
		Money           *float64    `json:"money"`
		RefundAmount    float64     `json:"refund_amount"`
		Status          string      `json:"status"`
		OrderType       string      `json:"order_type"`
		Currency        string      `json:"currency"`
		PaymentMethod   string      `json:"payment_method"`
		PaymentProvider string      `json:"payment_provider"`
	}
	if err := json.Unmarshal(raw, &row); err != nil {
		return UpstreamRechargeOrder{}, fmt.Errorf("parse recharge order: %w", err)
	}
	id, err := row.ID.Int64()
	if err != nil || id <= 0 {
		return UpstreamRechargeOrder{}, fmt.Errorf("upstream recharge order is missing a valid id")
	}
	order := UpstreamRechargeOrder{ID: row.ID.String(), Currency: strings.ToUpper(strings.TrimSpace(row.Currency))}
	if providerType == UpstreamProviderTypeNewAPI {
		order.Valid = strings.EqualFold(strings.TrimSpace(row.Status), "success")
		if !order.Valid {
			return order, nil
		}
		if row.Amount == nil || row.Money == nil || !validUpstreamMoney(*row.Amount) || !validUpstreamMoney(*row.Money) {
			return order, fmt.Errorf("NewAPI order %s has invalid amounts", order.ID)
		}
		channel := strings.ToLower(strings.TrimSpace(row.PaymentProvider))
		if channel == "" {
			channel = strings.ToLower(strings.TrimSpace(row.PaymentMethod))
		}
		if order.Currency == "" {
			order.Currency = "UNSPECIFIED/" + channel
		}
		order.Paid = row.Money
		credit := *row.Amount
		switch channel {
		case "stripe":
			// Stripe money is both the charged and credited USD amount in NewAPI.
			credit = *row.Money
		case "creem":
			credit /= NewAPIQuotaPerUnit
		case "epay", "alipay", "wxpay", "waffo":
		default:
			return order, nil // Plugin amount units cannot be inferred safely.
		}
		order.CreditUSD = &credit
		return order, nil
	}
	if row.OrderType != "balance" {
		return order, nil
	}
	switch strings.ToUpper(strings.TrimSpace(row.Status)) {
	case OrderStatusCompleted, OrderStatusRefundFailed, OrderStatusPartiallyRefunded, OrderStatusRefunded:
		order.Valid = true
	default:
		return order, nil // PAID/RECHARGING have not yet credited the wallet.
	}
	if row.Amount == nil || row.PayAmount == nil || !validUpstreamMoney(*row.Amount) || !validUpstreamMoney(*row.PayAmount) || !validUpstreamMoney(row.RefundAmount) {
		return order, fmt.Errorf("sub2api order %s has invalid amounts", order.ID)
	}
	credit := *row.Amount
	paid := *row.PayAmount
	order.Paid = &paid
	status := strings.ToUpper(strings.TrimSpace(row.Status))
	if status == OrderStatusRefunded {
		order.Valid = false
		return order, nil
	}
	if status == OrderStatusPartiallyRefunded {
		if credit <= 0 || row.RefundAmount > credit {
			return order, fmt.Errorf("sub2api order %s has invalid refund amount", order.ID)
		}
		paid *= (credit - row.RefundAmount) / credit
		credit -= row.RefundAmount
	}
	order.CreditUSD = &credit
	if order.Currency == "" {
		order.Currency = "UNSPECIFIED"
	}
	return order, nil
}
