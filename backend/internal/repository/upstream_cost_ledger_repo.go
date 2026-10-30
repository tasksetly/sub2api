package repository

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/pkg/usagestats"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// SyncUpstreamCostLedger rebuilds the affected local-date buckets from the
// immutable entry table. Rebuilding a bounded window makes retries idempotent
// and also captures late-arriving entries without ever touching usage_logs.
func (r *dashboardAggregationRepository) SyncUpstreamCostLedger(ctx context.Context, now time.Time) error {
	if r == nil || r.sql == nil {
		return nil
	}
	db, ok := r.sql.(*sql.DB)
	if !ok {
		return r.syncUpstreamCostLedgerInTx(ctx, r.sql, now)
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := r.syncUpstreamCostLedgerInTx(ctx, tx, now); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *dashboardAggregationRepository) syncUpstreamCostLedgerInTx(ctx context.Context, q sqlExecutor, now time.Time) error {
	var watermark time.Time
	if err := scanSingleRow(ctx, q, `SELECT watermark FROM upstream_cost_ledger_state WHERE id = 1 FOR UPDATE`, nil, &watermark); err != nil {
		return fmt.Errorf("lock upstream cost ledger state: %w", err)
	}
	start := watermark.UTC().Add(-48 * time.Hour)
	if watermark.IsZero() || watermark.Equal(time.Unix(0, 0).UTC()) {
		start = time.Unix(0, 0).UTC()
	}
	end := now.UTC()
	if !end.After(start) {
		return nil
	}
	loc := timezone.Location()
	if start.After(time.Unix(0, 0).UTC()) {
		localStart := start.In(loc)
		start = time.Date(localStart.Year(), localStart.Month(), localStart.Day(), 0, 0, 0, 0, loc).UTC()
	}
	startDate := start.In(loc).Format("2006-01-02")
	endDate := end.In(loc).AddDate(0, 0, 1).Format("2006-01-02")
	if _, err := q.ExecContext(ctx, `DELETE FROM upstream_cost_daily_rollups WHERE bucket_date >= $1::date AND bucket_date < $2::date`, startDate, endDate); err != nil {
		return fmt.Errorf("clear upstream cost ledger window: %w", err)
	}
	_, err := q.ExecContext(ctx, `
		INSERT INTO upstream_cost_daily_rollups (
			bucket_date, dimension_hash, provider_id, provider_name, provider_type, supplier, model,
			user_id, api_key_id, account_id, group_id, request_type, stream, openai_ws_mode, billing_type,
			billing_mode, image_count, requests, input_tokens, output_tokens, cache_tokens,
			standard_cost, account_cost, user_billed, computed_at
		)
		SELECT
			(e.created_at AT TIME ZONE $3)::date,
			md5(jsonb_build_array(e.provider_id, e.provider_name, e.provider_type, e.supplier, e.model,
				e.user_id, e.api_key_id, e.account_id, e.group_id, e.request_type, e.stream,
				e.openai_ws_mode, e.billing_type, e.billing_mode, e.image_count)::text),
			e.provider_id, e.provider_name, e.provider_type, e.supplier, e.model,
			e.user_id, e.api_key_id, e.account_id, e.group_id, e.request_type, e.stream,
			e.openai_ws_mode, e.billing_type, e.billing_mode, e.image_count,
			COUNT(*), COALESCE(SUM(e.input_tokens), 0), COALESCE(SUM(e.output_tokens), 0),
			COALESCE(SUM(e.cache_tokens), 0), COALESCE(SUM(e.standard_cost), 0),
			COALESCE(SUM(e.account_cost), 0), COALESCE(SUM(e.user_billed), 0), NOW()
		FROM upstream_cost_entries e
		WHERE e.created_at >= $1 AND e.created_at < $2
		GROUP BY 1, e.provider_id, e.provider_name, e.provider_type, e.supplier, e.model,
			e.user_id, e.api_key_id, e.account_id, e.group_id, e.request_type, e.stream, e.openai_ws_mode,
			e.billing_type, e.billing_mode, e.image_count
	`, start, end, timezone.Name())
	if err != nil {
		return fmt.Errorf("rebuild upstream cost ledger: %w", err)
	}
	if _, err := q.ExecContext(ctx, `UPDATE upstream_cost_ledger_state SET watermark = $1, updated_at = NOW() WHERE id = 1`, end); err != nil {
		return fmt.Errorf("update upstream cost ledger state: %w", err)
	}
	return nil
}

func (r *usageLogRepository) GetSupplierCostStatsFromLedger(ctx context.Context, filters usagestats.UsageLogFilters) ([]usagestats.SupplierCostStat, error) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 10)
	add := func(condition string, arg any) {
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
		args = append(args, arg)
	}
	if filters.UserID > 0 { add("r.user_id = $%d", filters.UserID) }
	if filters.APIKeyID > 0 { add("r.api_key_id = $%d", filters.APIKeyID) }
	if filters.AccountID > 0 { add("r.account_id = $%d", filters.AccountID) }
	if filters.GroupID > 0 { add("r.group_id = $%d", filters.GroupID) }
	if filters.Supplier != nil { add("r.supplier = $%d", *filters.Supplier) }
	if strings.TrimSpace(filters.Model) != "" { add("r.model = $%d", filters.Model) }
	if filters.RequestType != nil {
		condition, conditionArgs := buildRequestTypeFilterConditionWithAlias(len(args)+1, *filters.RequestType, "r")
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	} else if filters.Stream != nil {
		add("r.stream = $%d", *filters.Stream)
	}
	if filters.BillingType != nil { add("r.billing_type = $%d", *filters.BillingType) }
	conditions, args = appendUsageLogBillingModeWhereConditionWithAlias(conditions, args, filters.BillingMode, "r")
	tzArg := 0
	addDate := func(condition string, value time.Time) {
		if tzArg == 0 {
			tzArg = len(args) + 1
			args = append(args, timezone.Name())
		}
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1, tzArg))
		args = append(args, value)
	}
	if filters.StartTime != nil { addDate("r.bucket_date >= ($%d AT TIME ZONE $%d)::date", *filters.StartTime) }
	if filters.EndTime != nil { addDate("r.bucket_date < ($%d AT TIME ZONE $%d)::date", *filters.EndTime) }
	query := fmt.Sprintf(`
		WITH grouped AS (
			SELECT r.supplier, COUNT(DISTINCT NULLIF(r.account_id, 0)) AS account_count,
			COALESCE(SUM(r.requests), 0)::bigint AS requests,
			COALESCE(SUM(r.input_tokens), 0)::bigint AS input_tokens,
			COALESCE(SUM(r.output_tokens), 0)::bigint AS output_tokens,
			COALESCE(SUM(r.cache_tokens), 0)::bigint AS cache_tokens,
			COALESCE(SUM(r.input_tokens + r.output_tokens + r.cache_tokens), 0)::bigint AS total_tokens,
			SUM(r.standard_cost) AS standard_cost, SUM(r.account_cost) AS account_cost,
			SUM(r.user_billed) AS user_billed
			FROM upstream_cost_daily_rollups r WHERE %s GROUP BY r.supplier
		)
		SELECT supplier, account_count, requests, input_tokens, output_tokens, cache_tokens,
			total_tokens, standard_cost, account_cost, user_billed,
			user_billed - account_cost,
			CASE WHEN user_billed > 0 THEN (user_billed - account_cost) / user_billed ELSE 0 END,
			CASE WHEN SUM(account_cost) OVER () > 0 THEN account_cost / SUM(account_cost) OVER () ELSE 0 END
		FROM grouped ORDER BY account_cost DESC, supplier ASC`, strings.Join(conditions, " AND "))
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	results := make([]usagestats.SupplierCostStat, 0)
	for rows.Next() {
		var row usagestats.SupplierCostStat
		if err := rows.Scan(&row.Supplier, &row.AccountCount, &row.Requests, &row.InputTokens, &row.OutputTokens, &row.CacheTokens, &row.TotalTokens, &row.StandardCost, &row.AccountCost, &row.UserBilled, &row.GrossProfit, &row.GrossMargin, &row.CostPercentage); err != nil { return nil, err }
		results = append(results, row)
	}
	return results, rows.Err()
}

func (r *usageLogRepository) GetUpstreamCostDailyFromLedger(ctx context.Context, filters usagestats.UsageLogFilters) ([]usagestats.UpstreamCostDailyStat, error) {
	conditions := []string{"1 = 1"}
	args := make([]any, 0, 8)
	add := func(condition string, arg any) {
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1))
		args = append(args, arg)
	}
	if filters.UserID > 0 { add("r.user_id = $%d", filters.UserID) }
	if filters.APIKeyID > 0 { add("r.api_key_id = $%d", filters.APIKeyID) }
	if filters.AccountID > 0 { add("r.account_id = $%d", filters.AccountID) }
	if filters.GroupID > 0 { add("r.group_id = $%d", filters.GroupID) }
	if filters.Supplier != nil { add("r.supplier = $%d", *filters.Supplier) }
	if strings.TrimSpace(filters.Model) != "" { add("r.model = $%d", filters.Model) }
	if filters.RequestType != nil {
		condition, conditionArgs := buildRequestTypeFilterConditionWithAlias(len(args)+1, *filters.RequestType, "r")
		conditions = append(conditions, condition)
		args = append(args, conditionArgs...)
	} else if filters.Stream != nil {
		add("r.stream = $%d", *filters.Stream)
	}
	if filters.BillingType != nil { add("r.billing_type = $%d", *filters.BillingType) }
	conditions, args = appendUsageLogBillingModeWhereConditionWithAlias(conditions, args, filters.BillingMode, "r")
	tzArg := 0
	addDate := func(condition string, value time.Time) {
		if tzArg == 0 { tzArg = len(args) + 1; args = append(args, timezone.Name()) }
		conditions = append(conditions, fmt.Sprintf(condition, len(args)+1, tzArg))
		args = append(args, value)
	}
	if filters.StartTime != nil { addDate("r.bucket_date >= ($%d AT TIME ZONE $%d)::date", *filters.StartTime) }
	if filters.EndTime != nil { addDate("r.bucket_date < ($%d AT TIME ZONE $%d)::date", *filters.EndTime) }
	query := fmt.Sprintf(`
		SELECT r.bucket_date::text, r.provider_id, r.provider_name, r.provider_type, r.model,
		       COALESCE(SUM(r.requests), 0)::bigint, COALESCE(SUM(r.input_tokens), 0)::bigint,
		       COALESCE(SUM(r.output_tokens), 0)::bigint, COALESCE(SUM(r.cache_tokens), 0)::bigint,
		       SUM(r.account_cost), SUM(r.user_billed), SUM(r.user_billed) - SUM(r.account_cost),
		       CASE WHEN SUM(r.user_billed) > 0 THEN (SUM(r.user_billed) - SUM(r.account_cost)) / SUM(r.user_billed) ELSE 0 END
		FROM upstream_cost_daily_rollups r
		WHERE %s
		GROUP BY r.bucket_date, r.provider_id, r.provider_name, r.provider_type, r.model
		ORDER BY r.bucket_date DESC, r.provider_name ASC, r.model ASC`, strings.Join(conditions, " AND "))
	rows, err := r.sql.QueryContext(ctx, query, args...)
	if err != nil { return nil, err }
	defer rows.Close()
	results := make([]usagestats.UpstreamCostDailyStat, 0)
	for rows.Next() {
		var row usagestats.UpstreamCostDailyStat
		if err := rows.Scan(&row.Date, &row.ProviderID, &row.ProviderName, &row.ProviderType, &row.Model, &row.Requests, &row.InputTokens, &row.OutputTokens, &row.CacheTokens, &row.AccountCost, &row.UserBilled, &row.GrossProfit, &row.GrossMargin); err != nil { return nil, err }
		results = append(results, row)
	}
	return results, rows.Err()
}

var _ service.UpstreamCostLedgerSyncRepository = (*dashboardAggregationRepository)(nil)
var _ service.UpstreamCostSupplierRepository = (*usageLogRepository)(nil)
