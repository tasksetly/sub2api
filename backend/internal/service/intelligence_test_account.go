package service

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type AccountIntelligenceTestInput struct {
	ModelID         string `json:"model_id"`
	PromptText      string `json:"prompt_text"`
	ReasoningEffort string `json:"reasoning_effort"`
}

func (input *AccountIntelligenceTestInput) normalize() error {
	input.ModelID = strings.TrimSpace(input.ModelID)
	input.PromptText = strings.TrimSpace(input.PromptText)
	input.ReasoningEffort = strings.ToLower(strings.TrimSpace(input.ReasoningEffort))
	if input.ModelID == "" || utf8.RuneCountInString(input.ModelID) > 200 || input.PromptText == "" || utf8.RuneCountInString(input.PromptText) > 20000 {
		return infraerrors.BadRequest("INTELLIGENCE_TEST_INPUT_INVALID", "model (up to 200 characters) and prompt (up to 20000 characters) are required")
	}
	switch input.ReasoningEffort {
	case "", "low", "medium", "high", "xhigh", "max":
		return nil
	default:
		return infraerrors.BadRequest("INTELLIGENCE_TEST_EFFORT_INVALID", "invalid reasoning effort")
	}
}

func (s *IntelligenceTestService) lockAccountRun(ctx context.Context, accountID int64) (func(), bool, error) {
	if s.db == nil {
		return nil, false, errors.New("intelligence test database lock is unavailable")
	}
	return tryAcquireDBAdvisoryLockWithError(ctx, s.db, hashAdvisoryLockID("intelligence_test:account:"+strconv.FormatInt(accountID, 10)))
}

func (s *IntelligenceTestService) QueueAccountRun(ctx context.Context, accountID int64, input AccountIntelligenceTestInput) (*IntelligenceTestResult, error) {
	if err := input.normalize(); err != nil {
		return nil, err
	}
	account, err := s.accountRepo.GetByID(ctx, accountID)
	if err != nil {
		return nil, err
	}
	model := account.GetMappedModel(input.ModelID)
	if account.IsTypeSafe() || isOpenAIImageModel(model) || isGeminiImageOutputModel(model) || isGrokImageGenerationModel(model) || isGrokVideoGenerationModel(model) {
		return nil, infraerrors.BadRequest("INTELLIGENCE_TEST_MODEL_UNSUPPORTED", "SVG tests require an account and model that support text generation with system instructions")
	}
	if s.accountTest == nil {
		return nil, errors.New("account test service unavailable")
	}
	release, locked, err := s.lockAccountRun(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if !locked {
		return nil, ErrIntelligenceTestAlreadyRunning
	}
	if err := s.repo.FailInterruptedAccountResults(ctx, accountID); err != nil {
		release()
		return nil, err
	}
	now := time.Now()
	result := &IntelligenceTestResult{
		AccountID: &accountID, AccountName: account.Name, PromptText: input.PromptText,
		ModelID: input.ModelID, ModelName: input.ModelID, ReasoningEffort: input.ReasoningEffort,
		Status: "running", StartedAt: now, FinishedAt: now, CreatedAt: now,
	}
	if err := s.repo.InsertResult(ctx, result); err != nil {
		release()
		return nil, err
	}
	// Return an immutable snapshot while the worker updates its own copy.
	queued := *result
	go func() {
		defer release()
		jobCtx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		run, runErr := s.accountTest.RunTestBackgroundWithOptions(jobCtx, accountID, input.ModelID, input.PromptText, AccountTestOptions{
			ReasoningEffort: input.ReasoningEffort,
			SystemPrompt:    intelligenceTestSVGSystemPrompt,
		})
		applyIntelligenceTestRun(jobCtx, result, run, runErr)
		writeCtx, cancelWrite := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancelWrite()
		if err := s.repo.UpdateAccountResult(writeCtx, result); err != nil {
			slog.Error("account_intelligence_test_save_failed", "result_id", result.ID, "error", err)
		}
	}()
	return &queued, nil
}

func (s *IntelligenceTestService) ListAccountResults(ctx context.Context, accountID, beforeID int64) ([]*IntelligenceTestResult, error) {
	release, locked, err := s.lockAccountRun(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if locked {
		// A restart releases the lock; mark unfinished rows instead of polling forever.
		err = s.repo.FailInterruptedAccountResults(ctx, accountID)
		release()
		if err != nil {
			return nil, err
		}
	}
	return s.repo.ListAccountResults(ctx, accountID, beforeID, 20)
}

// Both scheduled and manual tests persist only extracted SVG and sanitized errors.
func applyIntelligenceTestRun(ctx context.Context, result *IntelligenceTestResult, run *ScheduledTestResult, runErr error) {
	result.Status = "failed"
	result.ErrorMessage = "upstream test failed"
	result.FinishedAt = time.Now()
	result.LatencyMs = result.FinishedAt.Sub(result.StartedAt).Milliseconds()
	if runErr == nil && run != nil {
		result.LatencyMs = run.LatencyMs
		result.StartedAt, result.FinishedAt = run.StartedAt, run.FinishedAt
		if run.Status == "success" {
			result.SVGContent = extractSVG(run.ResponseText)
			result.Check = result.SVGContent != ""
			if result.Check {
				result.Status, result.ErrorMessage = "validated", ""
			} else {
				result.ErrorMessage = "svg_missing_or_invalid"
			}
		} else if run.ErrorCode >= 100 {
			result.ErrorMessage = "upstream HTTP " + strconv.Itoa(run.ErrorCode)
		}
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		result.Status, result.Check, result.SVGContent = "failed", false, ""
		result.ErrorMessage = "benchmark timed out"
	}
}
