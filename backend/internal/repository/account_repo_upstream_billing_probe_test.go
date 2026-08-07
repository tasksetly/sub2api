package repository

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestUpstreamBillingProbeExtraIsSchedulerNeutral(t *testing.T) {
	require.True(t, isSchedulerNeutralExtraKey("upstream_billing_probe"))
	require.True(t, isSchedulerNeutralExtraKey("upstream_billing_probe_enabled"))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		"upstream_billing_probe":         map[string]any{"status": "ok"},
		"upstream_billing_probe_enabled": true,
	}))
}

func TestAccountTestMetricsExtraAreSchedulerNeutral(t *testing.T) {
	require.True(t, isSchedulerNeutralExtraKey(service.AccountTestLatencyExtraKey))
	require.True(t, isSchedulerNeutralExtraKey(service.AccountTestModelExtraKey))
	require.True(t, isSchedulerNeutralExtraKey(service.AccountTestCompletedAtExtraKey))
	require.False(t, isSchedulerNeutralExtraKey("last_test_future_policy"))
	require.False(t, shouldEnqueueSchedulerOutboxForExtraUpdates(map[string]any{
		service.AccountTestLatencyExtraKey:     int64(100),
		service.AccountTestModelExtraKey:       "gpt-5.4",
		service.AccountTestCompletedAtExtraKey: "2026-07-13T00:00:00Z",
	}))
}
