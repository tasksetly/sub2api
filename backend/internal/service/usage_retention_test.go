package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/stretchr/testify/require"
)

func TestUsageRetentionPolicy(t *testing.T) {
	for _, tt := range []struct {
		name, policy, legacy string
		enabled              bool
		days                 int
		invalid              bool
	}{
		{name: "deployment default", enabled: true, days: 90},
		{name: "legacy forever", legacy: `{"request_retention_days":0}`, days: 90},
		{name: "legacy window", legacy: `{"request_retention_days":30}`, enabled: true, days: 30},
		{name: "explicit off overrides legacy", policy: `{"enabled":false,"retention_days":90}`, legacy: `{"request_retention_days":7}`, days: 90},
		{name: "explicit window overrides legacy", policy: `{"enabled":true,"retention_days":14}`, legacy: `{"request_retention_days":90}`, enabled: true, days: 14},
		{name: "zero cannot mean delete everything", policy: `{"enabled":true,"retention_days":0}`, invalid: true},
		{name: "negative", policy: `{"enabled":true,"retention_days":-1}`, invalid: true},
		{name: "too large", policy: `{"enabled":true,"retention_days":3651}`, invalid: true},
		{name: "broken policy never falls back", policy: `{broken`, legacy: `{"request_retention_days":1}`, invalid: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			repo := newRuntimeSettingRepoStub()
			if tt.policy != "" {
				repo.values[SettingKeyUsageRetention] = tt.policy
			}
			if tt.legacy != "" {
				repo.values[SettingKeyOpsRuntimeLogConfig] = tt.legacy
			}
			cfg := config.DashboardAggregationConfig{Enabled: true, Retention: config.DashboardAggregationRetentionConfig{UsageLogsDays: 90}}
			policy, err := readUsageRetention(context.Background(), repo, cfg)
			if tt.invalid {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, UsageRetentionSettings{Enabled: tt.enabled, RetentionDays: tt.days}, policy)
		})
	}
}

func TestUsageRetentionReadFailureNeverDeletes(t *testing.T) {
	settings := newRuntimeSettingRepoStub()
	settings.getValueFn = func(string) (string, error) { return "", errors.New("unavailable") }
	repo := &dashboardAggregationRepoTestStub{}
	svc := &DashboardAggregationService{repo: repo, settingRepo: settings}
	svc.maybeCleanupRetention(context.Background(), time.Now().UTC())
	require.Zero(t, repo.cleanupUsageCalls)
	require.Zero(t, repo.cleanupDedupCalls)
}

func TestUsageRetentionLockContentionSkipsDeletion(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs(hashAdvisoryLockID(usageRetentionLockKey)).
		WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	repo := &dashboardAggregationRepoTestStub{}
	svc := &DashboardAggregationService{repo: repo, db: db}
	svc.maybeCleanupRetention(context.Background(), time.Now().UTC())
	require.Zero(t, repo.cleanupUsageCalls)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestManualCleanupSharesRetentionLock(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectQuery("SELECT pg_try_advisory_lock").WithArgs(hashAdvisoryLockID(usageRetentionLockKey)).
		WillReturnRows(sqlmock.NewRows([]string{"locked"}).AddRow(false))
	repo := &cleanupRepoStub{claimQueue: []*UsageCleanupTask{{ID: 1}}}
	svc := NewUsageCleanupService(repo, nil, nil, nil)
	svc.db = db
	svc.runOnce()
	require.Len(t, repo.claimQueue, 1, "must not claim a task while another process is deleting")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestBackfillCannotMoveRetentionCutoffIntoTheFuture(t *testing.T) {
	repo := &dashboardAggregationRepoTestStub{}
	svc := NewDashboardAggregationService(repo, nil, &config.Config{DashboardAgg: config.DashboardAggregationConfig{
		Enabled:   true,
		Retention: config.DashboardAggregationRetentionConfig{UsageLogsDays: 90, HourlyDays: 180, DailyDays: 730},
	}})
	now := time.Now().UTC()
	future := now.Add(7 * 24 * time.Hour)
	require.NoError(t, svc.backfillRange(context.Background(), future, future.Add(time.Hour)))
	require.WithinDuration(t, now.Add(-90*24*time.Hour), repo.cleanupUsageCutoff, time.Minute)
}
