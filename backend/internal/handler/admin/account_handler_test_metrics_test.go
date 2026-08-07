package admin

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestPersistSuccessfulAccountTestMetrics(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}

	handler.persistSuccessfulAccountTestMetrics(
		context.Background(),
		17,
		"  gpt-5.4  ",
		time.Now().Add(-20*time.Millisecond),
	)

	require.Equal(t, 1, adminSvc.updateAccountExtraCalls)
	require.Equal(t, int64(17), adminSvc.lastUpdateAccountExtraID)
	require.Equal(t, "gpt-5.4", adminSvc.lastUpdateAccountExtra[service.AccountTestModelExtraKey])

	latency, ok := adminSvc.lastUpdateAccountExtra[service.AccountTestLatencyExtraKey].(int64)
	require.True(t, ok)
	require.GreaterOrEqual(t, latency, int64(0))

	completedAt, ok := adminSvc.lastUpdateAccountExtra[service.AccountTestCompletedAtExtraKey].(string)
	require.True(t, ok)
	_, err := time.Parse(time.RFC3339Nano, completedAt)
	require.NoError(t, err)
}

func TestPersistSuccessfulAccountTestMetricsClearsModelWhenUnspecified(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}

	handler.persistSuccessfulAccountTestMetrics(context.Background(), 17, "", time.Now())

	require.Equal(t, 1, adminSvc.updateAccountExtraCalls)
	value, exists := adminSvc.lastUpdateAccountExtra[service.AccountTestModelExtraKey]
	require.True(t, exists)
	require.Nil(t, value)
}

func TestPersistSuccessfulAccountTestMetricsSkipsInvalidAccount(t *testing.T) {
	adminSvc := newStubAdminService()
	handler := &AccountHandler{adminService: adminSvc}

	handler.persistSuccessfulAccountTestMetrics(context.Background(), 0, "gpt-5.4", time.Now())

	require.Zero(t, adminSvc.updateAccountExtraCalls)
}
