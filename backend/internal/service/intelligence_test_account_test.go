//go:build unit

package service

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type accountIntelligenceRepo struct {
	IntelligenceTestRepository
	inserted      IntelligenceTestResult
	completed     chan IntelligenceTestResult
	interruptedID int64
}

func (r *accountIntelligenceRepo) InsertResult(_ context.Context, item *IntelligenceTestResult) error {
	item.ID = 17
	r.inserted = *item
	return nil
}

func (r *accountIntelligenceRepo) UpdateAccountResult(_ context.Context, item *IntelligenceTestResult) error {
	r.completed <- *item
	return nil
}

func (r *accountIntelligenceRepo) FailInterruptedAccountResults(_ context.Context, id int64) error {
	r.interruptedID = id
	return nil
}

func (r *accountIntelligenceRepo) ListAccountResults(context.Context, int64, int64, int) ([]*IntelligenceTestResult, error) {
	return []*IntelligenceTestResult{}, nil
}

func TestAccountIntelligenceRunPinsAccountAndPersistsSVG(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	account := &Account{
		ID: 285, Name: "Chosen account", Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
		Status: StatusActive, Concurrency: 1,
		Credentials: map[string]any{"api_key": "chosen-key", "base_url": "https://chosen.example"},
		Extra:       map[string]any{openai_compat.ExtraKeyResponsesMode: "force_responses"},
	}
	accountTest, upstream := adaptiveCNAccountTestService(account, newJSONResponse(http.StatusOK,
		"data: {\"type\":\"response.output_text.delta\",\"delta\":\"Explanation\\n<svg><text>A & B</text></svg>\\ntrailing text\"}\n\ndata: {\"type\":\"response.completed\"}\n\n"))
	repo := &accountIntelligenceRepo{completed: make(chan IntelligenceTestResult, 1)}
	svc := NewIntelligenceTestService(repo, nil, accountTest.accountRepo, accountTest, nil, db)
	lockID := hashAdvisoryLockID("intelligence_test:account:285")
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(lockID).
		WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(true))
	mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(lockID).
		WillReturnResult(sqlmock.NewResult(0, 1))

	ctx, cancel := context.WithCancel(context.Background())
	queued, err := svc.QueueAccountRun(ctx, account.ID, AccountIntelligenceTestInput{
		ModelID: "test-model", PromptText: "Draw my custom bird", ReasoningEffort: "high",
	})
	cancel() // Closing the HTTP request must not cancel generation or persistence.
	require.NoError(t, err)
	require.Equal(t, "running", queued.Status)
	require.Zero(t, repo.inserted.ConfigID)
	require.Equal(t, account.ID, *repo.inserted.AccountID)
	require.Equal(t, account.Name, repo.inserted.AccountName)
	require.Equal(t, "Draw my custom bird", repo.inserted.PromptText)
	select {
	case result := <-repo.completed:
		require.Equal(t, queued.ID, result.ID)
		require.Equal(t, "validated", result.Status)
		require.Equal(t, "<svg><text>A & B</text></svg>", result.SVGContent)
		require.Empty(t, result.ErrorMessage)
	case <-time.After(time.Second):
		t.Fatal("background result was not saved")
	}
	require.Len(t, upstream.requests, 1)
	require.Equal(t, "chosen.example", upstream.requests[0].URL.Host)
	require.Equal(t, "Bearer chosen-key", upstream.requests[0].Header.Get("Authorization"))
	require.Equal(t, "test-model", gjson.GetBytes(upstream.bodies[0], "model").String())
	require.Contains(t, string(upstream.bodies[0]), "Draw my custom bird")
	require.Contains(t, string(upstream.bodies[0]), intelligenceTestSVGSystemPrompt)
	require.Equal(t, "high", gjson.GetBytes(upstream.bodies[0], "reasoning.effort").String())
	require.Eventually(t, func() bool { return mock.ExpectationsWereMet() == nil }, time.Second, time.Millisecond)
}

func TestAccountIntelligenceHistoryOnlyRecoversUnownedRuns(t *testing.T) {
	for _, locked := range []bool{false, true} {
		db, mock, err := sqlmock.New()
		require.NoError(t, err)
		repo := &accountIntelligenceRepo{}
		svc := NewIntelligenceTestService(repo, nil, nil, nil, nil, db)
		lockID := hashAdvisoryLockID("intelligence_test:account:7")
		mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(lockID).
			WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(locked))
		if locked {
			mock.ExpectExec(`SELECT pg_advisory_unlock\(\$1\)`).WithArgs(lockID).
				WillReturnResult(sqlmock.NewResult(0, 1))
		}
		_, err = svc.ListAccountResults(context.Background(), 7, 0)
		require.NoError(t, err)
		if locked {
			require.EqualValues(t, 7, repo.interruptedID)
		} else {
			require.Zero(t, repo.interruptedID)
		}
		require.NoError(t, mock.ExpectationsWereMet())
		db.Close()
	}
}

func TestAccountIntelligenceInputValidation(t *testing.T) {
	for _, input := range []AccountIntelligenceTestInput{
		{ModelID: " ", PromptText: "draw"},
		{ModelID: "model", PromptText: " "},
		{ModelID: strings.Repeat("a", 201), PromptText: "draw"},
		{ModelID: "model", PromptText: strings.Repeat("画", 20001)},
		{ModelID: "model", PromptText: "draw", ReasoningEffort: "unknown"},
	} {
		require.Error(t, input.normalize())
	}
	input := AccountIntelligenceTestInput{ModelID: " model ", PromptText: " 画鸟 ", ReasoningEffort: " HIGH "}
	require.NoError(t, input.normalize())
	require.Equal(t, AccountIntelligenceTestInput{ModelID: "model", PromptText: "画鸟", ReasoningEffort: "high"}, input)
}

func TestIntelligenceResultRejectsMissingSVGAndRedactsUpstreamErrors(t *testing.T) {
	for _, tc := range []struct {
		run  *ScheduledTestResult
		err  error
		want string
	}{
		{run: &ScheduledTestResult{Status: "success", ResponseText: "just text"}, want: "svg_missing_or_invalid"},
		{run: &ScheduledTestResult{Status: "failed", ErrorCode: 403, ErrorMessage: "secret key"}, want: "upstream HTTP 403"},
		{err: errors.New("secret key"), want: "upstream test failed"},
	} {
		result := &IntelligenceTestResult{StartedAt: time.Now()}
		applyIntelligenceTestRun(context.Background(), result, tc.run, tc.err)
		require.Equal(t, "failed", result.Status)
		require.False(t, result.Check)
		require.Empty(t, result.SVGContent)
		require.Equal(t, tc.want, result.ErrorMessage)
	}
}

func TestAccountIntelligenceRunRejectsConcurrentRun(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	account := &Account{ID: 285, Platform: PlatformOpenAI, Type: AccountTypeAPIKey}
	accountTest, upstream := adaptiveCNAccountTestService(account)
	repo := &accountIntelligenceRepo{}
	svc := NewIntelligenceTestService(repo, nil, accountTest.accountRepo, accountTest, nil, db)
	mock.ExpectQuery(`SELECT pg_try_advisory_lock\(\$1\)`).WithArgs(hashAdvisoryLockID("intelligence_test:account:285")).
		WillReturnRows(sqlmock.NewRows([]string{"acquired"}).AddRow(false))
	_, err = svc.QueueAccountRun(context.Background(), account.ID, AccountIntelligenceTestInput{ModelID: "model", PromptText: "draw"})
	require.ErrorIs(t, err, ErrIntelligenceTestAlreadyRunning)
	require.Zero(t, repo.inserted.ID)
	require.Empty(t, upstream.requests)
	require.NoError(t, mock.ExpectationsWereMet())
}
