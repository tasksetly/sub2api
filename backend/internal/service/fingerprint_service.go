package service

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// FingerprintModelCatalog 提供参考库里的模型清单。
// *FingerprintDetectorProvider 实现了它。
type FingerprintModelCatalog interface {
	ListReferenceModels(ctx context.Context) ([]FingerprintRefModel, error)
}

// FingerprintService 提供指纹校对计划的 CRUD 与结果留存。
type FingerprintService struct {
	planRepo   FingerprintPlanRepository
	resultRepo FingerprintResultRepository
	catalog    FingerprintModelCatalog
}

// NewFingerprintService 创建服务。
func NewFingerprintService(
	planRepo FingerprintPlanRepository,
	resultRepo FingerprintResultRepository,
	catalog FingerprintModelCatalog,
) *FingerprintService {
	return &FingerprintService{planRepo: planRepo, resultRepo: resultRepo, catalog: catalog}
}

// CreatePlan 校验并创建计划。
func (s *FingerprintService) CreatePlan(ctx context.Context, plan *FingerprintPlan) (*FingerprintPlan, error) {
	if err := s.validatePlan(ctx, plan); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("cron 表达式非法: %w", err)
	}
	plan.NextRunAt = &nextRun
	return s.planRepo.Create(ctx, plan)
}

// GetPlan 按 ID 取计划。
func (s *FingerprintService) GetPlan(ctx context.Context, id int64) (*FingerprintPlan, error) {
	return s.planRepo.GetByID(ctx, id)
}

// ListPlansByAccount 列出某账号的全部计划。
func (s *FingerprintService) ListPlansByAccount(ctx context.Context, accountID int64) ([]*FingerprintPlan, error) {
	return s.planRepo.ListByAccountID(ctx, accountID)
}

// UpdatePlan 校验并更新计划。
func (s *FingerprintService) UpdatePlan(ctx context.Context, plan *FingerprintPlan) (*FingerprintPlan, error) {
	if err := s.validatePlan(ctx, plan); err != nil {
		return nil, err
	}
	nextRun, err := computeNextRun(plan.CronExpression, time.Now())
	if err != nil {
		return nil, fmt.Errorf("cron 表达式非法: %w", err)
	}
	plan.NextRunAt = &nextRun
	return s.planRepo.Update(ctx, plan)
}

// DeletePlan 删除计划（结果由外键级联删除）。
func (s *FingerprintService) DeletePlan(ctx context.Context, id int64) error {
	return s.planRepo.Delete(ctx, id)
}

// ListResults 列出某计划的最近结果。
func (s *FingerprintService) ListResults(ctx context.Context, planID int64, limit int) ([]*FingerprintResult, error) {
	if limit <= 0 {
		limit = FingerprintDefaultMaxResults
	}
	return s.resultRepo.ListByPlanID(ctx, planID, limit)
}

// MonthlyTokenUsage 返回某账号本月的指纹检测估算消耗，供面板展示。
// 方案 A 直连上游不走网关，这笔花费不会进用量看板，只能从结果表汇总。
func (s *FingerprintService) MonthlyTokenUsage(ctx context.Context, accountID int64) (int64, error) {
	now := time.Now().UTC()
	since := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	return s.resultRepo.SumEstimatedTokensSince(ctx, accountID, since)
}

// SaveResult 落库一条结果、裁剪历史，并维护连续不符计数。
//
// 计数规则（规格 6.3 / 6.5）：
//   - mismatch  → 累加；达到阈值后才允许执行动账号的动作
//   - match     → 清零
//   - 其他两态  → 不动。无法判定和检测失败都不是"这次没问题"，也不是"又一次造假"，
//     让它们参与计数会把一串 429 攒成一次停号。
//
// 被配额拦下的结果同样不参与计数——那次检测根本没发生。
func (s *FingerprintService) SaveResult(ctx context.Context, plan *FingerprintPlan, result *FingerprintResult) error {
	if plan == nil || result == nil {
		return fmt.Errorf("plan and result are required")
	}

	result.PlanID = plan.ID
	if result.StartedAt.IsZero() {
		result.StartedAt = time.Now()
	}
	if result.FinishedAt.IsZero() {
		result.FinishedAt = time.Now()
	}

	if _, err := s.resultRepo.Create(ctx, result); err != nil {
		return err
	}

	keep := plan.MaxResults
	if keep <= 0 {
		keep = FingerprintDefaultMaxResults
	}
	if err := s.resultRepo.PruneOldResults(ctx, plan.ID, keep); err != nil {
		return err
	}

	if result.SkipReason != "" {
		return nil
	}

	switch result.Verdict {
	case FingerprintVerdictMismatch:
		next := plan.ConsecutiveMismatches + 1
		if err := s.planRepo.UpdateConsecutiveMismatches(ctx, plan.ID, next); err != nil {
			return err
		}
		plan.ConsecutiveMismatches = next
	case FingerprintVerdictMatch:
		if err := s.planRepo.UpdateConsecutiveMismatches(ctx, plan.ID, 0); err != nil {
			return err
		}
		plan.ConsecutiveMismatches = 0
	}
	return nil
}

// KnownModelSet 返回参考库模型 ID 的集合，供判定时校验预期模型。
// 清单拉取失败时返回 nil —— 调用方据此跳过参考库校验，而不是把所有账号判成无法判定。
func (s *FingerprintService) KnownModelSet(ctx context.Context) map[string]bool {
	if s == nil || s.catalog == nil {
		return nil
	}
	models, err := s.catalog.ListReferenceModels(ctx)
	if err != nil || len(models) == 0 {
		return nil
	}
	out := make(map[string]bool, len(models))
	for _, m := range models {
		out[m.ID] = true
	}
	return out
}

// validatePlan 做建/改计划时的全部校验。
func (s *FingerprintService) validatePlan(ctx context.Context, plan *FingerprintPlan) error {
	if plan == nil {
		return fmt.Errorf("计划不能为空")
	}
	if plan.AccountID <= 0 {
		return fmt.Errorf("必须指定账号")
	}
	if strings.TrimSpace(plan.TestModel) == "" {
		return fmt.Errorf("测试模型不能为空")
	}
	if !IsValidFingerprintAPI(plan.API) {
		return fmt.Errorf("api 必须是 responses、chat_completions 或 messages")
	}
	if plan.ChallengeCount < 1 || plan.ChallengeCount > 3 {
		return fmt.Errorf("挑战数必须在 1 到 3 之间")
	}
	if plan.MaxResults <= 0 {
		plan.MaxResults = FingerprintDefaultMaxResults
	}
	if plan.MismatchActionThreshold <= 0 {
		plan.MismatchActionThreshold = FingerprintDefaultMismatchThreshold
	}

	// 两个数组列是 NOT NULL；nil 切片经 pq.Array 会写成 SQL NULL，DEFAULT '{}' 不生效，
	// 直接违反约束。统一归一成空切片并去重。
	normalized, err := normalizeFingerprintGroupIDs(plan.FailureRemoveGroupIDs)
	if err != nil {
		return err
	}
	plan.FailureRemoveGroupIDs = normalized
	normalized, err = normalizeFingerprintGroupIDs(plan.SuccessAddGroupIDs)
	if err != nil {
		return err
	}
	plan.SuccessAddGroupIDs = normalized

	return s.validateExpectedModel(ctx, plan.ExpectedModel)
}

// normalizeFingerprintGroupIDs 把分组 id 列表归一成非 nil、去重、全为正数的切片。
func normalizeFingerprintGroupIDs(ids []int64) ([]int64, error) {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return nil, fmt.Errorf("分组 id 必须为正数")
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

// validateExpectedModel 要求预期模型必须在参考库里。
//
// 参考库是封闭集合，库外的模型也会被硬配一个"最像"的候选，于是每一轮都判不符
// ——永久误报。从建计划就拦住，比事后向管理员解释误报便宜得多。
// 清单暂时拉不到时放行，避免数据源抖动把建计划整个堵死。
func (s *FingerprintService) validateExpectedModel(ctx context.Context, expected string) error {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return fmt.Errorf("必须选择预期模型")
	}
	known := s.KnownModelSet(ctx)
	if known == nil {
		return nil
	}
	if !known[expected] {
		return fmt.Errorf("参考库暂不支持模型 %s，请从清单中选择", expected)
	}
	return nil
}
