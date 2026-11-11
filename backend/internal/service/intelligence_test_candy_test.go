package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCandyAnswerPasses(t *testing.T) {
	for _, answer := range []string{"21", "答案是21颗。", "**21**", "The answer is 21.", "```\n21\n```", "先算 121，再算 21。"} {
		require.True(t, candyAnswerPasses(answer), answer)
	}
	for _, answer := range []string{"", "20", "121", "210", "21.5", "0.21", "21.0", "1,021", "-21", "1e21", "21e3", "id21", "21st", "21_value", "二十一"} {
		require.False(t, candyAnswerPasses(answer), answer)
	}
}

func TestCandyRunSeparatesIncorrectAnswersFromFailures(t *testing.T) {
	for _, tc := range []struct {
		name, upstream, answer, status string
		err                            error
	}{
		{"pass", "success", "答案是21颗。", "validated", nil},
		{"wrong", "success", "29", "incorrect", nil},
		{"decimal", "success", "21.5", "incorrect", nil},
		{"empty", "success", " \n", "failed", nil},
		{"upstream error", "failed", "21 and private error body", "failed", nil},
		{"transport error", "success", "21", "failed", errors.New("private upstream error")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			now := time.Now()
			result := &IntelligenceTestResult{AnswerMode: "candy", StartedAt: now}
			applyIntelligenceTestRun(context.Background(), result, &ScheduledTestResult{
				Status: tc.upstream, ResponseText: tc.answer, StartedAt: now, FinishedAt: now, LatencyMs: 42,
			}, tc.err)
			require.Equal(t, tc.status, result.Status)
			require.Equal(t, tc.status == "validated", result.Check)
			require.Empty(t, result.SVGContent)
			if tc.status == "failed" {
				require.Empty(t, result.ResponseText)
			} else {
				require.Equal(t, tc.answer, result.ResponseText)
			}
		})
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	result := &IntelligenceTestResult{AnswerMode: "candy", StartedAt: time.Now()}
	applyIntelligenceTestRun(ctx, result, &ScheduledTestResult{Status: "success", ResponseText: "21"}, nil)
	require.Equal(t, "failed", result.Status)
	require.False(t, result.Check)
	require.Empty(t, result.ResponseText)
}

func TestIntelligenceTestCandyConfig(t *testing.T) {
	svc := &IntelligenceTestService{}
	input := IntelligenceTestConfigInput{Name: "Candy", ModelID: "model", APIKeyID: 1, AnswerMode: "candy", PromptText: "Ignore the puzzle"}
	config, err := svc.normalizeConfig(input)
	require.NoError(t, err)
	require.Equal(t, intelligenceTestCandyPrompt, config.PromptText)
	require.NotContains(t, config.PromptText, "21", "the prompt must not disclose the expected answer")
	require.Equal(t, "candy", config.AnswerMode)
	input.AnswerMode = ""
	config, err = svc.normalizeConfig(input)
	require.NoError(t, err)
	require.Equal(t, "svg", config.AnswerMode)
	input.AnswerMode = "unsupported"
	_, err = svc.normalizeConfig(input)
	require.Error(t, err)

	repo := &intelligenceTestLockRepo{config: &IntelligenceTestConfig{AnswerMode: "svg", Name: "Original"}}
	svc.repo = repo
	input.AnswerMode = "candy"
	_, err = svc.UpdateConfig(context.Background(), 1, input)
	require.Error(t, err)
	require.Equal(t, "Original", repo.config.Name)
}

type candyPublicRepo struct {
	IntelligenceTestRepository
	configs   []*IntelligenceTestConfig
	results   map[int64][]*IntelligenceTestResult
	listed    []int64
	svgListed []int64
}

func (r *candyPublicRepo) ListConfigs(context.Context) ([]*IntelligenceTestConfig, error) {
	return r.configs, nil
}
func (r *candyPublicRepo) GetConfig(_ context.Context, id int64) (*IntelligenceTestConfig, error) {
	for _, c := range r.configs {
		if c.ID == id {
			return c, nil
		}
	}
	return nil, ErrIntelligenceTestConfigNotFound
}
func (r *candyPublicRepo) ListResults(_ context.Context, id int64, _ int) ([]*IntelligenceTestResult, error) {
	r.listed = append(r.listed, id)
	return r.results[id], nil
}
func (r *candyPublicRepo) ListSVGResults(_ context.Context, id int64, _ int) ([]*IntelligenceTestResult, error) {
	r.svgListed = append(r.svgListed, id)
	return r.results[id], nil
}

func TestCandyPublicHistoryIsolatedFromSVGAndPrivateMetadata(t *testing.T) {
	groupID := int64(7)
	repo := &candyPublicRepo{
		configs: []*IntelligenceTestConfig{
			{ID: 1, AnswerMode: "svg", Enabled: true, KeepCount: 20},
			{ID: 2, AnswerMode: "candy", Enabled: true, KeepCount: 60, GroupID: &groupID, GroupName: "Group"},
			{ID: 3, AnswerMode: "candy", Enabled: false},
		},
		results: map[int64][]*IntelligenceTestResult{
			1: {{ID: 11, AnswerMode: "svg", Status: "validated", Check: true, SVGContent: "<svg/>"}},
			2: {
				{ID: 21, AnswerMode: "candy", Status: "validated", Check: true, ResponseText: "21", APIKeyID: 999, AccountName: "private-account", ErrorMessage: "private-error"},
				{ID: 22, AnswerMode: "candy", Status: "incorrect", ResponseText: "29"},
				{ID: 23, AnswerMode: "candy", Status: "failed"},
			},
		},
	}
	svc := &IntelligenceTestService{repo: repo}
	response, err := svc.Public(context.Background(), 0, "candy")
	require.NoError(t, err)
	require.Equal(t, []int64{2}, repo.listed)
	require.Empty(t, repo.svgListed)
	require.Len(t, response.Items, 3)
	require.Len(t, response.Groups, 1)
	require.Equal(t, intelligenceTestCandyPrompt, response.Config.PromptText)
	require.Equal(t, 60, response.KeepCount)
	encoded, err := json.Marshal(response)
	require.NoError(t, err)
	for _, secret := range []string{"api_key_id", "account_name", "private-account", "private-error"} {
		require.NotContains(t, string(encoded), secret)
	}

	response, err = svc.Public(context.Background(), 0, "")
	require.NoError(t, err)
	require.Equal(t, []int64{1}, repo.svgListed)
	require.Len(t, response.Items, 1)
	require.Equal(t, "<svg/>", response.Items[0].SVGContent)
	for _, id := range []int64{1, 3, 404} {
		response, err = svc.Public(context.Background(), id, "candy")
		require.NoError(t, err)
		require.Empty(t, response.Items)
	}
	_, err = svc.Public(context.Background(), 0, "unsupported")
	require.Error(t, err)
}
