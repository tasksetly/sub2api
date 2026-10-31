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
