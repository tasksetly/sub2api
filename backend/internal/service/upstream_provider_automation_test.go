//go:build unit

package service

import (
	"context"
	"errors"
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestUpstreamSyncSchedule(t *testing.T) {
	from := time.Date(2026, 11, 11, 0, 45, 0, 0, time.UTC)
	for _, tc := range []struct {
		expr string
		want time.Time
	}{
		{defaultUpstreamSyncCron, time.Date(2026, 11, 11, 1, 0, 0, 0, time.UTC)},
		{"0 9 * * *", time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)},
		{"CRON_TZ=Asia/Shanghai 0 9 * * *", time.Date(2026, 11, 11, 1, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.expr, func(t *testing.T) {
			next, err := nextUpstreamSync(tc.expr, from.In(time.FixedZone("server", -7*3600)))
			require.NoError(t, err)
			require.True(t, next.Equal(tc.want))
		})
	}
	for _, expr := range []string{"CRON_TZ=UTC", "* * * * * *", "0 25 * * *", "0 0 31 2 *", "@every 1s", "CRON_TZ=invalid * * * * *"} {
		_, err := nextUpstreamSync(expr, from)
		require.Error(t, err, expr)
	}
}

func TestUpstreamAutomationValidationAndOmittedUpdate(t *testing.T) {
	now := time.Now()
	cfg := UpstreamProviderAutomationConfig{
		SyncCron: "CRON_TZ=Asia/Shanghai 0 9 * * *", BalanceAlertEnabled: true,
		BalanceAlertThreshold: 10, BalanceAlertEmail: "  ops@example.com  ",
	}
	p := &UpstreamProvider{Automation: DefaultUpstreamProviderAutomation()}
	require.NoError(t, applyUpstreamAutomation(p, &cfg, now))
	require.Equal(t, "ops@example.com", p.Automation.BalanceAlertEmail)
	require.NotNil(t, p.Automation.NextSyncAt)
	before := p.Automation
	require.NoError(t, applyUpstreamAutomation(p, nil, now.Add(time.Hour)))
	require.Equal(t, before, p.Automation)
	for _, change := range []func(*UpstreamProviderAutomationConfig){
		func(c *UpstreamProviderAutomationConfig) { c.BalanceAlertEmail = "" },
		func(c *UpstreamProviderAutomationConfig) { c.BalanceAlertEmail = "not-an-email" },
		func(c *UpstreamProviderAutomationConfig) {
			c.BalanceAlertEmail = "ops@example.com\r\nBcc: other@example.com"
		},
		func(c *UpstreamProviderAutomationConfig) { c.BalanceAlertThreshold = -1 },
		func(c *UpstreamProviderAutomationConfig) { c.BalanceAlertThreshold = math.NaN() },
		func(c *UpstreamProviderAutomationConfig) { c.BalanceAlertThreshold = math.Inf(1) },
		func(c *UpstreamProviderAutomationConfig) { c.BalanceAlertThreshold = 1e12 },
	} {
		invalid := cfg
		change(&invalid)
		require.Error(t, applyUpstreamAutomation(p, &invalid, now))
		require.Equal(t, before, p.Automation)
	}
}

type upstreamAlertRepoStub struct {
	UpstreamProviderRepository
	claim     bool
	prepared  int
	finished  int
	lastError string
}

func (r *upstreamAlertRepoStub) PrepareBalanceAlert(context.Context, *UpstreamProvider, UpstreamSyncSnapshot, time.Time) (bool, error) {
	r.prepared++
	return r.claim, nil
}

func (r *upstreamAlertRepoStub) FinishBalanceAlert(_ context.Context, _ int64, _ time.Time, lastError string) error {
	r.finished++
	r.lastError = lastError
	return nil
}

type upstreamAlertMailStub struct {
	calls int
	to    string
	body  string
	err   error
}

func (m *upstreamAlertMailStub) SendEmail(_ context.Context, to, _ string, body string) error {
	m.calls++
	m.to, m.body = to, body
	return m.err
}

func TestUpstreamBalanceNotificationRequiresFreshKnownBalanceAndClaim(t *testing.T) {
	repo := &upstreamAlertRepoStub{}
	mailer := &upstreamAlertMailStub{}
	svc := &UpstreamProviderService{repo: repo, emailService: mailer}
	p := &UpstreamProvider{Name: "<script>provider</script>", BaseURL: "https://example.com"}
	p.Automation.BalanceAlertEnabled = true
	p.Automation.BalanceAlertEmail = "ops@example.com"
	p.Automation.BalanceAlertThreshold = 10
	svc.notifyUpstreamBalance(context.Background(), p, UpstreamSyncSnapshot{})
	require.Zero(t, repo.prepared)
	balance := 5.0
	snapshot := UpstreamSyncSnapshot{Balance: &balance, SyncedAt: time.Now()}
	svc.notifyUpstreamBalance(context.Background(), p, snapshot)
	require.Zero(t, mailer.calls)
	repo.claim = true
	svc.notifyUpstreamBalance(context.Background(), p, snapshot)
	require.Equal(t, 1, mailer.calls)
	require.Equal(t, "ops@example.com", mailer.to)
	require.Contains(t, mailer.body, "&lt;script&gt;")
	require.NotContains(t, mailer.body, "<script>")
	require.Equal(t, 1, repo.finished)
	require.Empty(t, repo.lastError)
	mailer.err = errors.New("SMTP unavailable")
	svc.notifyUpstreamBalance(context.Background(), p, snapshot)
	require.Equal(t, "SMTP unavailable", repo.lastError)
}
