package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

const scheduledTestDefaultMaxWorkers = 10
const scheduledTestPlanLockTTL = 6 * time.Minute

// ScheduledTestRunnerService periodically scans due test plans and executes them.
type ScheduledTestRunnerService struct {
	planRepo       ScheduledTestPlanRepository
	scheduledSvc   *ScheduledTestService
	accountTestSvc *AccountTestService
	rateLimitSvc   *RateLimitService
	cfg            *config.Config
	lockCache      LeaderLockCache
	db             *sql.DB
	instanceID     string

	cron      *cron.Cron
	startOnce sync.Once
	stopOnce  sync.Once
	manualMu  sync.Mutex
	manualRuns map[int64]struct{}
}

// NewScheduledTestRunnerService creates a new runner.
func NewScheduledTestRunnerService(
	planRepo ScheduledTestPlanRepository,
	scheduledSvc *ScheduledTestService,
	accountTestSvc *AccountTestService,
	rateLimitSvc *RateLimitService,
	cfg *config.Config,
	lockCache LeaderLockCache,
	db *sql.DB,
) *ScheduledTestRunnerService {
	return &ScheduledTestRunnerService{
		planRepo:       planRepo,
		scheduledSvc:   scheduledSvc,
		accountTestSvc: accountTestSvc,
		rateLimitSvc:   rateLimitSvc,
		cfg:            cfg,
		lockCache:      lockCache,
		db:             db,
		instanceID:     uuid.NewString(),
		manualRuns:     make(map[int64]struct{}),
	}
}

// Start begins the cron ticker (every minute).
func (s *ScheduledTestRunnerService) Start() {
	if s == nil {
		return
	}
	s.startOnce.Do(func() {
		loc := time.Local
		if s.cfg != nil {
			if parsed, err := time.LoadLocation(s.cfg.Timezone); err == nil && parsed != nil {
				loc = parsed
			}
		}

		c := cron.New(cron.WithParser(scheduledTestCronParser), cron.WithLocation(loc))
		_, err := c.AddFunc("* * * * *", func() { s.runScheduled() })
		if err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] started (tick=every minute)")
	})
}

// Stop gracefully shuts down the cron scheduler.
func (s *ScheduledTestRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] cron stop timed out")
			}
		}
	})
}

func (s *ScheduledTestRunnerService) runScheduled() {
	// Delay 10s so execution lands at ~:10 of each minute instead of :00.
	time.Sleep(10 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	now := time.Now()
	plans, err := s.planRepo.ListDue(ctx, now)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] found %d due plans", len(plans))

	sem := make(chan struct{}, scheduledTestDefaultMaxWorkers)
	var wg sync.WaitGroup

	for _, plan := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *ScheduledTestPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlan(ctx, p)
		}(plan)
	}

	wg.Wait()
}

func (s *ScheduledTestRunnerService) runOnePlan(ctx context.Context, plan *ScheduledTestPlan) {
	release, acquired := s.acquirePlanLock(ctx, plan.ID)
	if !acquired {
		return
	}
	defer release()
	result, err := s.executePlan(ctx, plan)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d execute error: %v", plan.ID, err)
	}
	if result == nil {
		return
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}
	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

// StartPlanNow starts an immediate test independently of the initiating HTTP
// request. The result is persisted when execution finishes.
func (s *ScheduledTestRunnerService) StartPlanNow(ctx context.Context, planID int64) error {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return err
	}
	s.manualMu.Lock()
	if _, running := s.manualRuns[planID]; running {
		s.manualMu.Unlock()
		return fmt.Errorf("scheduled test is already running")
	}
	release, acquired := s.acquirePlanLock(ctx, planID)
	if !acquired {
		s.manualMu.Unlock()
		return fmt.Errorf("scheduled test is already running on another instance")
	}
	s.manualRuns[planID] = struct{}{}
	s.manualMu.Unlock()
	go func() {
		defer func() {
			release()
			s.manualMu.Lock()
			delete(s.manualRuns, planID)
			s.manualMu.Unlock()
		}()
		backgroundCtx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if _, err := s.executePlan(backgroundCtx, plan); err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] manual plan=%d execute error: %v", planID, err)
		}
	}()
	return nil
}

func (s *ScheduledTestRunnerService) acquirePlanLock(ctx context.Context, planID int64) (func(), bool) {
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := fmt.Sprintf("scheduled_test:plan:%d", planID)
	return tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, key, s.instanceID, scheduledTestPlanLockTTL)
}

func (s *ScheduledTestRunnerService) executePlan(ctx context.Context, plan *ScheduledTestPlan) (*ScheduledTestResult, error) {
	if s.accountTestSvc == nil {
		return nil, fmt.Errorf("account test service is unavailable")
	}
	result, err := s.accountTestSvc.RunTestBackgroundWithOptions(ctx, plan.AccountID, plan.ModelID, plan.PromptText, AccountTestOptions{ReasoningEffort: plan.ReasoningEffort})
	if err != nil {
		return nil, err
	}
	connectivityOK := result.Status == "success"
	result.CheckOK = connectivityOK
	if strings.TrimSpace(plan.PromptText) != "" {
		result.CheckOK = result.CheckOK && normalizeScheduledTestAnswer(result.ResponseText) == normalizeScheduledTestAnswer(plan.ExpectedAnswer)
		if !result.CheckOK {
			result.Status = "failed"
			if result.ErrorMessage == "" {
				result.ErrorMessage = "prompt answer mismatch"
			}
		}
	}

	if err := s.scheduledSvc.SaveResult(ctx, plan.ID, plan.MaxResults, result); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d SaveResult error: %v", plan.ID, err)
	}
	s.updateIntelligenceStatus(ctx, plan.AccountID, plan, result)

	if result.Status == "failed" {
		s.applyFailureActions(ctx, plan, result)
	} else {
		s.applySuccessActions(ctx, plan)
	}

	if !connectivityOK && plan.TempUnschedulableEnabled && containsInt(plan.TempUnschedulableErrorCodes, result.ErrorCode) {
		duration := time.Duration(plan.TempUnschedulableMinutes) * time.Minute
		if plan.TempUnschedulableMinutes <= 0 { duration = scheduledTestFailureCooldown }
		if s.rateLimitSvc != nil {
			if err := s.rateLimitSvc.MarkAccountTemporarilyUnschedulableAfterScheduledTestFailureFor(ctx, plan.AccountID, duration); err != nil {
				logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d temporary unschedulable failed: %v", plan.ID, err)
			}
		}
	} else if connectivityOK {
		s.clearScheduledTestTemporaryUnschedulable(ctx, plan.AccountID, plan.ID)
	}

	// Auto-recover account if test succeeded and auto_recover is enabled.
	if connectivityOK && plan.AutoRecover {
		s.tryRecoverAccount(ctx, plan.AccountID, plan.ID)
	}
	return result, nil
}

func normalizeScheduledTestAnswer(value string) string {
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(value), "`"))
}

func (s *ScheduledTestRunnerService) accountRepo() AccountRepository {
	if s == nil || s.accountTestSvc == nil {
		return nil
	}
	return s.accountTestSvc.accountRepo
}

func (s *ScheduledTestRunnerService) updateIntelligenceStatus(ctx context.Context, accountID int64, plan *ScheduledTestPlan, result *ScheduledTestResult) {
	if plan == nil || strings.TrimSpace(plan.PromptText) == "" {
		return
	}
	repo := s.accountRepo()
	if repo == nil {
		return
	}
	status := IntelligenceStatusOffline
	if result.CheckOK {
		status = IntelligenceStatusOnline
	}
	if err := repo.UpdateExtra(ctx, accountID, map[string]any{IntelligenceStatusExtraKey: status}); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] account=%d intelligence status update failed: %v", accountID, err)
	}
}

func (s *ScheduledTestRunnerService) applyFailureActions(ctx context.Context, plan *ScheduledTestPlan, result *ScheduledTestResult) {
	repo := s.accountRepo()
	if repo == nil {
		return
	}
	groupIDs := append([]int64(nil), plan.FailureRemoveGroupIDs...)
	if len(groupIDs) == 0 && plan.FailureRemoveGroupID != nil {
		groupIDs = []int64{*plan.FailureRemoveGroupID}
	}
	if len(groupIDs) > 0 {
		if mutation, ok := repo.(AccountGroupMutationRepository); ok {
			for _, groupID := range groupIDs {
				if err := mutation.RemoveFromGroup(ctx, plan.AccountID, groupID); err != nil {
					logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d remove group=%d failed: %v", plan.ID, groupID, err)
				}
			}
		}
	}
	if plan.FailureDisable {
		updateAccountStatus(ctx, repo, plan.AccountID, StatusDisabled, "scheduled test failed")
	}
	if plan.FailureTempUnschedulableMinutes > 0 {
		until := time.Now().Add(time.Duration(plan.FailureTempUnschedulableMinutes) * time.Minute)
		if err := repo.SetTempUnschedulable(ctx, plan.AccountID, until, "scheduled test prompt failed"); err != nil {
			logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d failure temporary unschedulable failed: %v", plan.ID, err)
		}
	}
	_ = result
}

func (s *ScheduledTestRunnerService) applySuccessActions(ctx context.Context, plan *ScheduledTestPlan) {
	repo := s.accountRepo()
	if repo == nil {
		return
	}
	groupIDs := plan.SuccessAddGroupIDs
	if len(groupIDs) == 0 && plan.SuccessAddGroupID != nil {
		groupIDs = []int64{*plan.SuccessAddGroupID}
	}
	if len(groupIDs) > 0 {
		if mutation, ok := repo.(AccountGroupMutationRepository); ok {
			for _, groupID := range groupIDs {
				if err := mutation.AddToGroup(ctx, plan.AccountID, groupID, 1); err != nil {
					logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d add group=%d failed: %v", plan.ID, groupID, err)
				}
			}
		}
	}
	if plan.SuccessEnable {
		updateAccountStatus(ctx, repo, plan.AccountID, StatusActive, "")
	}
	if plan.SuccessRecover {
		if err := repo.ClearError(ctx, plan.AccountID); err != nil { logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d recover error status failed: %v", plan.ID, err) }
		if err := repo.ClearRateLimit(ctx, plan.AccountID); err != nil { logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d recover rate limit failed: %v", plan.ID, err) }
		if err := repo.ClearTempUnschedulable(ctx, plan.AccountID); err != nil { logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d recover temporary unschedulable failed: %v", plan.ID, err) }
		if err := repo.SetSchedulable(ctx, plan.AccountID, true); err != nil { logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d restore scheduling failed: %v", plan.ID, err) }
	}
}

func updateAccountStatus(ctx context.Context, repo AccountRepository, accountID int64, status, errorMessage string) {
	account, err := repo.GetByID(ctx, accountID)
	if err != nil || account == nil {
		return
	}
	account.Status = status
	account.ErrorMessage = errorMessage
	if err := repo.Update(ctx, account); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] account=%d status update failed: %v", accountID, err)
	}
}

func containsInt(values []int, target int) bool {
	for _, value := range values { if value == target { return true } }
	return false
}

func (s *ScheduledTestRunnerService) clearScheduledTestTemporaryUnschedulable(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}
	if err := s.rateLimitSvc.ClearScheduledTestFailureTemporaryUnschedulable(ctx, accountID); err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d clear scheduled-test temporary unschedulable failed: %v", planID, err)
	}
}

// tryRecoverAccount attempts to recover an account from recoverable runtime state.
func (s *ScheduledTestRunnerService) tryRecoverAccount(ctx context.Context, accountID int64, planID int64) {
	if s.rateLimitSvc == nil {
		return
	}

	recovery, err := s.rateLimitSvc.RecoverAccountAfterSuccessfulTest(ctx, accountID)
	if err != nil {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover failed: %v", planID, err)
		return
	}
	if recovery == nil {
		return
	}

	if recovery.ClearedError {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d recovered from error status", planID, accountID)
	}
	if recovery.ClearedRateLimit {
		logger.LegacyPrintf("service.scheduled_test_runner", "[ScheduledTestRunner] plan=%d auto-recover: account=%d cleared rate-limit/runtime state", planID, accountID)
	}
}
