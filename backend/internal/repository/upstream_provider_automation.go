package repository

import (
	"context"
	"fmt"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// Credentials and automation settings must commit together.
func (r *upstreamProviderRepository) withAutomationTransaction(ctx context.Context, fn func(*upstreamProviderRepository) error) error {
	if tx := dbent.TxFromContext(ctx); tx != nil {
		return fn(&upstreamProviderRepository{client: tx.Client(), sql: tx.Client(), encryptor: r.encryptor})
	}
	tx, err := r.client.Tx(ctx)
	if err == dbent.ErrTxStarted {
		return fn(&upstreamProviderRepository{client: r.client, sql: r.client, encryptor: r.encryptor})
	}
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := fn(&upstreamProviderRepository{client: tx.Client(), sql: tx.Client(), encryptor: r.encryptor}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *upstreamProviderRepository) loadAutomation(ctx context.Context, providers []*service.UpstreamProvider) error {
	if len(providers) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(providers))
	index := make(map[int64]*service.UpstreamProvider, len(providers))
	for _, p := range providers {
		p.Automation = service.DefaultUpstreamProviderAutomation()
		ids = append(ids, p.ID)
		index[p.ID] = p
	}
	rows, err := r.sql.QueryContext(ctx, `SELECT upstream_provider_id, sync_cron, next_sync_at,
        balance_alert_enabled, balance_alert_threshold, balance_alert_email,
        balance_alert_sent_at, balance_alert_last_error
        FROM upstream_provider_automation WHERE upstream_provider_id = ANY($1)`, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("load upstream automation: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id int64
		var a service.UpstreamProviderAutomation
		if err := rows.Scan(&id, &a.SyncCron, &a.NextSyncAt, &a.BalanceAlertEnabled,
			&a.BalanceAlertThreshold, &a.BalanceAlertEmail, &a.BalanceAlertSentAt, &a.BalanceAlertLastError); err != nil {
			return err
		}
		index[id].Automation = a
	}
	return rows.Err()
}

func (r *upstreamProviderRepository) saveAutomation(ctx context.Context, provider *service.UpstreamProvider, resetAlert bool) error {
	a := provider.Automation
	if a.SyncCron == "" {
		a.SyncCron = service.DefaultUpstreamProviderAutomation().SyncCron
	}
	const changed = `($7 OR (upstream_provider_automation.balance_alert_enabled,
        upstream_provider_automation.balance_alert_threshold, upstream_provider_automation.balance_alert_email)
        IS DISTINCT FROM (EXCLUDED.balance_alert_enabled, EXCLUDED.balance_alert_threshold, EXCLUDED.balance_alert_email))`
	_, err := r.sql.ExecContext(ctx, `INSERT INTO upstream_provider_automation
        (upstream_provider_id, sync_cron, next_sync_at, balance_alert_enabled, balance_alert_threshold, balance_alert_email)
        VALUES ($1, $2, COALESCE($3, NOW()), $4, $5, $6)
        ON CONFLICT (upstream_provider_id) DO UPDATE SET
        sync_cron = EXCLUDED.sync_cron,
        next_sync_at = CASE WHEN upstream_provider_automation.sync_cron <> EXCLUDED.sync_cron
            THEN EXCLUDED.next_sync_at ELSE upstream_provider_automation.next_sync_at END,
        balance_alert_enabled = EXCLUDED.balance_alert_enabled,
        balance_alert_threshold = EXCLUDED.balance_alert_threshold,
        balance_alert_email = EXCLUDED.balance_alert_email,
        balance_checked_at = CASE WHEN `+changed+` THEN NOW() ELSE upstream_provider_automation.balance_checked_at END,
        balance_alert_sent_at = CASE WHEN `+changed+` THEN NULL ELSE upstream_provider_automation.balance_alert_sent_at END,
        balance_alert_claimed_at = CASE WHEN `+changed+` THEN NULL ELSE upstream_provider_automation.balance_alert_claimed_at END,
        balance_alert_last_error = CASE WHEN `+changed+` THEN '' ELSE upstream_provider_automation.balance_alert_last_error END`,
		provider.ID, a.SyncCron, a.NextSyncAt, a.BalanceAlertEnabled, a.BalanceAlertThreshold, a.BalanceAlertEmail, resetAlert)
	if err != nil {
		return fmt.Errorf("save upstream automation: %w", err)
	}
	return r.loadAutomation(ctx, []*service.UpstreamProvider{provider})
}

func (r *upstreamProviderRepository) ClaimScheduledSync(ctx context.Context, id int64, cron string, now, next time.Time) (bool, error) {
	result, err := r.sql.ExecContext(ctx, `UPDATE upstream_provider_automation a SET next_sync_at = $4
        FROM upstream_providers p WHERE a.upstream_provider_id = $1 AND p.id = a.upstream_provider_id
        AND p.deleted_at IS NULL AND p.status = 'active' AND p.sync_enabled
        AND a.sync_cron = $2 AND a.next_sync_at <= $3`, id, cron, now, next)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *upstreamProviderRepository) PrepareBalanceAlert(ctx context.Context, provider *service.UpstreamProvider, snapshot service.UpstreamSyncSnapshot, claimedAt time.Time) (bool, error) {
	if snapshot.Balance == nil {
		return false, nil
	}
	a := provider.Automation
	// Older concurrent syncs cannot clear a newer notification or re-arm an old balance.
	_, err := r.sql.ExecContext(ctx, `UPDATE upstream_provider_automation a SET
        balance_checked_at = $2,
        balance_alert_sent_at = CASE WHEN $3 >= a.balance_alert_threshold THEN NULL ELSE a.balance_alert_sent_at END,
        balance_alert_claimed_at = CASE WHEN $3 >= a.balance_alert_threshold THEN NULL ELSE a.balance_alert_claimed_at END,
        balance_alert_last_error = CASE WHEN $3 >= a.balance_alert_threshold THEN '' ELSE a.balance_alert_last_error END
        FROM upstream_providers p WHERE a.upstream_provider_id = $1 AND p.id = a.upstream_provider_id
        AND p.deleted_at IS NULL AND p.status = 'active' AND a.balance_alert_enabled
        AND p.base_url = $6 AND p.provider_type = $7 AND p.username = $8
        AND a.balance_alert_threshold = $4 AND a.balance_alert_email = $5
        AND (a.balance_checked_at IS NULL OR a.balance_checked_at <= $2)`,
		provider.ID, snapshot.SyncedAt, *snapshot.Balance, a.BalanceAlertThreshold, a.BalanceAlertEmail,
		provider.BaseURL, providerTypeOrDefault(provider.ProviderType), provider.Username)
	if err != nil {
		return false, err
	}
	// shortcut: SMTP and the database are not atomic; a crash after delivery may retry after this 10-minute lease. Use an idempotent delivery provider if strict exactly-once delivery becomes necessary.
	result, err := r.sql.ExecContext(ctx, `UPDATE upstream_provider_automation a SET balance_alert_claimed_at = $6
        FROM upstream_providers p WHERE a.upstream_provider_id = $1 AND p.id = a.upstream_provider_id
        AND p.deleted_at IS NULL AND p.status = 'active' AND a.balance_alert_enabled
        AND p.base_url = $7 AND p.provider_type = $8 AND p.username = $9
        AND a.balance_checked_at = $2 AND $3 < a.balance_alert_threshold
        AND a.balance_alert_threshold = $4 AND a.balance_alert_email = $5
        AND a.balance_alert_sent_at IS NULL
        AND (a.balance_alert_claimed_at IS NULL OR a.balance_alert_claimed_at <= $6::timestamptz - INTERVAL '10 minutes')`,
		provider.ID, snapshot.SyncedAt, *snapshot.Balance, a.BalanceAlertThreshold, a.BalanceAlertEmail, claimedAt,
		provider.BaseURL, providerTypeOrDefault(provider.ProviderType), provider.Username)
	if err != nil {
		return false, err
	}
	count, err := result.RowsAffected()
	return count == 1, err
}

func (r *upstreamProviderRepository) FinishBalanceAlert(ctx context.Context, id int64, claimedAt time.Time, lastError string) error {
	_, err := r.sql.ExecContext(ctx, `UPDATE upstream_provider_automation SET
        balance_alert_sent_at = CASE WHEN $3 = '' THEN NOW() ELSE balance_alert_sent_at END,
        balance_alert_claimed_at = NULL, balance_alert_last_error = $3
        WHERE upstream_provider_id = $1 AND balance_alert_claimed_at = $2`, id, claimedAt, lastError)
	return err
}
