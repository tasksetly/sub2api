package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// --- Plan Repository ---

const fingerprintPlanColumns = `id, account_id, test_model, expected_model, api, challenge_count,
	cron_expression, enabled, max_results, consecutive_mismatches, mismatch_action_threshold,
	failure_disable, failure_temp_unschedulable_minutes, failure_remove_group_ids, success_add_group_ids,
	last_run_at, next_run_at, created_at, updated_at`

type fingerprintPlanRepository struct {
	db *sql.DB
}

func NewFingerprintPlanRepository(db *sql.DB) service.FingerprintPlanRepository {
	return &fingerprintPlanRepository{db: db}
}

func (r *fingerprintPlanRepository) Create(ctx context.Context, plan *service.FingerprintPlan) (*service.FingerprintPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO fingerprint_plans (account_id, test_model, expected_model, api, challenge_count,
			cron_expression, enabled, max_results, consecutive_mismatches, mismatch_action_threshold,
			failure_disable, failure_temp_unschedulable_minutes, failure_remove_group_ids, success_add_group_ids,
			next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, NOW(), NOW())
		RETURNING `+fingerprintPlanColumns+`
	`, plan.AccountID, plan.TestModel, plan.ExpectedModel, plan.API, plan.ChallengeCount,
		plan.CronExpression, plan.Enabled, plan.MaxResults, plan.ConsecutiveMismatches, plan.MismatchActionThreshold,
		plan.FailureDisable, plan.FailureTempUnschedulableMinutes, pq.Array(plan.FailureRemoveGroupIDs), pq.Array(plan.SuccessAddGroupIDs),
		plan.NextRunAt)
	return scanFingerprintPlan(row)
}

func (r *fingerprintPlanRepository) GetByID(ctx context.Context, id int64) (*service.FingerprintPlan, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+fingerprintPlanColumns+` FROM fingerprint_plans WHERE id = $1`, id)
	return scanFingerprintPlan(row)
}

func (r *fingerprintPlanRepository) ListByAccountID(ctx context.Context, accountID int64) ([]*service.FingerprintPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+fingerprintPlanColumns+` FROM fingerprint_plans WHERE account_id = $1 ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanFingerprintPlans(rows)
}

func (r *fingerprintPlanRepository) ListDue(ctx context.Context, now time.Time) ([]*service.FingerprintPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+fingerprintPlanColumns+`
		FROM fingerprint_plans
		WHERE enabled = true AND next_run_at <= $1
		ORDER BY next_run_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanFingerprintPlans(rows)
}

func (r *fingerprintPlanRepository) Update(ctx context.Context, plan *service.FingerprintPlan) (*service.FingerprintPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE fingerprint_plans
		SET test_model = $2, expected_model = $3, api = $4, challenge_count = $5,
			cron_expression = $6, enabled = $7, max_results = $8, mismatch_action_threshold = $9,
			failure_disable = $10, failure_temp_unschedulable_minutes = $11,
			failure_remove_group_ids = $12, success_add_group_ids = $13,
			next_run_at = $14, updated_at = NOW()
		WHERE id = $1
		RETURNING `+fingerprintPlanColumns+`
	`, plan.ID, plan.TestModel, plan.ExpectedModel, plan.API, plan.ChallengeCount,
		plan.CronExpression, plan.Enabled, plan.MaxResults, plan.MismatchActionThreshold,
		plan.FailureDisable, plan.FailureTempUnschedulableMinutes,
		pq.Array(plan.FailureRemoveGroupIDs), pq.Array(plan.SuccessAddGroupIDs),
		plan.NextRunAt)
	return scanFingerprintPlan(row)
}

func (r *fingerprintPlanRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM fingerprint_plans WHERE id = $1`, id)
	return err
}

func (r *fingerprintPlanRepository) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fingerprint_plans SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1
	`, id, lastRunAt, nextRunAt)
	return err
}

func (r *fingerprintPlanRepository) UpdateConsecutiveMismatches(ctx context.Context, planID int64, n int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE fingerprint_plans SET consecutive_mismatches = $2, updated_at = NOW() WHERE id = $1
	`, planID, n)
	return err
}

// --- Result Repository ---

const fingerprintResultColumns = `id, plan_id, verdict, decision, predicted_model, predicted_model_name,
	predicted_family, probability, analysis_json, error_message, skip_reason,
	estimated_output_tokens, latency_ms, started_at, finished_at, created_at`

type fingerprintResultRepository struct {
	db *sql.DB
}

func NewFingerprintResultRepository(db *sql.DB) service.FingerprintResultRepository {
	return &fingerprintResultRepository{db: db}
}

func (r *fingerprintResultRepository) Create(ctx context.Context, result *service.FingerprintResult) (*service.FingerprintResult, error) {
	// analysis_json 是 JSONB，空字符串不是合法 JSON，必须写 NULL。
	var analysis any
	if result.AnalysisJSON != "" {
		analysis = result.AnalysisJSON
	}
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO fingerprint_results (plan_id, verdict, decision, predicted_model, predicted_model_name,
			predicted_family, probability, analysis_json, error_message, skip_reason,
			estimated_output_tokens, latency_ms, started_at, finished_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, NOW())
		RETURNING `+fingerprintResultColumns+`
	`, result.PlanID, result.Verdict, result.Decision, result.PredictedModel, result.PredictedModelName,
		result.PredictedFamily, result.Probability, analysis, result.ErrorMessage, result.SkipReason,
		result.EstimatedOutputTokens, result.LatencyMs, result.StartedAt, result.FinishedAt)
	return scanFingerprintResult(row)
}

func (r *fingerprintResultRepository) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*service.FingerprintResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT `+fingerprintResultColumns+`
		FROM fingerprint_results
		WHERE plan_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, planID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	results := make([]*service.FingerprintResult, 0)
	for rows.Next() {
		item, err := scanFingerprintResult(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, item)
	}
	return results, rows.Err()
}

func (r *fingerprintResultRepository) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM fingerprint_results
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY plan_id ORDER BY created_at DESC) AS rn
				FROM fingerprint_results
				WHERE plan_id = $1
			) ranked
			WHERE rn > $2
		)
	`, planID, keepCount)
	return err
}

func (r *fingerprintResultRepository) SumEstimatedTokensSince(ctx context.Context, accountID int64, since time.Time) (int64, error) {
	var total sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(fr.estimated_output_tokens), 0)
		FROM fingerprint_results fr
		JOIN fingerprint_plans fp ON fp.id = fr.plan_id
		WHERE fp.account_id = $1 AND fr.created_at >= $2
	`, accountID, since).Scan(&total)
	if err != nil {
		return 0, err
	}
	return total.Int64, nil
}

func (r *fingerprintResultRepository) CountSince(ctx context.Context, accountID int64, since time.Time) (int64, error) {
	var count int64
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM fingerprint_results fr
		JOIN fingerprint_plans fp ON fp.id = fr.plan_id
		WHERE fp.account_id = $1 AND fr.created_at >= $2 AND fr.skip_reason = ''
	`, accountID, since).Scan(&count)
	return count, err
}

// --- scan helpers ---

func scanFingerprintPlan(row scannable) (*service.FingerprintPlan, error) {
	p := &service.FingerprintPlan{}
	if err := row.Scan(
		&p.ID, &p.AccountID, &p.TestModel, &p.ExpectedModel, &p.API, &p.ChallengeCount,
		&p.CronExpression, &p.Enabled, &p.MaxResults, &p.ConsecutiveMismatches, &p.MismatchActionThreshold,
		&p.FailureDisable, &p.FailureTempUnschedulableMinutes, pq.Array(&p.FailureRemoveGroupIDs), pq.Array(&p.SuccessAddGroupIDs),
		&p.LastRunAt, &p.NextRunAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return p, nil
}

func scanFingerprintPlans(rows *sql.Rows) ([]*service.FingerprintPlan, error) {
	plans := make([]*service.FingerprintPlan, 0)
	for rows.Next() {
		p, err := scanFingerprintPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}

func scanFingerprintResult(row scannable) (*service.FingerprintResult, error) {
	item := &service.FingerprintResult{}
	var analysis sql.NullString
	if err := row.Scan(
		&item.ID, &item.PlanID, &item.Verdict, &item.Decision, &item.PredictedModel, &item.PredictedModelName,
		&item.PredictedFamily, &item.Probability, &analysis, &item.ErrorMessage, &item.SkipReason,
		&item.EstimatedOutputTokens, &item.LatencyMs, &item.StartedAt, &item.FinishedAt, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.AnalysisJSON = analysis.String
	return item, nil
}
