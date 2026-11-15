//go:build integration

package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestRechargeWebhookTriggerAndRollback(t *testing.T) {
	ctx := context.Background()
	user := mustCreateUser(t, testEntClient(t), &service.User{})
	t.Cleanup(func() { _, _ = integrationDB.Exec(`DELETE FROM users WHERE id = $1`, user.ID) })
	const enabledConfig = `{"enabled":true,"version":"v1","destination":"encrypted"}`
	for _, tc := range []struct {
		name       string
		config     string
		orderType  string
		status     string
		completed  bool
		currency   string
		wantEvents int
	}{
		{"balance", enabledConfig, "balance", "COMPLETED", true, " usd ", 1},
		{"currency fallback", enabledConfig, "balance", "COMPLETED", true, "invalid", 1},
		{"subscription", enabledConfig, "subscription", "COMPLETED", true, "CNY", 0},
		{"not completed", enabledConfig, "balance", "PAID", false, "CNY", 0},
		{"missing completion time", enabledConfig, "balance", "COMPLETED", false, "CNY", 0},
		{"disabled", `{"enabled":false,"version":"v1","destination":"encrypted"}`, "balance", "COMPLETED", true, "CNY", 0},
		{"bad config", `broken-json`, "balance", "COMPLETED", true, "CNY", 0},
		{"missing destination", `{"enabled":true,"version":"v1"}`, "balance", "COMPLETED", true, "CNY", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx := testTx(t)
			_, err := tx.ExecContext(ctx, `INSERT INTO settings (key,value) VALUES ($1,$2)
				ON CONFLICT (key) DO UPDATE SET value=EXCLUDED.value`, service.SettingKeyNotificationWebhook, tc.config)
			require.NoError(t, err)
			snapshot, err := json.Marshal(map[string]string{"currency": tc.currency})
			require.NoError(t, err)
			var orderID int64
			require.NoError(t, tx.QueryRowContext(ctx, `
				INSERT INTO payment_orders (user_id,user_email,user_name,amount,bonus_amount,pay_amount,
					order_type,status,expires_at,paid_at,provider_snapshot)
				VALUES ($1,'buyer@example.com','Buyer',12,2,8,$2,'RECHARGING',NOW(),NOW(),$3::jsonb)
				RETURNING id`, user.ID, tc.orderType, string(snapshot)).Scan(&orderID))
			_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status=$2,
				completed_at=CASE WHEN $3 THEN NOW() ELSE NULL END WHERE id=$1`, orderID, tc.status, tc.completed)
			require.NoError(t, err, "invalid optional webhook config must not block an order")
			var count int
			require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_webhook_deliveries WHERE order_id=$1`, orderID).Scan(&count))
			require.Equal(t, tc.wantEvents, count)
			if tc.wantEvents == 1 {
				var payload struct {
					ID   string `json:"id"`
					Data struct {
						Credited float64 `json:"credited_amount"`
						Bonus    float64 `json:"bonus_amount"`
						Paid     float64 `json:"pay_amount"`
						Currency string  `json:"pay_currency"`
						Balance  string  `json:"balance_currency"`
					} `json:"data"`
				}
				var raw []byte
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT payload FROM notification_webhook_deliveries WHERE order_id=$1`, orderID).Scan(&raw))
				require.NoError(t, json.Unmarshal(raw, &payload))
				require.Equal(t, fmt.Sprintf("payment.recharge.completed:%d", orderID), payload.ID)
				require.Equal(t, 12.0, payload.Data.Credited)
				require.Equal(t, 2.0, payload.Data.Bonus)
				require.Equal(t, 8.0, payload.Data.Paid)
				require.Equal(t, "USD", payload.Data.Balance)
				if tc.currency == " usd " {
					require.Equal(t, "USD", payload.Data.Currency)
				} else {
					require.Equal(t, "CNY", payload.Data.Currency)
				}
				_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='COMPLETED' WHERE id=$1`, orderID)
				require.NoError(t, err)
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_webhook_deliveries WHERE order_id=$1`, orderID).Scan(&count))
				require.Equal(t, 1, count, "duplicate callback must not create another event")
				// Even after history retention removes the row, a refund reversal
				// must not recreate the original recharge event.
				_, err = tx.ExecContext(ctx, `DELETE FROM notification_webhook_deliveries WHERE order_id=$1`, orderID)
				require.NoError(t, err)
				for _, status := range []string{"REFUNDING", "COMPLETED"} {
					_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status=$2 WHERE id=$1`, orderID, status)
					require.NoError(t, err)
				}
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_webhook_deliveries WHERE order_id=$1`, orderID).Scan(&count))
				require.Zero(t, count)
				// Restore the pending state to exercise transaction rollback with
				// an actual queued event present.
				_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='RECHARGING',completed_at=NULL WHERE id=$1`, orderID)
				require.NoError(t, err)
				_, err = tx.ExecContext(ctx, `UPDATE payment_orders SET status='COMPLETED',completed_at=NOW() WHERE id=$1`, orderID)
				require.NoError(t, err)
				require.NoError(t, tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_webhook_deliveries WHERE order_id=$1`, orderID).Scan(&count))
				require.Equal(t, 1, count)
			}
			require.NoError(t, tx.Rollback())
			require.NoError(t, integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM notification_webhook_deliveries WHERE order_id=$1`, orderID).Scan(&count))
			require.Zero(t, count, "the event must roll back with order completion")
		})
	}
}

func TestWebhookConfigCompareAndSwap(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	repo := &notificationWebhookRepository{db: tx}
	_, err := tx.ExecContext(ctx, `DELETE FROM settings WHERE key=$1`, service.SettingKeyNotificationWebhook)
	require.NoError(t, err)
	saved, err := repo.SaveConfig(ctx, "deleted-version", `{"version":"v1"}`)
	require.NoError(t, err)
	require.False(t, saved, "a stale form must not recreate removed settings")
	saved, err = repo.SaveConfig(ctx, "", `{"version":"v1"}`)
	require.NoError(t, err)
	require.True(t, saved)
	saved, err = repo.SaveConfig(ctx, "", `{"version":"stale"}`)
	require.NoError(t, err)
	require.False(t, saved, "two initial saves cannot overwrite each other")
	saved, err = repo.SaveConfig(ctx, "v1", `{"version":"v2"}`)
	require.NoError(t, err)
	require.True(t, saved)
	saved, err = repo.SaveConfig(ctx, "v1", `{"version":"stale"}`)
	require.NoError(t, err)
	require.False(t, saved)
	var value string
	require.NoError(t, tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, service.SettingKeyNotificationWebhook).Scan(&value))
	require.JSONEq(t, `{"version":"v2"}`, value)
}

func TestWebhookDeliveryLeaseRecoveryAndHistory(t *testing.T) {
	ctx := context.Background()
	tx := testTx(t)
	repo := &notificationWebhookRepository{db: tx}
	// Transactional isolation prevents other tests' history from being claimed.
	_, err := tx.ExecContext(ctx, `DELETE FROM notification_webhook_deliveries`)
	require.NoError(t, err)
	_, err = tx.ExecContext(ctx, `INSERT INTO notification_webhook_deliveries
		(event_id,event_type,order_id,config_version,payload) VALUES
		('lease-event','payment.recharge.completed',1,'v1','{"id":"lease-event"}')`)
	require.NoError(t, err)
	first, err := repo.Claim(ctx, "worker-a")
	require.NoError(t, err)
	require.NotNil(t, first)
	require.Equal(t, 1, first.Attempts)
	second, err := repo.Claim(ctx, "worker-b")
	require.NoError(t, err)
	require.Nil(t, second, "an active lease cannot be claimed twice")
	require.Error(t, repo.Finish(ctx, first.ID, "worker-b", "delivered", 200, "", time.Now()))
	_, err = tx.ExecContext(ctx, `UPDATE notification_webhook_deliveries SET claimed_at=NOW()-INTERVAL '61 seconds' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	second, err = repo.Claim(ctx, "worker-b")
	require.NoError(t, err)
	require.NotNil(t, second)
	require.Equal(t, first.EventID, second.EventID)
	require.Equal(t, first.Payload, second.Payload)
	require.Equal(t, 2, second.Attempts)
	require.Error(t, repo.Finish(ctx, first.ID, "worker-a", "delivered", 200, "", time.Now()))
	require.NoError(t, repo.Finish(ctx, second.ID, "worker-b", "pending", 503, "receiver returned HTTP 503", time.Now().Add(time.Hour)))
	notReady, err := repo.Claim(ctx, "worker-c")
	require.NoError(t, err)
	require.Nil(t, notReady)
	_, err = tx.ExecContext(ctx, `UPDATE notification_webhook_deliveries
		SET status='delivering',attempts=5,claimed_by='crashed',claimed_at=NOW()-INTERVAL '61 seconds' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	exhausted, err := repo.Claim(ctx, "worker-c")
	require.NoError(t, err)
	require.Nil(t, exhausted)
	history, err := repo.ListRecent(ctx)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, "failed", history[0].Status)
	require.NotNil(t, history[0].FinishedAt)
	_, err = tx.ExecContext(ctx, `UPDATE notification_webhook_deliveries SET finished_at=NOW()-INTERVAL '31 days' WHERE id=$1`, first.ID)
	require.NoError(t, err)
	require.NoError(t, repo.Cleanup(ctx))
	history, err = repo.ListRecent(ctx)
	require.NoError(t, err)
	require.Empty(t, history)
}
