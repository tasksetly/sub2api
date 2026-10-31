package service

import (
	"context"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

type intelligenceTestLockRepo struct {
	IntelligenceTestRepository
	config         *IntelligenceTestConfig
	results        []*IntelligenceTestResult
	getConfigCalls int
}

func (r *intelligenceTestLockRepo) ListDueConfigs(context.Context, time.Time) ([]*IntelligenceTestConfig, error) {
	// A scheduler may already have read this ID before a peer completes it or
	// an administrator disables it.
	return []*IntelligenceTestConfig{{ID: r.config.ID}}, nil
}

func (r *intelligenceTestLockRepo) GetConfig(context.Context, int64) (*IntelligenceTestConfig, error) {
	r.getConfigCalls++
	return r.config, nil
}

func (r *intelligenceTestLockRepo) ListResults(context.Context, int64, int) ([]*IntelligenceTestResult, error) {
	return r.results, nil
}

func intelligenceTestLockID(id int64) int64 {
	return hashAdvisoryLockID("intelligence_test:config:" + strconv.FormatInt(id, 10))
}

func TestIntelligenceTestScheduledRunSkipsLockHeldByPeer(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	repo := &intelligenceTestLockRepo{config: &IntelligenceTestConfig{ID: 7}}
	svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, db)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
		WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(false))

	svc.runDue()

	require.Zero(t, repo.getConfigCalls, "a peer owns this run, so no inference may start")
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIntelligenceTestScheduledRunRechecksConfigAfterLock(t *testing.T) {
	nextRun := time.Now().Add(time.Hour)
	for name, config := range map[string]*IntelligenceTestConfig{
		"completed by peer":   {ID: 7, Enabled: true, NextRunAt: &nextRun},
		"disabled after scan": {ID: 7, Enabled: false},
	} {
		t.Run(name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			repo := &intelligenceTestLockRepo{config: config}
			svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, db)
			mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
				WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(true))
			mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
				WillReturnResult(sqlmock.NewResult(0, 1))

			svc.runDue()

			require.Equal(t, 1, repo.getConfigCalls)
			require.Empty(t, svc.runs, "stale schedules must not start a new run")
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestIntelligenceTestManualRunUsesSameLockAsSchedule(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := NewIntelligenceTestService(nil, nil, nil, nil, nil, db)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
		WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(false))

	_, err = svc.RunNow(context.Background(), 7)

	require.ErrorIs(t, err, ErrIntelligenceTestAlreadyRunning)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIntelligenceTestLockFailureDoesNotRunLocally(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	svc := NewIntelligenceTestService(nil, nil, nil, nil, nil, db)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
		WillReturnError(errors.New("database unavailable"))

	_, err = svc.QueueRun(context.Background(), 7)

	require.ErrorContains(t, err, "database unavailable")
	require.Empty(t, svc.runs)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIntelligenceTestRunStatusOnAnotherInstance(t *testing.T) {
	for _, running := range []bool{true, false} {
		t.Run(strconv.FormatBool(running), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			finished := time.Now()
			repo := &intelligenceTestLockRepo{results: []*IntelligenceTestResult{{ID: 42, Status: "validated", FinishedAt: finished}}}
			svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, db)
			mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
				WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(!running))
			if !running {
				mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(intelligenceTestLockID(7)).
					WillReturnResult(sqlmock.NewResult(0, 1))
			}

			state, err := svc.RunStatus(context.Background(), 7)

			require.NoError(t, err)
			require.Equal(t, running, state.Running)
			if running {
				require.Equal(t, "running", state.Status)
			} else {
				require.Equal(t, "validated", state.Status)
				require.EqualValues(t, 42, state.ResultID)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
