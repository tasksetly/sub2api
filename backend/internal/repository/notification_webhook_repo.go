package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type notificationWebhookRepository struct {
	db sqlExecutor
}

func NewNotificationWebhookRepository(db *sql.DB) service.NotificationWebhookRepository {
	return &notificationWebhookRepository{db: db}
}

func (r *notificationWebhookRepository) SaveConfig(ctx context.Context, expectedVersion, value string) (bool, error) {
	var result sql.Result
	var err error
	if expectedVersion == "" {
		result, err = r.db.ExecContext(ctx, `
			INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, NOW())
			ON CONFLICT (key) DO NOTHING
		`, service.SettingKeyNotificationWebhook, value)
	} else {
		// UPDATE cannot recreate settings removed since the form was loaded.
		result, err = r.db.ExecContext(ctx, `
			UPDATE settings SET value = $2, updated_at = NOW()
			WHERE key = $1 AND value::jsonb->>'version' = $3
		`, service.SettingKeyNotificationWebhook, value, expectedVersion)
	}
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *notificationWebhookRepository) Claim(ctx context.Context, workerID string) (*service.NotificationWebhookDelivery, error) {
	// A crashed final attempt must become terminal after its lease expires.
	if _, err := r.db.ExecContext(ctx, `
		UPDATE notification_webhook_deliveries
		SET status = 'failed', finished_at = NOW(), claimed_at = NULL, claimed_by = NULL,
		    last_error = 'delivery lease expired after final attempt'
		WHERE status = 'delivering' AND attempts >= $1
		  AND claimed_at < NOW() - INTERVAL '60 seconds'
	`, service.WebhookMaxAttempts); err != nil {
		return nil, err
	}
	var item service.NotificationWebhookDelivery
	err := scanSingleRow(ctx, r.db, `
		WITH candidate AS (
			SELECT id FROM notification_webhook_deliveries
			WHERE status IN ('pending', 'delivering') AND attempts < $2
			  AND available_at <= NOW()
			  AND (claimed_at IS NULL OR claimed_at < NOW() - INTERVAL '60 seconds')
			ORDER BY available_at, id LIMIT 1 FOR UPDATE SKIP LOCKED
		)
		UPDATE notification_webhook_deliveries AS d
		SET status = 'delivering', attempts = attempts + 1, claimed_at = NOW(), claimed_by = $1
		FROM candidate AS c WHERE d.id = c.id
		RETURNING d.id, d.event_id, d.event_type, d.order_id, d.config_version, d.payload,
		          d.status, d.attempts, d.created_at
	`, []any{workerID, service.WebhookMaxAttempts},
		&item.ID, &item.EventID, &item.EventType, &item.OrderID, &item.ConfigVersion, &item.Payload,
		&item.Status, &item.Attempts, &item.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *notificationWebhookRepository) Finish(ctx context.Context, id int64, workerID, status string, statusCode int, message string, availableAt time.Time) error {
	result, err := r.db.ExecContext(ctx, `
		UPDATE notification_webhook_deliveries
		SET status = $3, last_status_code = $4, last_error = $5, available_at = $6,
		    claimed_at = NULL, claimed_by = NULL,
		    finished_at = CASE WHEN $3 = 'pending' THEN NULL ELSE NOW() END
		WHERE id = $1 AND claimed_by = $2 AND status = 'delivering'
	`, id, workerID, status, statusCode, message, availableAt)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err == nil && count != 1 {
		return errors.New("webhook delivery lease was lost")
	}
	return err
}

func (r *notificationWebhookRepository) ListRecent(ctx context.Context) ([]service.NotificationWebhookDelivery, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, event_id, event_type, order_id, status, attempts, last_status_code,
		       last_error, created_at, finished_at
		FROM notification_webhook_deliveries ORDER BY id DESC LIMIT 20
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]service.NotificationWebhookDelivery, 0, 20)
	for rows.Next() {
		var item service.NotificationWebhookDelivery
		if err := rows.Scan(&item.ID, &item.EventID, &item.EventType, &item.OrderID, &item.Status,
			&item.Attempts, &item.LastStatusCode, &item.LastError, &item.CreatedAt, &item.FinishedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *notificationWebhookRepository) Cleanup(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `
		DELETE FROM notification_webhook_deliveries WHERE id IN (
			SELECT id FROM notification_webhook_deliveries
			WHERE finished_at < NOW() - INTERVAL '30 days'
			ORDER BY finished_at LIMIT 1000 FOR UPDATE SKIP LOCKED
		)
	`)
	return err
}
