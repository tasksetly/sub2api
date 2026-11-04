package service

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

const (
	fingerprintRunnerLogScope = "service.fingerprint_runner"
	fingerprintPlanLockTTL    = 6 * time.Minute
	fingerprintMaxWorkers     = 4
	fingerprintRunTimeout     = 5 * time.Minute
)

// fingerprintDetectFunc 是一次在线检测，抽成函数以便测试注入。
type fingerprintDetectFunc func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error)

// FingerprintMismatchNotifier 在判定不符时发告警。由 FingerprintAlerter 实现。
type FingerprintMismatchNotifier interface {
	NotifyMismatch(ctx context.Context, plan *FingerprintPlan, account *Account, result *FingerprintResult) error
}

// FingerprintRunnerService 扫描到期的指纹计划并执行。
//
// 形状照既有的 ScheduledTestRunnerService：每分钟 tick、ListDue、信号量并发、
// 计划级分布式锁、StartPlanNow 手动触发；但两者互不共用实例，语义也不混。
type FingerprintRunnerService struct {
	planRepo    FingerprintPlanRepository
	fpSvc       *FingerprintService
	accountRepo AccountRepository
	detector    *FingerprintDetectorProvider
	quota       *FingerprintQuota
	alerter     FingerprintMismatchNotifier

	cfg        *config.Config
	lockCache  LeaderLockCache
	db         *sql.DB
	instanceID string

	detectFn fingerprintDetectFunc

	cron       *cron.Cron
	startOnce  sync.Once
	stopOnce   sync.Once
	manualMu   sync.Mutex
	manualRuns map[int64]struct{}
}

// NewFingerprintRunnerService 创建执行器。
func NewFingerprintRunnerService(
	planRepo FingerprintPlanRepository,
	fpSvc *FingerprintService,
	accountRepo AccountRepository,
	detector *FingerprintDetectorProvider,
	quota *FingerprintQuota,
	cfg *config.Config,
	lockCache LeaderLockCache,
	db *sql.DB,
) *FingerprintRunnerService {
	s := &FingerprintRunnerService{
		planRepo:    planRepo,
		fpSvc:       fpSvc,
		accountRepo: accountRepo,
		detector:    detector,
		quota:       quota,
		cfg:         cfg,
		lockCache:   lockCache,
		db:          db,
		instanceID:  uuid.NewString(),
		manualRuns:  make(map[int64]struct{}),
	}
	s.detectFn = s.defaultDetect
	return s
}

// SetAlerter 注入告警通道（装配时调用，避免构造函数参数继续膨胀）。
func (s *FingerprintRunnerService) SetAlerter(alerter FingerprintMismatchNotifier) {
	if s != nil {
		s.alerter = alerter
	}
}

// defaultDetect 用常驻的 detector 发一轮检测。
func (s *FingerprintRunnerService) defaultDetect(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
	detector, err := s.detector.Get(ctx)
	if err != nil {
		return nil, err
	}
	return detector.Detect(ctx, cfg)
}

// Start 启动每分钟的调度 tick。
func (s *FingerprintRunnerService) Start() {
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
		if _, err := c.AddFunc("* * * * *", func() { s.runScheduled() }); err != nil {
			logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] not started (invalid schedule): %v", err)
			return
		}
		s.cron = c
		s.cron.Start()
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] started (tick=every minute)")
	})
}

// Stop 停止调度。
func (s *FingerprintRunnerService) Stop() {
	if s == nil {
		return
	}
	s.stopOnce.Do(func() {
		if s.cron != nil {
			ctx := s.cron.Stop()
			select {
			case <-ctx.Done():
			case <-time.After(3 * time.Second):
				logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] cron stop timed out")
			}
		}
	})
}

func (s *FingerprintRunnerService) runScheduled() {
	// 错开 30 秒，避免和定时测试的 tick 同时打到上游。
	time.Sleep(30 * time.Second)

	ctx, cancel := context.WithTimeout(context.Background(), fingerprintRunTimeout)
	defer cancel()

	plans, err := s.planRepo.ListDue(ctx, time.Now())
	if err != nil {
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] ListDue error: %v", err)
		return
	}
	if len(plans) == 0 {
		return
	}

	sem := make(chan struct{}, fingerprintMaxWorkers)
	var wg sync.WaitGroup
	for _, plan := range plans {
		sem <- struct{}{}
		wg.Add(1)
		go func(p *FingerprintPlan) {
			defer wg.Done()
			defer func() { <-sem }()
			s.runOnePlan(ctx, p)
		}(plan)
	}
	wg.Wait()
}

func (s *FingerprintRunnerService) runOnePlan(ctx context.Context, plan *FingerprintPlan) {
	release, acquired := s.acquirePlanLock(ctx, plan.ID)
	if !acquired {
		return
	}
	defer release()

	if _, err := s.executePlan(ctx, plan, false); err != nil {
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d execute error: %v", plan.ID, err)
	}

	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d computeNextRun error: %v", plan.ID, err)
		return
	}
	if err := s.planRepo.UpdateAfterRun(ctx, plan.ID, time.Now(), nextRun); err != nil {
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d UpdateAfterRun error: %v", plan.ID, err)
	}
}

// StartPlanNow 立刻在后台跑一次，不阻塞发起它的 HTTP 请求。
func (s *FingerprintRunnerService) StartPlanNow(ctx context.Context, planID int64) error {
	plan, err := s.planRepo.GetByID(ctx, planID)
	if err != nil {
		return err
	}

	s.manualMu.Lock()
	if _, running := s.manualRuns[planID]; running {
		s.manualMu.Unlock()
		return fmt.Errorf("该指纹校对正在执行中")
	}
	release, acquired := s.acquirePlanLock(ctx, planID)
	if !acquired {
		s.manualMu.Unlock()
		return fmt.Errorf("该指纹校对正在其他实例上执行")
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
		bgCtx, cancel := context.WithTimeout(context.Background(), fingerprintRunTimeout)
		defer cancel()
		if _, err := s.executePlan(bgCtx, plan, true); err != nil {
			logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] manual plan=%d execute error: %v", planID, err)
		}
	}()
	return nil
}

func (s *FingerprintRunnerService) acquirePlanLock(ctx context.Context, planID int64) (func(), bool) {
	lockCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	key := fmt.Sprintf("fingerprint:plan:%d", planID)
	return tryAcquireSingletonLeaderLock(lockCtx, s.lockCache, s.db, key, s.instanceID, fingerprintPlanLockTTL)
}

// executePlan 跑一次指纹校对并落库。
//
// manual 为真表示管理员手动触发：此时不检查账号是否可调度——他明确点了按钮。
func (s *FingerprintRunnerService) executePlan(ctx context.Context, plan *FingerprintPlan, manual bool) (*FingerprintResult, error) {
	startedAt := time.Now()

	account, err := s.accountRepo.GetByID(ctx, plan.AccountID)
	if err != nil || account == nil {
		return s.persist(ctx, plan, nil, &FingerprintResult{
			Verdict:      FingerprintVerdictError,
			ErrorMessage: "账号不存在或读取失败",
			StartedAt:    startedAt,
		})
	}

	if eligibility := CheckFingerprintEligibility(account); !eligibility.Supported {
		return s.persist(ctx, plan, account, &FingerprintResult{
			Verdict:      FingerprintVerdictError,
			ErrorMessage: fmt.Sprintf("账号不支持指纹校对: %s", eligibility.Reason),
			StartedAt:    startedAt,
		})
	}

	// 定期抽查跳过不可调度的账号：跑一个已停用的账号纯属烧钱，结论也没人用。
	if !manual && !account.IsSchedulable() {
		return s.persist(ctx, plan, account, &FingerprintResult{
			Verdict:    FingerprintVerdictError,
			SkipReason: FingerprintSkipAccountInactive,
			StartedAt:  startedAt,
		})
	}

	release, skipReason := s.quota.Acquire(ctx, plan.AccountID)
	defer release()
	if skipReason != "" {
		return s.persist(ctx, plan, account, &FingerprintResult{
			Verdict:    FingerprintVerdictError,
			SkipReason: skipReason,
			StartedAt:  startedAt,
		})
	}

	cfg, err := BuildDetectConfig(account, *plan)
	if err != nil {
		return s.persist(ctx, plan, account, &FingerprintResult{
			Verdict:      FingerprintVerdictError,
			ErrorMessage: err.Error(),
			StartedAt:    startedAt,
		})
	}

	detectStart := time.Now()
	detectRes, detectErr := s.detectFn(ctx, cfg)
	latency := time.Since(detectStart)

	result := EvaluateFingerprint(detectRes, detectErr, plan.ExpectedModel, s.fpSvc.KnownModelSet(ctx))
	result.StartedAt = startedAt
	result.LatencyMs = latency.Milliseconds()
	if result.LatencyMs <= 0 {
		result.LatencyMs = 1
	}
	// SDK 自己会把 Key 换成 [REDACTED]，但我们拼接的文本未必经过它。
	result.ErrorMessage = redactFingerprintSecrets(result.ErrorMessage, cfg.APIKey)

	return s.persist(ctx, plan, account, &result)
}

// persist 落库结果，然后按判定执行告警与动账号的动作。
func (s *FingerprintRunnerService) persist(ctx context.Context, plan *FingerprintPlan, account *Account, result *FingerprintResult) (*FingerprintResult, error) {
	result.FinishedAt = time.Now()
	if result.StartedAt.IsZero() {
		result.StartedAt = result.FinishedAt
	}

	if err := s.fpSvc.SaveResult(ctx, plan, result); err != nil {
		return result, err
	}
	s.updateAccountFingerprintStatus(ctx, plan.AccountID, result)

	if result.Verdict != FingerprintVerdictMismatch {
		// 相符时执行成功动作；无法判定与检测失败什么都不做。
		if result.Verdict == FingerprintVerdictMatch {
			s.applySuccessActions(ctx, plan)
		}
		return result, nil
	}

	// 告警是单次立即出，不等连续两次——管理员要第一时间知道。
	if s.alerter != nil && account != nil {
		if err := s.alerter.NotifyMismatch(ctx, plan, account, result); err != nil {
			logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d notify error: %v", plan.ID, err)
		}
	}

	// 动账号的动作要连续两次不符才执行。
	threshold := plan.MismatchActionThreshold
	if threshold <= 0 {
		threshold = FingerprintDefaultMismatchThreshold
	}
	if plan.ConsecutiveMismatches >= threshold {
		s.applyFailureActions(ctx, plan)
	}
	return result, nil
}

// updateAccountFingerprintStatus 把最近一次判定写进账号 extra，账号列表据此打标记。
//
// 标记按最近一次结果自动更新，不需要管理员手动确认——证据不会丢，
// 历史结果表里永久留存，手动确认只会多一套接口和交互。
// 被配额跳过的结果不覆盖标记：那次检测根本没发生。
func (s *FingerprintRunnerService) updateAccountFingerprintStatus(ctx context.Context, accountID int64, result *FingerprintResult) {
	if s.accountRepo == nil || result == nil || result.SkipReason != "" {
		return
	}
	extra := map[string]any{
		FingerprintStatusExtraKey:    result.Verdict,
		FingerprintCheckedAtExtraKey: time.Now().UTC().Format(time.RFC3339),
		FingerprintPredictedExtraKey: result.PredictedModel,
	}
	if err := s.accountRepo.UpdateExtra(ctx, accountID, extra); err != nil {
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] account=%d fingerprint status update failed: %v", accountID, err)
	}
}

// applyFailureActions 执行管理员在计划里开启的动账号动作。默认全关。
func (s *FingerprintRunnerService) applyFailureActions(ctx context.Context, plan *FingerprintPlan) {
	if s.accountRepo == nil {
		return
	}

	if len(plan.FailureRemoveGroupIDs) > 0 {
		if mutation, ok := s.accountRepo.(AccountGroupMutationRepository); ok {
			for _, groupID := range plan.FailureRemoveGroupIDs {
				if err := mutation.RemoveFromGroup(ctx, plan.AccountID, groupID); err != nil {
					logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d remove group=%d failed: %v", plan.ID, groupID, err)
				}
			}
		}
	}
	if plan.FailureDisable {
		updateAccountStatus(ctx, s.accountRepo, plan.AccountID, StatusDisabled, "指纹校对连续判定不符")
	}
	if plan.FailureTempUnschedulableMinutes > 0 {
		until := time.Now().Add(time.Duration(plan.FailureTempUnschedulableMinutes) * time.Minute)
		if err := s.accountRepo.SetTempUnschedulable(ctx, plan.AccountID, until, "指纹校对连续判定不符"); err != nil {
			logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d temp unschedulable failed: %v", plan.ID, err)
		}
	}
}

// applySuccessActions 只做加入分组。
//
// 刻意不提供"相符就启用账号"：既有定时测试有那个动作是因为它承担自动恢复探测，
// 而指纹计划里一次相符去启用管理员手工停用的账号是意外副作用，
// 与"停不停我自己决定"直接冲突。
func (s *FingerprintRunnerService) applySuccessActions(ctx context.Context, plan *FingerprintPlan) {
	if s.accountRepo == nil || len(plan.SuccessAddGroupIDs) == 0 {
		return
	}
	mutation, ok := s.accountRepo.(AccountGroupMutationRepository)
	if !ok {
		return
	}
	for _, groupID := range plan.SuccessAddGroupIDs {
		if err := mutation.AddToGroup(ctx, plan.AccountID, groupID, 1); err != nil {
			logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintRunner] plan=%d add group=%d failed: %v", plan.ID, groupID, err)
		}
	}
}

// redactFingerprintSecrets 把 API Key 从要落库或展示的文本里抹掉。
// 脱敏不能变成清空——界面上仍要看得到失败原因。
func redactFingerprintSecrets(text string, secret string) string {
	if text == "" || len(strings.TrimSpace(secret)) < 8 {
		return text
	}
	return strings.ReplaceAll(text, secret, "[REDACTED]")
}
