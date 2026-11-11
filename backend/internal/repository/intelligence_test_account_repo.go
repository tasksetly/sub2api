package repository

import (
	"context"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (r *intelligenceTestRepository) UpdateAccountResult(ctx context.Context, item *service.IntelligenceTestResult) error {
	_, err := r.db.ExecContext(ctx, `UPDATE intelligence_test_results
SET status=$2, check_ok=$3, svg_content=$4, error_message=$5, latency_ms=$6, finished_at=$7
WHERE id=$1 AND question_id IS NULL`, item.ID, item.Status, item.Check, item.SVGContent, item.ErrorMessage, item.LatencyMs, item.FinishedAt)
	return err
}

// Only call while holding the account's advisory lock: no worker can still own these rows.
func (r *intelligenceTestRepository) FailInterruptedAccountResults(ctx context.Context, accountID int64) error {
	_, err := r.db.ExecContext(ctx, `UPDATE intelligence_test_results
SET status='failed', error_message='benchmark interrupted', finished_at=NOW()
WHERE account_id=$1 AND question_id IS NULL AND status='running'`, accountID)
	return err
}

func (r *intelligenceTestRepository) ListAccountResults(ctx context.Context, accountID, beforeID int64, limit int) ([]*service.IntelligenceTestResult, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.QueryContext(ctx, `SELECT id, account_id, account_name, prompt_text, model_id, model_name,
reasoning_effort, status, check_ok, svg_content, error_message, latency_ms, started_at, finished_at, created_at
FROM intelligence_test_results WHERE account_id=$1 AND question_id IS NULL AND ($2::bigint=0 OR id<$2)
ORDER BY id DESC LIMIT $3`, accountID, beforeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]*service.IntelligenceTestResult, 0)
	for rows.Next() {
		item := &service.IntelligenceTestResult{AnswerMode: "svg"}
		if err := rows.Scan(&item.ID, &item.AccountID, &item.AccountName, &item.PromptText, &item.ModelID, &item.ModelName,
			&item.ReasoningEffort, &item.Status, &item.Check, &item.SVGContent, &item.ErrorMessage, &item.LatencyMs,
			&item.StartedAt, &item.FinishedAt, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
