//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func automationTestProvider(t *testing.T) (*upstreamProviderRepository, *service.UpstreamProvider) {
	t.Helper()
	ctx := context.Background()
	tx := testEntTx(t)
	entity, err := tx.Client().UpstreamProvider.Create().
		SetName("automation-test").SetBaseURL("https://example.com").
		SetUsername("admin@example.com").SetPasswordEncrypted("").
		SetStatus(service.StatusActive).SetSyncEnabled(true).Save(ctx)
	require.NoError(t, err)
	repo := &upstreamProviderRepository{client: tx.Client(), sql: tx}
	p := &service.UpstreamProvider{
		ID: entity.ID, BaseURL: entity.BaseURL, ProviderType: entity.ProviderType, Username: entity.Username,
		Automation: service.DefaultUpstreamProviderAutomation(),
	}
	p.Automation.BalanceAlertEnabled = true
	p.Automation.BalanceAlertThreshold = 10
	p.Automation.BalanceAlertEmail = "ops@example.com"
	require.NoError(t, repo.saveAutomation(ctx, p, false))
	return repo, p
}

func TestUpstreamBalanceAlertDedupRecoveryAndRetry(t *testing.T) {
	repo, p := automationTestProvider(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Minute)
	low, healthy := 5.0, 10.0
	snapshot := service.UpstreamSyncSnapshot{Balance: &low, SyncedAt: now}

	claimed, err := repo.PrepareBalanceAlert(ctx, p, snapshot, now)
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, now)
	require.NoError(t, err)
	require.False(t, claimed, "even an identical claim timestamp cannot deliver twice")
	require.NoError(t, repo.FinishBalanceAlert(ctx, p.ID, now, ""))

	snapshot.SyncedAt = now.Add(time.Minute)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.False(t, claimed, "persisted sent state suppresses further low balances")
	// An older healthy snapshot must not reset the low-balance episode.
	stale := service.UpstreamSyncSnapshot{Balance: &healthy, SyncedAt: now}
	_, err = repo.PrepareBalanceAlert(ctx, p, stale, now.Add(2*time.Minute))
	require.NoError(t, err)
	require.NoError(t, repo.loadAutomation(ctx, []*service.UpstreamProvider{p}))
	require.NotNil(t, p.Automation.BalanceAlertSentAt)

	snapshot = service.UpstreamSyncSnapshot{Balance: &healthy, SyncedAt: now.Add(2 * time.Minute)}
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.False(t, claimed)
	snapshot = service.UpstreamSyncSnapshot{Balance: &low, SyncedAt: now.Add(3 * time.Minute)}
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.True(t, claimed, "recovery followed by another drop re-arms the alert")
	require.NoError(t, repo.FinishBalanceAlert(ctx, p.ID, snapshot.SyncedAt, "SMTP unavailable"))
	require.NoError(t, repo.loadAutomation(ctx, []*service.UpstreamProvider{p}))
	require.Nil(t, p.Automation.BalanceAlertSentAt)
	require.Equal(t, "SMTP unavailable", p.Automation.BalanceAlertLastError)
	snapshot.SyncedAt = now.Add(4 * time.Minute)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.True(t, claimed, "failed delivery can be retried")
}

func TestUpstreamAutomationConfigChangesAndScheduleClaims(t *testing.T) {
	repo, p := automationTestProvider(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Minute)
	next := now.Add(30 * time.Minute)
	claimed, err := repo.ClaimScheduledSync(ctx, p.ID, p.Automation.SyncCron, now, next)
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = repo.ClaimScheduledSync(ctx, p.ID, p.Automation.SyncCron, now, next)
	require.NoError(t, err)
	require.False(t, claimed)

	// Saving a stale form must not overwrite a schedule advanced by a worker.
	require.NoError(t, repo.saveAutomation(ctx, p, false))
	require.True(t, p.Automation.NextSyncAt.Equal(next))
	low := 5.0
	snapshot := service.UpstreamSyncSnapshot{Balance: &low, SyncedAt: now}
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, now)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, repo.FinishBalanceAlert(ctx, p.ID, now, ""))
	require.NoError(t, repo.saveAutomation(ctx, p, false))
	require.NotNil(t, p.Automation.BalanceAlertSentAt, "ordinary edit preserves sent state")

	stale := *p
	p.Automation.BalanceAlertEmail = "new@example.com"
	p.Automation.SyncCron = "0 9 * * *"
	next = next.Add(24 * time.Hour)
	p.Automation.NextSyncAt = &next
	require.NoError(t, repo.saveAutomation(ctx, p, false))
	require.Nil(t, p.Automation.BalanceAlertSentAt)
	require.True(t, p.Automation.NextSyncAt.Equal(next))
	claimed, err = repo.ClaimScheduledSync(ctx, p.ID, stale.Automation.SyncCron, next, next.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, claimed, "stale worker cannot claim a changed schedule")
	claimed, err = repo.PrepareBalanceAlert(ctx, &stale, snapshot, now.Add(time.Minute))
	require.NoError(t, err)
	require.False(t, claimed, "stale worker cannot send to a replaced recipient")

	_, err = repo.sql.ExecContext(ctx, `UPDATE upstream_providers SET status = 'inactive' WHERE id = $1`, p.ID)
	require.NoError(t, err)
	snapshot.SyncedAt = now.Add(2 * time.Minute)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.False(t, claimed)
	claimed, err = repo.ClaimScheduledSync(ctx, p.ID, p.Automation.SyncCron, next, next.Add(time.Hour))
	require.NoError(t, err)
	require.False(t, claimed)
}

func TestUpstreamBalanceAlertLeaseExpiryAndStaleCompletion(t *testing.T) {
	repo, p := automationTestProvider(t)
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Microsecond).Add(time.Minute)
	low := 5.0
	snapshot := service.UpstreamSyncSnapshot{Balance: &low, SyncedAt: now}
	claimed, err := repo.PrepareBalanceAlert(ctx, p, snapshot, now)
	require.NoError(t, err)
	require.True(t, claimed)

	// Recover work abandoned by a stopped process after the lease expires.
	snapshot.SyncedAt = now.Add(11 * time.Minute)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, repo.FinishBalanceAlert(ctx, p.ID, now, "old error"))
	require.NoError(t, repo.loadAutomation(ctx, []*service.UpstreamProvider{p}))
	require.Empty(t, p.Automation.BalanceAlertLastError)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.False(t, claimed, "stale completion must not release another worker's claim")
	require.NoError(t, repo.FinishBalanceAlert(ctx, p.ID, snapshot.SyncedAt, ""))

	p.Automation.BalanceAlertEnabled = false
	require.NoError(t, repo.saveAutomation(ctx, p, false))
	snapshot.SyncedAt = now.Add(12 * time.Minute)
	claimed, err = repo.PrepareBalanceAlert(ctx, p, snapshot, snapshot.SyncedAt)
	require.NoError(t, err)
	require.False(t, claimed, "disabled alerts must not claim or send")
}
