package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type intelligenceTestRepository struct{ db *sql.DB }

func NewIntelligenceTestRepository(db *sql.DB) service.IntelligenceTestRepository {
	return &intelligenceTestRepository{db: db}
}

const intelligenceTestConfigColumns = `q.id, q.name, q.model_id, q.question_text, q.api_key_id,
 q.cron_expression, q.keep_count, q.enabled, q.last_run_at, q.next_run_at, q.created_at, q.updated_at,
 COALESCE(k.name, ''), k.group_id, COALESCE(g.name, '')`

func scanIntelligenceTestConfig(scanner interface{ Scan(...any) error }) (*service.IntelligenceTestConfig, error) {
	c := &service.IntelligenceTestConfig{}
	var apiKeyID, groupID sql.NullInt64
	if err := scanner.Scan(&c.ID, &c.Name, &c.ModelID, &c.PromptText, &apiKeyID, &c.CronExpression,
		&c.KeepCount, &c.Enabled, &c.LastRunAt, &c.NextRunAt, &c.CreatedAt, &c.UpdatedAt,
		&c.APIKeyName, &groupID, &c.GroupName); err != nil {
		return nil, err
	}
	if apiKeyID.Valid { c.APIKeyID = apiKeyID.Int64 }
	if groupID.Valid { c.GroupID = &groupID.Int64 }
	return c, nil
}

func (r *intelligenceTestRepository) ListConfigs(ctx context.Context) ([]*service.IntelligenceTestConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+intelligenceTestConfigColumns+` FROM intelligence_test_questions q
LEFT JOIN api_keys k ON k.id = q.api_key_id
LEFT JOIN groups g ON g.id = k.group_id
ORDER BY q.updated_at DESC, q.id DESC`)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]*service.IntelligenceTestConfig, 0)
	for rows.Next() {
		item, err := scanIntelligenceTestConfig(rows)
		if err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *intelligenceTestRepository) GetConfig(ctx context.Context, id int64) (*service.IntelligenceTestConfig, error) {
	item, err := scanIntelligenceTestConfig(r.db.QueryRowContext(ctx, `SELECT `+intelligenceTestConfigColumns+` FROM intelligence_test_questions q
LEFT JOIN api_keys k ON k.id = q.api_key_id
LEFT JOIN groups g ON g.id = k.group_id WHERE q.id = $1`, id))
	if err == sql.ErrNoRows { return nil, service.ErrIntelligenceTestConfigNotFound }
	return item, err
}

func (r *intelligenceTestRepository) CreateConfig(ctx context.Context, c *service.IntelligenceTestConfig) (*service.IntelligenceTestConfig, error) {
	row := r.db.QueryRowContext(ctx, `INSERT INTO intelligence_test_questions
(name, model_id, question_text, answer_mode, strength, api_key_id, cron_expression, keep_count, enabled, next_run_at)
VALUES ($1, $2, $3, 'svg', 'medium', $4, $5, $6, $7, $8) RETURNING id`, c.Name, c.ModelID, c.PromptText,
		intelligenceTestNullInt64(c.APIKeyID), c.CronExpression, c.KeepCount, c.Enabled, c.NextRunAt)
	if err := row.Scan(&c.ID); err != nil { return nil, err }
	return r.GetConfig(ctx, c.ID)
}

func (r *intelligenceTestRepository) UpdateConfig(ctx context.Context, c *service.IntelligenceTestConfig) (*service.IntelligenceTestConfig, error) {
	result, err := r.db.ExecContext(ctx, `UPDATE intelligence_test_questions SET name=$2, model_id=$3, question_text=$4,
api_key_id=$5, cron_expression=$6, keep_count=$7, enabled=$8, updated_at=NOW() WHERE id=$1`, c.ID, c.Name,
		c.ModelID, c.PromptText, intelligenceTestNullInt64(c.APIKeyID), c.CronExpression, c.KeepCount, c.Enabled)
	if err != nil { return nil, err }
	count, err := result.RowsAffected()
	if err != nil { return nil, err }
	if count == 0 { return nil, service.ErrIntelligenceTestConfigNotFound }
	return r.GetConfig(ctx, c.ID)
}

func (r *intelligenceTestRepository) DeleteConfig(ctx context.Context, id int64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM intelligence_test_questions WHERE id=$1`, id)
	if err != nil { return err }
	count, err := result.RowsAffected()
	if err != nil { return err }
	if count == 0 { return service.ErrIntelligenceTestConfigNotFound }
	return nil
}

func (r *intelligenceTestRepository) ListDueConfigs(ctx context.Context, now time.Time) ([]*service.IntelligenceTestConfig, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT `+intelligenceTestConfigColumns+` FROM intelligence_test_questions q
LEFT JOIN api_keys k ON k.id = q.api_key_id
LEFT JOIN groups g ON g.id = k.group_id
WHERE q.enabled = TRUE AND (q.next_run_at IS NULL OR q.next_run_at <= $1)
ORDER BY q.next_run_at NULLS FIRST, q.id`, now)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]*service.IntelligenceTestConfig, 0)
	for rows.Next() {
		item, err := scanIntelligenceTestConfig(rows)
		if err != nil { return nil, err }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *intelligenceTestRepository) UpdateRun(ctx context.Context, id int64, lastRun, nextRun time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE intelligence_test_questions SET last_run_at=$2, next_run_at=$3, updated_at=NOW() WHERE id=$1`, id, lastRun, nextRun)
	return err
}

func (r *intelligenceTestRepository) ListAPIKeyOptions(ctx context.Context) ([]*service.IntelligenceTestAPIKeyOption, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT k.id, COALESCE(NULLIF(k.name, ''), 'API Key #' || k.id::text), LEFT(k.key, 8), k.status, k.group_id, COALESCE(g.name, '')
FROM api_keys k LEFT JOIN groups g ON g.id = k.group_id
WHERE k.deleted_at IS NULL AND k.status = 'active' AND k.group_id IS NOT NULL
ORDER BY g.name, k.name, k.id`)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]*service.IntelligenceTestAPIKeyOption, 0)
	for rows.Next() {
		item := &service.IntelligenceTestAPIKeyOption{}
		var groupID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.Name, &item.KeyPrefix, &item.Status, &groupID, &item.GroupName); err != nil { return nil, err }
		if groupID.Valid { item.GroupID = &groupID.Int64 }
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *intelligenceTestRepository) InsertResult(ctx context.Context, item *service.IntelligenceTestResult) error {
	row := r.db.QueryRowContext(ctx, `INSERT INTO intelligence_test_results
(question_id, api_key_id, group_id, group_name, model_id, status, check_ok, svg_content, response_text, error_message, latency_ms, started_at, finished_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13) RETURNING id, created_at`, item.ConfigID,
		intelligenceTestNullInt64(item.APIKeyID), item.GroupID, item.GroupName, item.ModelID, item.Status, item.Check, item.SVGContent,
		item.ResponseText, item.ErrorMessage, item.LatencyMs, item.StartedAt, item.FinishedAt)
	return row.Scan(&item.ID, &item.CreatedAt)
}

func (r *intelligenceTestRepository) PruneResults(ctx context.Context, configID int64, keepCount int) error {
	if keepCount <= 0 {
		keepCount = 20
	}
	_, err := r.db.ExecContext(ctx, `DELETE FROM intelligence_test_results
WHERE question_id = $1
  AND id NOT IN (
    SELECT id FROM intelligence_test_results
    WHERE question_id = $1
    ORDER BY created_at DESC, id DESC
    LIMIT $2
  )`, configID, keepCount)
	return err
}

func (r *intelligenceTestRepository) ListResults(ctx context.Context, configID int64, limit int) ([]*service.IntelligenceTestResult, error) {
	if limit <= 0 { limit = 20 }
	if limit > 100 { limit = 100 }
	rows, err := r.db.QueryContext(ctx, `SELECT id, question_id, api_key_id, group_id, group_name, model_id, status, check_ok, svg_content, response_text, error_message, latency_ms, started_at, finished_at, created_at
FROM intelligence_test_results WHERE question_id=$1 ORDER BY created_at DESC LIMIT $2`, configID, limit)
	if err != nil { return nil, err }
	defer rows.Close()
	items := make([]*service.IntelligenceTestResult, 0)
	for rows.Next() {
		item := &service.IntelligenceTestResult{}
		var apiKeyID, groupID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ConfigID, &apiKeyID, &groupID, &item.GroupName, &item.ModelID, &item.Status,
			&item.Check, &item.SVGContent, &item.ResponseText, &item.ErrorMessage, &item.LatencyMs, &item.StartedAt, &item.FinishedAt, &item.CreatedAt); err != nil { return nil, err }
		if apiKeyID.Valid { item.APIKeyID = apiKeyID.Int64 }
		if groupID.Valid { item.GroupID = &groupID.Int64 }
		items = append(items, item)
	}
	return items, rows.Err()
}

func intelligenceTestNullInt64(value int64) any {
	if value == 0 { return nil }
	return value
}
