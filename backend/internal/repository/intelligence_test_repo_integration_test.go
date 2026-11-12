//go:build integration

package repository

import (
	"context"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestIntelligenceTestDisplayLimitDoesNotDeleteValidHistory(t *testing.T) {
	ctx := context.Background()
	repo := NewIntelligenceTestRepository(integrationDB)
	config, err := repo.CreateConfig(ctx, &service.IntelligenceTestConfig{
		Name: "history-retention", ModelID: "test-model", ModelName: "test-model",
		PromptText: "Generate SVG", CronExpression: "0 * * * *", KeepCount: 10,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.DeleteConfig(ctx, config.ID)) })

	var newestID int64
	for i := 0; i < 30; i++ {
		now := time.Now()
		result := &service.IntelligenceTestResult{
			ConfigID: config.ID, ModelID: "test-model", ModelName: "test-model",
			Status: "validated", Check: true, SVGContent: "<svg/>", StartedAt: now, FinishedAt: now,
		}
		require.NoError(t, repo.InsertResult(ctx, result))
		require.NoError(t, repo.PruneResults(ctx, config.ID))
		newestID = result.ID
	}
	history, err := repo.ListResults(ctx, config.ID, 100)
	require.NoError(t, err)
	require.Len(t, history, 30)
	top, err := repo.ListSVGResults(ctx, config.ID, config.KeepCount)
	require.NoError(t, err)
	require.Len(t, top, 10)
	require.Equal(t, newestID, top[0].ID)
}

func TestAccountIntelligenceResultsPersistWithoutPublicConfig(t *testing.T) {
	ctx := context.Background()
	var accountID int64
	err := integrationDB.QueryRowContext(ctx, `INSERT INTO accounts (name, platform, type, credentials)
VALUES ('manual-svg-history', 'openai', 'apikey', '{}') RETURNING id`).Scan(&accountID)
	require.NoError(t, err)
	t.Cleanup(func() {
		_, err := integrationDB.ExecContext(ctx, `DELETE FROM intelligence_test_results WHERE account_id=$1`, accountID)
		require.NoError(t, err)
		_, err = integrationDB.ExecContext(ctx, `DELETE FROM accounts WHERE id=$1`, accountID)
		require.NoError(t, err)
	})
	repo := NewIntelligenceTestRepository(integrationDB)
	now := time.Now()
	result := &service.IntelligenceTestResult{
		AccountID: &accountID, AccountName: "manual-svg-history", PromptText: "Draw a bird",
		ModelID: "test-model", ModelName: "test-model", ReasoningEffort: "high",
		Status: "running", StartedAt: now, FinishedAt: now,
	}
	require.NoError(t, repo.InsertResult(ctx, result))
	result.Status, result.Check, result.SVGContent = "validated", true, "<svg/>"
	require.NoError(t, repo.UpdateAccountResult(ctx, result))

	history, err := repo.ListAccountResults(ctx, accountID, 0, 20)
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, "Draw a bird", history[0].PromptText)
	require.Equal(t, "<svg/>", history[0].SVGContent)
	require.Equal(t, "high", history[0].ReasoningEffort)
	require.True(t, history[0].Check)
	older, err := repo.ListAccountResults(ctx, accountID, result.ID, 20)
	require.NoError(t, err)
	require.Empty(t, older)
	otherAccount, err := repo.ListAccountResults(ctx, -accountID, 0, 20)
	require.NoError(t, err)
	require.Empty(t, otherAccount)
	public, err := repo.ListSVGResults(ctx, 0, 20)
	require.NoError(t, err)
	require.Empty(t, public, "manual results must not be reachable through a public config")
}

func TestCandyHistoryRetainsAllStatusesUpToMaximumDisplayLimit(t *testing.T) {
	ctx := context.Background()
	repo := NewIntelligenceTestRepository(integrationDB)
	config, err := repo.CreateConfig(ctx, &service.IntelligenceTestConfig{
		Name: "candy-history", AnswerMode: "candy", ModelID: "test-model", ModelName: "test-model",
		PromptText: "Candy puzzle", CronExpression: "0 * * * *", KeepCount: 60,
	})
	require.NoError(t, err)
	require.Equal(t, "candy", config.AnswerMode)
	t.Cleanup(func() { require.NoError(t, repo.DeleteConfig(ctx, config.ID)) })
	statuses := []string{"validated", "incorrect", "failed"}
	var newestID int64
	for i := 0; i < 105; i++ {
		now := time.Now()
		status := statuses[i%len(statuses)]
		answer := ""
		if status == "validated" {
			answer = "21"
		} else if status == "incorrect" {
			answer = "29"
		}
		result := &service.IntelligenceTestResult{
			ConfigID: config.ID, AnswerMode: "candy", ModelID: "test-model", ModelName: "test-model",
			Status: status, Check: status == "validated", ResponseText: answer, StartedAt: now, FinishedAt: now,
		}
		require.NoError(t, repo.InsertResult(ctx, result))
		require.NoError(t, repo.PruneResults(ctx, config.ID))
		newestID = result.ID
	}
	history, err := repo.ListResults(ctx, config.ID, config.KeepCount)
	require.NoError(t, err)
	require.Len(t, history, 60)
	require.Equal(t, newestID, history[0].ID)
	for _, item := range history {
		require.Equal(t, "candy", item.AnswerMode)
		if item.Check {
			require.Equal(t, "21", item.ResponseText)
		}
	}
	var count int
	err = integrationDB.QueryRowContext(ctx, `SELECT COUNT(*) FROM intelligence_test_results WHERE question_id=$1`, config.ID).Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 100, count)
	svg, err := repo.ListSVGResults(ctx, config.ID, 100)
	require.NoError(t, err)
	require.Empty(t, svg)
}

func TestIntelligenceTestScheduleUpdatesPersistAndSurviveRunningJob(t *testing.T) {
	ctx := context.Background()
	repo := NewIntelligenceTestRepository(integrationDB)
	oldNext := time.Now().Add(24 * time.Hour).Truncate(time.Microsecond)
	config, err := repo.CreateConfig(ctx, &service.IntelligenceTestConfig{
		Name: "candy-schedule", AnswerMode: "candy", ModelID: "test-model", ModelName: "test-model",
		PromptText: "Candy puzzle", CronExpression: "0 0 * * *", KeepCount: 20, Enabled: true, NextRunAt: &oldNext,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, repo.DeleteConfig(ctx, config.ID)) })
	newNext := time.Now().Add(time.Minute).Truncate(time.Microsecond)
	config.CronExpression, config.NextRunAt = "* * * * *", &newNext
	updated, err := repo.UpdateConfig(ctx, config)
	require.NoError(t, err)
	require.True(t, updated.NextRunAt.Equal(newNext))

	finished := time.Now().Truncate(time.Microsecond)
	require.NoError(t, repo.UpdateRun(ctx, config.ID, finished, oldNext, &oldNext))
	updated, err = repo.GetConfig(ctx, config.ID)
	require.NoError(t, err)
	require.True(t, updated.NextRunAt.Equal(newNext), "an old run must not overwrite the edited schedule")
	require.True(t, updated.LastRunAt.Equal(finished))
	due, err := repo.ListDueConfigs(ctx, newNext)
	require.NoError(t, err)
	found := false
	for _, item := range due {
		if item.ID == config.ID {
			found = true
		}
	}
	require.True(t, found, "the edited candy schedule must become due")

	next := newNext.Add(time.Minute)
	require.NoError(t, repo.UpdateRun(ctx, config.ID, newNext, next, &newNext))
	updated, err = repo.GetConfig(ctx, config.ID)
	require.NoError(t, err)
	require.True(t, updated.NextRunAt.Equal(next), "an unchanged schedule must advance after execution")
}
