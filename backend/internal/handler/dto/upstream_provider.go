package dto

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// UpstreamProvider 是上游供应商的响应 DTO。
//
// 安全约定：password/totp_secret/token 一律不出现在这里。管理员想改密码
// 只能重新填一次，不提供回显——回显等于给任何拿到管理端会话的人一份明文凭据。
// HasPassword/HasTotpSecret 只告诉前端「有没有配」，用于渲染占位提示。
type UpstreamProvider struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	BaseURL string  `json:"base_url"`
	Notes   *string `json:"notes"`

	Username      string `json:"username"`
	HasPassword   bool   `json:"has_password"`
	HasTotpSecret bool   `json:"has_totp_secret"`

	// RateCorrection 是充值比例修正系数，比价倍率 = 声明倍率 × 它。
	// 1.0 表示 1:1 充值、不做修正。
	RateCorrection float64 `json:"rate_correction"`

	// 上游账户信息（同步来的只读快照）
	Balance             *float64 `json:"balance"`
	FrozenBalance       *float64 `json:"frozen_balance"`
	UpstreamConcurrency *int     `json:"upstream_concurrency"`
	UpstreamUserID      string   `json:"upstream_user_id,omitempty"`

	Status        string     `json:"status"`
	LastSyncAt    *time.Time `json:"last_sync_at"`
	LastSyncError string     `json:"last_sync_error,omitempty"`
	SyncEnabled   bool       `json:"sync_enabled"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpstreamProviderWithStats 是列表页的聚合视图。
type UpstreamProviderWithStats struct {
	UpstreamProvider
	AccountCount int64 `json:"account_count"`
	GroupCount   int64 `json:"group_count"`
	// LocalCostUSD 是本地按该上游归集的实际花费（usage_logs.total_cost）
	LocalCostUSD  float64  `json:"local_cost_usd"`
	LocalRequests int64    `json:"local_requests"`
	MinRate       *float64 `json:"min_rate"`
	MaxRate       *float64 `json:"max_rate"`
}

// UpstreamGroup 是上游分组快照的响应 DTO。
type UpstreamGroup struct {
	ID                 int64  `json:"id"`
	UpstreamProviderID int64  `json:"upstream_provider_id"`
	RemoteGroupID      int64  `json:"remote_group_id"`
	Name               string `json:"name"`
	Platform           string `json:"platform"`
	SubscriptionType   string `json:"subscription_type"`

	RateMultiplier          float64  `json:"rate_multiplier"`
	EffectiveRateMultiplier *float64 `json:"effective_rate_multiplier"`
	// ComparableRate 是前端该用来排序比价的值：有专属倍率用它，否则用基础倍率
	ComparableRate     float64  `json:"comparable_rate"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakRateMultiplier *float64 `json:"peak_rate_multiplier"`
	PeakStart          string   `json:"peak_start,omitempty"`
	PeakEnd            string   `json:"peak_end,omitempty"`

	DailyLimitUSD   *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD  *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD *float64 `json:"monthly_limit_usd"`

	SyncedAt time.Time `json:"synced_at"`
}

// UpstreamGroupComparison 是跨上游拉平的分组比价视图。
//
// 带上游名称和余额：倍率再低，上游没余额也用不了，所以这两个要一起看。
type UpstreamGroupComparison struct {
	UpstreamGroup
	ProviderName        string   `json:"provider_name"`
	ProviderStatus      string   `json:"provider_status"`
	ProviderBalance     *float64 `json:"provider_balance"`
	ProviderSyncEnabled bool     `json:"provider_sync_enabled"`
	// ProviderRateCorrection 是该上游的充值比例修正系数
	ProviderRateCorrection float64 `json:"provider_rate_correction"`
	// CorrectedRate 是跨上游可比的真实成本倍率：声明倍率 × 修正系数。
	// 后端排序用的就是这个值，前端展示也该用它而不是 ComparableRate。
	CorrectedRate float64 `json:"corrected_rate"`
	// LocalAccountCount >0 说明这个价位本地已经在用了
	LocalAccountCount int64 `json:"local_account_count"`
}

// UpstreamProfile 是「测试连接」返回的上游账户信息。
type UpstreamProfile struct {
	Email         string  `json:"email"`
	Username      string  `json:"username"`
	Balance       float64 `json:"balance"`
	FrozenBalance float64 `json:"frozen_balance"`
	Concurrency   int     `json:"concurrency"`
	Status        string  `json:"status"`
}

func UpstreamProviderFromService(p *service.UpstreamProvider) *UpstreamProvider {
	if p == nil {
		return nil
	}
	return &UpstreamProvider{
		ID:                  p.ID,
		Name:                p.Name,
		BaseURL:             p.BaseURL,
		Notes:               p.Notes,
		Username:            p.Username,
		HasPassword:         p.Password != "",
		HasTotpSecret:       p.TotpSecret != "",
		RateCorrection:      service.NormalizeRateCorrection(p.RateCorrection),
		Balance:             p.Balance,
		FrozenBalance:       p.FrozenBalance,
		UpstreamConcurrency: p.UpstreamConcurrency,
		UpstreamUserID:      p.UpstreamUserID,
		Status:              p.Status,
		LastSyncAt:          p.LastSyncAt,
		LastSyncError:       p.LastSyncError,
		SyncEnabled:         p.SyncEnabled,
		CreatedAt:           p.CreatedAt,
		UpdatedAt:           p.UpdatedAt,
	}
}

func UpstreamProviderWithStatsFromService(p *service.UpstreamProviderWithStats) *UpstreamProviderWithStats {
	if p == nil {
		return nil
	}
	base := UpstreamProviderFromService(&p.UpstreamProvider)
	if base == nil {
		return nil
	}
	return &UpstreamProviderWithStats{
		UpstreamProvider: *base,
		AccountCount:     p.AccountCount,
		GroupCount:       p.GroupCount,
		LocalCostUSD:     p.LocalCostUSD,
		LocalRequests:    p.LocalRequests,
		MinRate:          p.MinRate,
		MaxRate:          p.MaxRate,
	}
}

func UpstreamGroupFromService(g *service.UpstreamGroup) *UpstreamGroup {
	if g == nil {
		return nil
	}
	return &UpstreamGroup{
		ID:                      g.ID,
		UpstreamProviderID:      g.UpstreamProviderID,
		RemoteGroupID:           g.RemoteGroupID,
		Name:                    g.Name,
		Platform:                g.Platform,
		SubscriptionType:        g.SubscriptionType,
		RateMultiplier:          g.RateMultiplier,
		EffectiveRateMultiplier: g.EffectiveRateMultiplier,
		ComparableRate:          g.ComparableRate(),
		PeakRateEnabled:         g.PeakRateEnabled,
		PeakRateMultiplier:      g.PeakRateMultiplier,
		PeakStart:               g.PeakStart,
		PeakEnd:                 g.PeakEnd,
		DailyLimitUSD:           g.DailyLimitUSD,
		WeeklyLimitUSD:          g.WeeklyLimitUSD,
		MonthlyLimitUSD:         g.MonthlyLimitUSD,
		SyncedAt:                g.SyncedAt,
	}
}

func UpstreamGroupComparisonFromService(
	g *service.UpstreamGroupComparison,
) *UpstreamGroupComparison {
	if g == nil {
		return nil
	}
	base := UpstreamGroupFromService(&g.UpstreamGroup)
	if base == nil {
		return nil
	}
	return &UpstreamGroupComparison{
		UpstreamGroup:          *base,
		ProviderName:           g.ProviderName,
		ProviderStatus:         g.ProviderStatus,
		ProviderBalance:        g.ProviderBalance,
		ProviderSyncEnabled:    g.ProviderSyncEnabled,
		ProviderRateCorrection: service.NormalizeRateCorrection(g.ProviderRateCorrection),
		CorrectedRate:          g.CorrectedRate(),
		LocalAccountCount:      g.LocalAccountCount,
	}
}

func UpstreamProfileFromService(p *service.UpstreamProfile) *UpstreamProfile {
	if p == nil {
		return nil
	}
	return &UpstreamProfile{
		Email:         p.Email,
		Username:      p.Username,
		Balance:       p.Balance,
		FrozenBalance: p.FrozenBalance,
		Concurrency:   p.Concurrency,
		Status:        p.Status,
	}
}
