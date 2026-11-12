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

func (r *intelligenceTestLockRepo) UpdateConfig(_ context.Context, config *IntelligenceTestConfig) (*IntelligenceTestConfig, error) {
	r.config = config
	return config, nil
}

func TestIntelligenceTestUpdateReschedulesOnlyWhenNeeded(t *testing.T) {
	for _, mode := range []string{"candy", "svg"} {
		for _, tc := range []struct {
			name        string
			cron        string
			enabled     bool
			missingNext bool
			reschedule  bool
		}{
			{"changed cron", "* * * * *", true, false, true},
			{"re-enabled", "0 * * * *", false, false, true},
			{"missing next run", "0 * * * *", true, true, true},
			{"stale next run", "0 * * * *", true, false, true},
			{"unrelated edit", "0 * * * *", true, false, false},
		} {
			t.Run(mode+"/"+tc.name, func(t *testing.T) {
				oldNext := time.Now().Add(24 * time.Hour)
				if !tc.reschedule {
					oldNext = time.Now().Add(-time.Minute)
				}
				config := &IntelligenceTestConfig{ID: 7, AnswerMode: mode, CronExpression: "0 * * * *", Enabled: tc.enabled, NextRunAt: &oldNext}
				if tc.missingNext {
					config.NextRunAt = nil
				}
				repo := &intelligenceTestLockRepo{config: config}
				svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, nil)
				before := time.Now()
				updated, err := svc.UpdateConfig(context.Background(), 7, IntelligenceTestConfigInput{
					AnswerMode: mode, Name: "edited", ModelID: "test", PromptText: "Draw a bird", APIKeyID: 1, CronExpression: tc.cron,
				})
				require.NoError(t, err)
				require.NotNil(t, updated.NextRunAt)
				if tc.reschedule {
					require.True(t, updated.NextRunAt.After(before))
					require.True(t, updated.NextRunAt.Before(before.Add(2*time.Hour)))
				} else {
					require.Equal(t, oldNext, *updated.NextRunAt, "editing a prompt must not postpone a due job")
				}
			})
		}
	}
}

type intelligenceTestConcurrentRepo struct {
	IntelligenceTestRepository
	entered chan context.Context
	release chan struct{}
}

func (r *intelligenceTestConcurrentRepo) ListDueConfigs(context.Context, time.Time) ([]*IntelligenceTestConfig, error) {
	return []*IntelligenceTestConfig{{ID: 7}, {ID: 8}}, nil
}

func (r *intelligenceTestConcurrentRepo) GetConfig(ctx context.Context, id int64) (*IntelligenceTestConfig, error) {
	r.entered <- ctx
	<-r.release
	return &IntelligenceTestConfig{ID: id, Enabled: false}, nil
}

func TestIntelligenceTestDueJobsHaveIndependentTimeouts(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.MatchExpectationsInOrder(false)
	for _, id := range []int64{7, 8} {
		mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(intelligenceTestLockID(id)).
			WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(true))
		mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(intelligenceTestLockID(id)).
			WillReturnResult(sqlmock.NewResult(0, 1))
	}
	repo := &intelligenceTestConcurrentRepo{entered: make(chan context.Context, 2), release: make(chan struct{})}
	svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, db)
	done := make(chan struct{})
	go func() { svc.runDue(); close(done) }()
	var contexts []context.Context
	for range 2 {
		select {
		case ctx := <-repo.entered:
			contexts = append(contexts, ctx)
		case <-time.After(time.Second):
			close(repo.release)
			<-done
			t.Fatal("a slow task blocked the next due test")
		}
	}
	close(repo.release)
	<-done
	require.NotSame(t, contexts[0], contexts[1])
	for _, ctx := range contexts {
		deadline, ok := ctx.Deadline()
		require.True(t, ok)
		require.Greater(t, time.Until(deadline), 14*time.Minute)
	}
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestIntelligenceTestBusySchedulerLeavesJobsForNextScan(t *testing.T) {
	repo := &intelligenceTestLockRepo{config: &IntelligenceTestConfig{ID: 7, Enabled: true}}
	svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, nil)
	for range cap(svc.runSlots) {
		svc.runSlots <- struct{}{}
	}
	svc.runDue()
	require.Zero(t, repo.getConfigCalls)
	require.Empty(t, svc.runs, "a full worker pool must leave due jobs available to a later scan")
}
