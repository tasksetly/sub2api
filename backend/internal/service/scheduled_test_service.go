package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

var scheduledTestCronParser = cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)

// ScheduledTestService provides CRUD operations for scheduled test plans and results.
type ScheduledTestService struct {
	planRepo   ScheduledTestPlanRepository
	resultRepo ScheduledTestResultRepository
}

// NewScheduledTestService creates a new ScheduledTestService.
func NewScheduledTestService(
	planRepo ScheduledTestPlanRepository,
	resultRepo ScheduledTestResultRepository,
) *ScheduledTestService {
	return &ScheduledTestService{
		planRepo:   planRepo,
		resultRepo: resultRepo,
	}
}

// CreatePlan validates the cron expression, computes next_run_at, and persists the plan.
func (s *ScheduledTestService) CreatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	if err := validateScheduledTestActions(plan); err != nil { return nil, err }
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	plan.NextRunAt = &nextRun

	if plan.MaxResults <= 0 {
		plan.MaxResults = 50
	}
	if plan.TempUnschedulableMinutes <= 0 { plan.TempUnschedulableMinutes = 5 }

	return s.planRepo.Create(ctx, plan)
}

// GetPlan retrieves a plan by ID.
func (s *ScheduledTestService) GetPlan(ctx context.Context, id int64) (*ScheduledTestPlan, error) {
	return s.planRepo.GetByID(ctx, id)
}

// ListPlansByAccount returns all plans for a given account.
func (s *ScheduledTestService) ListPlansByAccount(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error) {
	return s.planRepo.ListByAccountID(ctx, accountID)
}

// UpdatePlan validates cron and updates the plan.
func (s *ScheduledTestService) UpdatePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error) {
	if err := validateScheduledTestActions(plan); err != nil { return nil, err }
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("invalid cron expression: %w", err)
	}
	plan.NextRunAt = &nextRun

	return s.planRepo.Update(ctx, plan)
}

func validateScheduledTestActions(plan *ScheduledTestPlan) error {
	if plan.TempUnschedulableMinutes < 0 || plan.TempUnschedulableMinutes > 10080 { return fmt.Errorf("temporary unschedulable duration must be between 1 and 10080 minutes") }
	if plan.FailureTempUnschedulableMinutes < 0 || plan.FailureTempUnschedulableMinutes > 10080 { return fmt.Errorf("failure temporary unschedulable duration must be between 1 and 10080 minutes") }
	for _, code := range plan.TempUnschedulableErrorCodes { if code < 100 || code > 599 { return fmt.Errorf("error codes must be between 100 and 599") } }
	if plan.TempUnschedulableEnabled && len(plan.TempUnschedulableErrorCodes) == 0 { return fmt.Errorf("select at least one error code") }
	if plan.PromptText != "" && plan.ExpectedAnswer == "" { return fmt.Errorf("expected answer is required when a prompt is configured") }
	if plan.FailureRemoveGroupID != nil && *plan.FailureRemoveGroupID <= 0 { return fmt.Errorf("failure group id must be positive") }
	if plan.SuccessAddGroupID != nil && *plan.SuccessAddGroupID <= 0 { return fmt.Errorf("success group id must be positive") }
	seenGroupIDs := make(map[int64]struct{}, len(plan.SuccessAddGroupIDs))
	groupIDs := make([]int64, 0, len(plan.SuccessAddGroupIDs))
	for _, groupID := range plan.SuccessAddGroupIDs {
		if groupID <= 0 {
			return fmt.Errorf("success group ids must be positive")
		}
		if _, exists := seenGroupIDs[groupID]; !exists {
			seenGroupIDs[groupID] = struct{}{}
			groupIDs = append(groupIDs, groupID)
		}
	}
	plan.SuccessAddGroupIDs = groupIDs
	return nil
}

// DeletePlan removes a plan and its results (via CASCADE).
func (s *ScheduledTestService) DeletePlan(ctx context.Context, id int64) error {
	return s.planRepo.Delete(ctx, id)
}

// ListResults returns the most recent results for a plan.
func (s *ScheduledTestService) ListResults(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error) {
	if limit <= 0 {
		limit = 50
	}
	return s.resultRepo.ListByPlanID(ctx, planID, limit)
}

func (s *ScheduledTestService) ListPromptTemplates(ctx context.Context) ([]*ScheduledTestPromptTemplate, error) {
	return s.planRepo.ListPromptTemplates(ctx)
}

func (s *ScheduledTestService) CreatePromptTemplate(ctx context.Context, template *ScheduledTestPromptTemplate) (*ScheduledTestPromptTemplate, error) {
	if template == nil || strings.TrimSpace(template.Name) == "" || strings.TrimSpace(template.PromptText) == "" {
		return nil, fmt.Errorf("template name and prompt are required")
	}
	template.Name = strings.TrimSpace(template.Name)
	template.PromptText = strings.TrimSpace(template.PromptText)
	template.ExpectedAnswer = strings.TrimSpace(template.ExpectedAnswer)
	return s.planRepo.CreatePromptTemplate(ctx, template)
}

func (s *ScheduledTestService) DeletePromptTemplate(ctx context.Context, id int64) error {
	return s.planRepo.DeletePromptTemplate(ctx, id)
}

// SaveResult inserts a result and prunes old entries beyond maxResults.
func (s *ScheduledTestService) SaveResult(ctx context.Context, planID int64, maxResults int, result *ScheduledTestResult) error {
	result.PlanID = planID
	created, err := s.resultRepo.Create(ctx, result)
	if err != nil {
		return err
	}
	*result = *created
	return s.resultRepo.PruneOldResults(ctx, planID, maxResults)
}

func computeNextRun(cronExpr string, from time.Time) (time.Time, error) {
	sched, err := scheduledTestCronParser.Parse(cronExpr)
	if err != nil {
		return time.Time{}, err
	}
	return sched.Next(from), nil
}
