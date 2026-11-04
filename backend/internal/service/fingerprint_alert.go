package service

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
)

// FingerprintAlertMetricType 标识指纹不符的内置告警规则。
//
// 既有的告警事件都来自指标阈值 evaluator，而指纹不符是事件驱动的、没有
// metric/threshold。仍然给它建一条规则，是为了复用规则上挂着的静默、冷却、
// 严重级别和邮件开关——走无规则的裸事件最省事，但"能静默"就没了，
// 而那正是承诺给管理员的能力（同一个账号不会反复刷屏）。
const FingerprintAlertMetricType = "fingerprint_mismatch"

const fingerprintAlertRuleName = "指纹校对：实判模型与预期不符"

// FingerprintOpsGateway 是告警需要的 ops 能力子集。*OpsService 实现了它。
type FingerprintOpsGateway interface {
	ListAlertRules(ctx context.Context) ([]*OpsAlertRule, error)
	CreateAlertRule(ctx context.Context, rule *OpsAlertRule) (*OpsAlertRule, error)
	CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) (*OpsAlertEvent, error)
	IsAlertSilenced(ctx context.Context, ruleID int64, platform string, groupID *int64, region *string, now time.Time) (bool, error)
}

// FingerprintAlerter 把指纹不符送进既有的 ops 告警体系。
type FingerprintAlerter struct {
	ops        FingerprintOpsGateway
	settingSvc *SettingService

	mu     sync.Mutex
	ruleID int64
}

// NewFingerprintAlerter 创建告警器。
func NewFingerprintAlerter(ops FingerprintOpsGateway, settingSvc *SettingService) *FingerprintAlerter {
	return &FingerprintAlerter{ops: ops, settingSvc: settingSvc}
}

// NotifyMismatch 在判定不符时发一条告警事件。
//
// 单次不符即发——告警和账号列表标记都不等连续两次，只有动账号的动作才等。
//
// 运维监控总开关关闭时返回 nil 而不是错误：第 2 层告警跟随该开关消失是预期的，
// 但第 1 层（账号列表上的指纹标记）不依赖它，管理员打开列表仍看得见。
// 让这里返回错误只会把整次检测执行标记成失败，反而掩盖了真正的判定结果。
func (a *FingerprintAlerter) NotifyMismatch(ctx context.Context, plan *FingerprintPlan, account *Account, result *FingerprintResult) error {
	if a == nil || a.ops == nil || plan == nil || account == nil || result == nil {
		return nil
	}
	if result.Verdict != FingerprintVerdictMismatch {
		return nil
	}

	ruleID, err := a.ensureRule(ctx)
	if err != nil {
		if errors.Is(err, ErrOpsDisabled) {
			return nil
		}
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintAlerter] ensure rule failed: %v", err)
		return nil
	}

	silenced, err := a.ops.IsAlertSilenced(ctx, ruleID, account.Platform, nil, nil, time.Now())
	if err != nil {
		if errors.Is(err, ErrOpsDisabled) {
			return nil
		}
		logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintAlerter] silence check failed: %v", err)
	}
	if silenced {
		return nil
	}

	event := &OpsAlertEvent{
		RuleID:      ruleID,
		Severity:    "warning",
		Status:      OpsAlertStatusFiring,
		Title:       fmt.Sprintf("账号「%s」指纹不符：预期 %s，实判 %s", account.Name, plan.ExpectedModel, result.PredictedModel),
		Description: a.describe(plan, account, result),
		Dimensions: map[string]any{
			"account_id":       account.ID,
			"account_name":     account.Name,
			"platform":         account.Platform,
			"plan_id":          plan.ID,
			"test_model":       plan.TestModel,
			"expected_model":   plan.ExpectedModel,
			"predicted_model":  result.PredictedModel,
			"predicted_family": result.PredictedFamily,
			"decision":         result.Decision,
			"probability":      result.Probability,
		},
		FiredAt: time.Now(),
	}

	if _, err := a.ops.CreateAlertEvent(ctx, event); err != nil {
		if !errors.Is(err, ErrOpsDisabled) {
			logger.LegacyPrintf(fingerprintRunnerLogScope, "[FingerprintAlerter] create event failed: %v", err)
		}
	}
	return nil
}

// describe 写给管理员看的说明。只用判定结果，绝不带凭证。
func (a *FingerprintAlerter) describe(plan *FingerprintPlan, account *Account, result *FingerprintResult) string {
	probability := "不可用"
	if result.Probability != nil {
		probability = fmt.Sprintf("%.2f", *result.Probability)
	}
	return fmt.Sprintf(
		"账号 #%d「%s」用测试模型 %s 发起指纹校对，预期 %s，实判 %s（判定类型 %s，置信度 %s）。"+
			"排名为参考库内的封闭集合排序，请结合连续多次结果与其他证据判断。",
		account.ID, account.Name, plan.TestModel, plan.ExpectedModel, result.PredictedModel,
		result.Decision, probability,
	)
}

// ensureRule 取内置规则的 ID，没有就建一条。结果缓存在内存里，避免每次告警都查一遍。
func (a *FingerprintAlerter) ensureRule(ctx context.Context) (int64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ruleID > 0 {
		return a.ruleID, nil
	}

	rules, err := a.ops.ListAlertRules(ctx)
	if err != nil {
		return 0, err
	}
	for _, rule := range rules {
		if rule != nil && rule.MetricType == FingerprintAlertMetricType {
			a.ruleID = rule.ID
			return a.ruleID, nil
		}
	}

	created, err := a.ops.CreateAlertRule(ctx, &OpsAlertRule{
		Name:        fingerprintAlertRuleName,
		Description: "指纹校对判定实判模型与管理员指定的预期模型不一致时触发。事件驱动，不走指标阈值。",
		Enabled:     true,
		Severity:    "warning",
		MetricType:  FingerprintAlertMetricType,
		// 事件驱动的规则没有阈值可言，填中性占位值以兼容既有规则模型。
		Operator:        "gt",
		Threshold:       0,
		WindowMinutes:   0,
		CooldownMinutes: 60,
		NotifyEmail:     a.notifyEmail(ctx),
	})
	if err != nil {
		return 0, err
	}
	a.ruleID = created.ID
	return a.ruleID, nil
}

// notifyEmail 读取邮件开关，默认关闭——抽查频繁，开了容易变垃圾邮件。
func (a *FingerprintAlerter) notifyEmail(ctx context.Context) bool {
	if a.settingSvc == nil {
		return false
	}
	return a.settingSvc.GetFingerprintSettings(ctx).NotifyEmail
}
