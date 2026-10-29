package service

import (
	"context"
	"time"
)

const (
	IntelligenceStatusExtraKey = "scheduled_test_intelligence_status"
	IntelligenceStatusOnline   = "online"
	IntelligenceStatusOffline  = "offline"
)

// ScheduledTestPlan represents a scheduled test plan domain model.
type ScheduledTestPlan struct {
	ID             int64      `json:"id"`
	AccountID      int64      `json:"account_id"`
	ModelID        string     `json:"model_id"`
	PromptText     string     `json:"prompt_text"`
	ExpectedAnswer string     `json:"expected_answer"`
	CronExpression string     `json:"cron_expression"`
	Enabled        bool       `json:"enabled"`
	MaxResults     int        `json:"max_results"`
	AutoRecover    bool       `json:"auto_recover"`
	TempUnschedulableEnabled bool `json:"temp_unschedulable_enabled"`
	TempUnschedulableErrorCodes []int `json:"temp_unschedulable_error_codes"`
	TempUnschedulableMinutes int `json:"temp_unschedulable_minutes"`
	FailureRemoveGroupID *int64 `json:"failure_remove_group_id,omitempty"`
	FailureDisable bool `json:"failure_disable"`
	FailureTempUnschedulableMinutes int `json:"failure_temp_unschedulable_minutes"`
	SuccessAddGroupID *int64 `json:"success_add_group_id,omitempty"`
	SuccessEnable bool `json:"success_enable"`
	SuccessRecover bool `json:"success_recover"`
	LastRunAt      *time.Time `json:"last_run_at"`
	NextRunAt      *time.Time `json:"next_run_at"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// ScheduledTestResult represents a single test execution result.
type ScheduledTestResult struct {
	ID           int64     `json:"id"`
	PlanID       int64     `json:"plan_id"`
	Status       string    `json:"status"`
	ResponseText string    `json:"response_text"`
	ErrorMessage string    `json:"error_message"`
	ErrorCode    int       `json:"error_code"`
	CheckOK      bool      `json:"check_ok"`
	LatencyMs    int64     `json:"latency_ms"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at"`
	CreatedAt    time.Time `json:"created_at"`
}

// ScheduledTestPlanRepository defines the data access interface for test plans.
type ScheduledTestPlanRepository interface {
	Create(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	GetByID(ctx context.Context, id int64) (*ScheduledTestPlan, error)
	ListByAccountID(ctx context.Context, accountID int64) ([]*ScheduledTestPlan, error)
	ListDue(ctx context.Context, now time.Time) ([]*ScheduledTestPlan, error)
	Update(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestPlan, error)
	Delete(ctx context.Context, id int64) error
	UpdateAfterRun(ctx context.Context, id int64, lastRunAt time.Time, nextRunAt time.Time) error
}

// ScheduledTestResultRepository defines the data access interface for test results.
type ScheduledTestResultRepository interface {
	Create(ctx context.Context, result *ScheduledTestResult) (*ScheduledTestResult, error)
	ListByPlanID(ctx context.Context, planID int64, limit int) ([]*ScheduledTestResult, error)
	PruneOldResults(ctx context.Context, planID int64, keepCount int) error
}
