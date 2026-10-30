package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lib/pq"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// --- Plan Repository ---

type scheduledTestPlanRepository struct {
	db *sql.DB
}

func NewScheduledTestPlanRepository(db *sql.DB) service.ScheduledTestPlanRepository {
	return &scheduledTestPlanRepository{db: db}
}

func (r *scheduledTestPlanRepository) Create(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_plans (account_id, model_id, reasoning_effort, prompt_text, expected_answer, cron_expression, enabled, max_results, auto_recover, temp_unschedulable_enabled, temp_unschedulable_error_codes, temp_unschedulable_minutes, failure_remove_group_id, failure_remove_group_ids, failure_disable, failure_temp_unschedulable_minutes, success_add_group_id, success_add_group_ids, success_enable, success_recover, next_run_at, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, NOW(), NOW())
		RETURNING id, account_id, model_id, reasoning_effort, prompt_text, expected_answer, cron_expression, enabled, max_results, auto_recover, temp_unschedulable_enabled, temp_unschedulable_error_codes, temp_unschedulable_minutes, failure_remove_group_id, failure_remove_group_ids, failure_disable, failure_temp_unschedulable_minutes, success_add_group_id, success_add_group_ids, success_enable, success_recover, last_run_at, next_run_at, created_at, updated_at
	`, plan.AccountID, plan.ModelID, plan.ReasoningEffort, plan.PromptText, plan.ExpectedAnswer, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.TempUnschedulableEnabled, pq.Array(plan.TempUnschedulableErrorCodes), plan.TempUnschedulableMinutes, plan.FailureRemoveGroupID, pq.Array(plan.FailureRemoveGroupIDs), plan.FailureDisable, plan.FailureTempUnschedulableMinutes, plan.SuccessAddGroupID, pq.Array(plan.SuccessAddGroupIDs), plan.SuccessEnable, plan.SuccessRecover, plan.NextRunAt)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) GetByID(ctx context.Context, id int64) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, account_id, model_id, reasoning_effort, prompt_text, expected_answer, cron_expression, enabled, max_results, auto_recover, temp_unschedulable_enabled, temp_unschedulable_error_codes, temp_unschedulable_minutes, failure_remove_group_id, failure_remove_group_ids, failure_disable, failure_temp_unschedulable_minutes, success_add_group_id, success_add_group_ids, success_enable, success_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE id = $1
	`, id)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) ListByAccountID(ctx context.Context, accountID int64) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, reasoning_effort, prompt_text, expected_answer, cron_expression, enabled, max_results, auto_recover, temp_unschedulable_enabled, temp_unschedulable_error_codes, temp_unschedulable_minutes, failure_remove_group_id, failure_remove_group_ids, failure_disable, failure_temp_unschedulable_minutes, success_add_group_id, success_add_group_ids, success_enable, success_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans WHERE account_id = $1
		ORDER BY created_at DESC
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) ListDue(ctx context.Context, now time.Time) ([]*service.ScheduledTestPlan, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, account_id, model_id, reasoning_effort, prompt_text, expected_answer, cron_expression, enabled, max_results, auto_recover, temp_unschedulable_enabled, temp_unschedulable_error_codes, temp_unschedulable_minutes, failure_remove_group_id, failure_remove_group_ids, failure_disable, failure_temp_unschedulable_minutes, success_add_group_id, success_add_group_ids, success_enable, success_recover, last_run_at, next_run_at, created_at, updated_at
		FROM scheduled_test_plans
		WHERE enabled = true AND next_run_at <= $1
		ORDER BY next_run_at ASC
	`, now)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanPlans(rows)
}

func (r *scheduledTestPlanRepository) Update(ctx context.Context, plan *service.ScheduledTestPlan) (*service.ScheduledTestPlan, error) {
	row := r.db.QueryRowContext(ctx, `
		UPDATE scheduled_test_plans
		SET model_id = $2, reasoning_effort = $3, prompt_text = $4, expected_answer = $5, cron_expression = $6, enabled = $7, max_results = $8, auto_recover = $9, temp_unschedulable_enabled = $10, temp_unschedulable_error_codes = $11, temp_unschedulable_minutes = $12, failure_remove_group_id = $13, failure_remove_group_ids = $14, failure_disable = $15, failure_temp_unschedulable_minutes = $16, success_add_group_id = $17, success_add_group_ids = $18, success_enable = $19, success_recover = $20, next_run_at = $21, updated_at = NOW()
		WHERE id = $1
		RETURNING id, account_id, model_id, reasoning_effort, prompt_text, expected_answer, cron_expression, enabled, max_results, auto_recover, temp_unschedulable_enabled, temp_unschedulable_error_codes, temp_unschedulable_minutes, failure_remove_group_id, failure_remove_group_ids, failure_disable, failure_temp_unschedulable_minutes, success_add_group_id, success_add_group_ids, success_enable, success_recover, last_run_at, next_run_at, created_at, updated_at
	`, plan.ID, plan.ModelID, plan.ReasoningEffort, plan.PromptText, plan.ExpectedAnswer, plan.CronExpression, plan.Enabled, plan.MaxResults, plan.AutoRecover, plan.TempUnschedulableEnabled, pq.Array(plan.TempUnschedulableErrorCodes), plan.TempUnschedulableMinutes, plan.FailureRemoveGroupID, pq.Array(plan.FailureRemoveGroupIDs), plan.FailureDisable, plan.FailureTempUnschedulableMinutes, plan.SuccessAddGroupID, pq.Array(plan.SuccessAddGroupIDs), plan.SuccessEnable, plan.SuccessRecover, plan.NextRunAt)
	return scanPlan(row)
}

func (r *scheduledTestPlanRepository) Delete(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_test_plans WHERE id = $1`, id)
	return err
}

func (r *scheduledTestPlanRepository) UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE scheduled_test_plans SET last_run_at = $2, next_run_at = $3, updated_at = NOW() WHERE id = $1
	`, id, lastRunAt, nextRunAt)
	return err
}

func (r *scheduledTestPlanRepository) ListPromptTemplates(ctx context.Context) ([]*service.ScheduledTestPromptTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, name, prompt_text, expected_answer, created_at, updated_at FROM scheduled_test_prompt_templates ORDER BY updated_at DESC, id DESC`)
	if err != nil { return nil, err }
	defer func() { _ = rows.Close() }()
	items := make([]*service.ScheduledTestPromptTemplate, 0)
	for rows.Next() {
		item := &service.ScheduledTestPromptTemplate{}
		if err := rows.Scan(&item.ID, &item.Name, &item.PromptText, &item.ExpectedAnswer, &item.CreatedAt, &item.UpdatedAt); err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *scheduledTestPlanRepository) CreatePromptTemplate(ctx context.Context, template *service.ScheduledTestPromptTemplate) (*service.ScheduledTestPromptTemplate, error) {
	item := &service.ScheduledTestPromptTemplate{}
	err := r.db.QueryRowContext(ctx, `INSERT INTO scheduled_test_prompt_templates (name, prompt_text, expected_answer) VALUES ($1, $2, $3) RETURNING id, name, prompt_text, expected_answer, created_at, updated_at`, template.Name, template.PromptText, template.ExpectedAnswer).
		Scan(&item.ID, &item.Name, &item.PromptText, &item.ExpectedAnswer, &item.CreatedAt, &item.UpdatedAt)
	if err != nil { return nil, err }
	return item, nil
}

func (r *scheduledTestPlanRepository) DeletePromptTemplate(ctx context.Context, id int64) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM scheduled_test_prompt_templates WHERE id = $1`, id)
	return err
}

// --- Result Repository ---

type scheduledTestResultRepository struct {
	db *sql.DB
}

func NewScheduledTestResultRepository(db *sql.DB) service.ScheduledTestResultRepository {
	return &scheduledTestResultRepository{db: db}
}

func (r *scheduledTestResultRepository) Create(ctx context.Context, result *service.ScheduledTestResult) (*service.ScheduledTestResult, error) {
	row := r.db.QueryRowContext(ctx, `
		INSERT INTO scheduled_test_results (plan_id, status, response_text, error_message, error_code, check_ok, latency_ms, started_at, finished_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW())
		RETURNING id, plan_id, status, response_text, error_message, error_code, check_ok, latency_ms, started_at, finished_at, created_at
	`, result.PlanID, result.Status, result.ResponseText, result.ErrorMessage, result.ErrorCode, result.CheckOK, result.LatencyMs, result.StartedAt, result.FinishedAt)

	out := &service.ScheduledTestResult{}
	if err := row.Scan(
		&out.ID, &out.PlanID, &out.Status, &out.ResponseText, &out.ErrorMessage, &out.ErrorCode,
		&out.CheckOK, &out.LatencyMs, &out.StartedAt, &out.FinishedAt, &out.CreatedAt,
	); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *scheduledTestResultRepository) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*service.ScheduledTestResult, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, plan_id, status, response_text, error_message, error_code, check_ok, latency_ms, started_at, finished_at, created_at
		FROM scheduled_test_results
		WHERE plan_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`, planID, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []*service.ScheduledTestResult
	for rows.Next() {
		r := &service.ScheduledTestResult{}
		if err := rows.Scan(
			&r.ID, &r.PlanID, &r.Status, &r.ResponseText, &r.ErrorMessage, &r.ErrorCode,
			&r.CheckOK, &r.LatencyMs, &r.StartedAt, &r.FinishedAt, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func (r *scheduledTestResultRepository) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM scheduled_test_results
		WHERE id IN (
			SELECT id FROM (
				SELECT id, ROW_NUMBER() OVER (PARTITION BY plan_id ORDER BY created_at DESC) AS rn
				FROM scheduled_test_results
				WHERE plan_id = $1
			) ranked
			WHERE rn > $2
		)
	`, planID, keepCount)
	return err
}

// --- scan helpers ---

type scannable interface {
	Scan(dest ...any) error
}

func scanPlan(row scannable) (*service.ScheduledTestPlan, error) {
	p := &service.ScheduledTestPlan{}
	if err := row.Scan(
		&p.ID, &p.AccountID, &p.ModelID, &p.ReasoningEffort, &p.PromptText, &p.ExpectedAnswer, &p.CronExpression, &p.Enabled, &p.MaxResults, &p.AutoRecover,
		&p.TempUnschedulableEnabled, pq.Array(&p.TempUnschedulableErrorCodes), &p.TempUnschedulableMinutes,
		&p.FailureRemoveGroupID, pq.Array(&p.FailureRemoveGroupIDs), &p.FailureDisable, &p.FailureTempUnschedulableMinutes, &p.SuccessAddGroupID, pq.Array(&p.SuccessAddGroupIDs), &p.SuccessEnable, &p.SuccessRecover,
		&p.LastRunAt, &p.NextRunAt, &p.CreatedAt, &p.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return p, nil
}

func scanPlans(rows *sql.Rows) ([]*service.ScheduledTestPlan, error) {
	var plans []*service.ScheduledTestPlan
	for rows.Next() {
		p, err := scanPlan(rows)
		if err != nil {
			return nil, err
		}
		plans = append(plans, p)
	}
	return plans, rows.Err()
}
