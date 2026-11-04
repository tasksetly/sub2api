package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

type fingerprintOpsStub struct {
	rules      []*OpsAlertRule
	events     []*OpsAlertEvent
	silenced   bool
	disabled   bool
	createRule int
}

func (s *fingerprintOpsStub) ListAlertRules(ctx context.Context) ([]*OpsAlertRule, error) {
	if s.disabled {
		return nil, ErrOpsDisabled
	}
	return s.rules, nil
}

func (s *fingerprintOpsStub) CreateAlertRule(ctx context.Context, rule *OpsAlertRule) (*OpsAlertRule, error) {
	if s.disabled {
		return nil, ErrOpsDisabled
	}
	s.createRule++
	copied := *rule
	copied.ID = int64(len(s.rules) + 100)
	s.rules = append(s.rules, &copied)
	return &copied, nil
}

func (s *fingerprintOpsStub) CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) (*OpsAlertEvent, error) {
	if s.disabled {
		return nil, ErrOpsDisabled
	}
	s.events = append(s.events, event)
	return event, nil
}

func (s *fingerprintOpsStub) IsAlertSilenced(ctx context.Context, ruleID int64, platform string, groupID *int64, region *string, now time.Time) (bool, error) {
	if s.disabled {
		return false, ErrOpsDisabled
	}
	return s.silenced, nil
}

func newFingerprintAlerterForTest(ops *fingerprintOpsStub) *FingerprintAlerter {
	return NewFingerprintAlerter(ops, nil)
}

func fingerprintMismatchFixture() (*FingerprintPlan, *Account, *FingerprintResult) {
	plan := validFingerprintPlan()
	plan.ID = 9

	acc := fingerprintTestAccount(AccountTypeUpstream, nil)
	acc.ID = 42
	acc.Name = "便宜的中转商"

	prob := 0.91
	result := &FingerprintResult{
		Verdict:            FingerprintVerdictMismatch,
		Decision:           "scored",
		PredictedModel:     "deepseek-v4-flash",
		PredictedModelName: "deepseek-v4-flash",
		PredictedFamily:    "deepseek",
		Probability:        &prob,
	}
	return plan, acc, result
}

func TestNotifyMismatchFiresOnFirstMismatch(t *testing.T) {
	ops := &fingerprintOpsStub{}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()

	if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops.events) != 1 {
		t.Fatalf("events = %d, want 1（告警不等连续两次）", len(ops.events))
	}

	ev := ops.events[0]
	if ev.RuleID == 0 {
		t.Fatal("事件必须挂在内置规则上，否则静默与冷却都用不了")
	}
	for _, key := range []string{"account_id", "plan_id", "expected_model", "predicted_model"} {
		if _, ok := ev.Dimensions[key]; !ok {
			t.Fatalf("dimensions 缺少 %q: %+v", key, ev.Dimensions)
		}
	}
	if ev.Status != OpsAlertStatusFiring {
		t.Fatalf("Status = %q, want %q", ev.Status, OpsAlertStatusFiring)
	}
}

func TestNotifyMismatchCreatesBuiltinRuleOnce(t *testing.T) {
	ops := &fingerprintOpsStub{}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		if err := alerter.NotifyMismatch(ctx, plan, acc, result); err != nil {
			t.Fatalf("第 %d 次出错: %v", i+1, err)
		}
	}
	if ops.createRule != 1 {
		t.Fatalf("createRule = %d, want 1（内置规则只建一次）", ops.createRule)
	}
	if len(ops.rules) != 1 || ops.rules[0].MetricType != FingerprintAlertMetricType {
		t.Fatalf("rules = %+v", ops.rules)
	}
}

func TestNotifyMismatchReusesExistingRule(t *testing.T) {
	ops := &fingerprintOpsStub{rules: []*OpsAlertRule{
		{ID: 77, Name: "旧规则", MetricType: FingerprintAlertMetricType},
	}}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()

	if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ops.createRule != 0 {
		t.Fatal("已有同类规则时不应重复创建")
	}
	if ops.events[0].RuleID != 77 {
		t.Fatalf("RuleID = %d, want 77", ops.events[0].RuleID)
	}
}

func TestNotifyMismatchRespectsSilence(t *testing.T) {
	ops := &fingerprintOpsStub{silenced: true}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()

	if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ops.events) != 0 {
		t.Fatal("静默期内不应发事件")
	}
}

// 运维监控总开关关闭时，ops 事件消失是预期的，但不能让整次执行失败——
// 账号列表上的指纹标记不依赖该开关，仍然会出现。
func TestNotifyMismatchSurvivesMonitoringDisabled(t *testing.T) {
	ops := &fingerprintOpsStub{disabled: true}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()

	if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
		t.Fatalf("监控关闭不应让告警调用返回错误: %v", err)
	}
	if len(ops.events) != 0 {
		t.Fatal("监控关闭时不该发事件")
	}
}

func TestNotifyMismatchOnlyForMismatch(t *testing.T) {
	for _, verdict := range []string{
		FingerprintVerdictMatch,
		FingerprintVerdictInconclusive,
		FingerprintVerdictError,
	} {
		ops := &fingerprintOpsStub{}
		alerter := newFingerprintAlerterForTest(ops)
		plan, acc, result := fingerprintMismatchFixture()
		result.Verdict = verdict

		if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
			t.Fatalf("verdict %s: %v", verdict, err)
		}
		if len(ops.events) != 0 {
			t.Fatalf("verdict %s 不应发告警", verdict)
		}
	}
}

// Review Focus #4：ops 事件的 dimensions 会被管理员和日志广泛读取，绝不能带凭证。
func TestNotifyMismatchDimensionsNeverIncludeCredentials(t *testing.T) {
	ops := &fingerprintOpsStub{}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()

	if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ev := ops.events[0]
	for key, value := range ev.Dimensions {
		if key == "api_key" || key == "base_url" || key == "credentials" {
			t.Fatalf("dimensions 含敏感键 %q", key)
		}
		if s, ok := value.(string); ok && s == acc.GetCredential("api_key") {
			t.Fatalf("dimensions[%q] 泄露了 API Key", key)
		}
	}
	if ev.Description == "" {
		t.Fatal("描述不能为空，管理员要据此判断")
	}
	if containsFingerprintSecret(ev.Title+ev.Description, acc.GetCredential("api_key")) {
		t.Fatal("标题或描述泄露了 API Key")
	}
}

func TestNotifyMismatchToleratesEventFailure(t *testing.T) {
	ops := &fingerprintOpsStub{}
	alerter := newFingerprintAlerterForTest(ops)
	plan, acc, result := fingerprintMismatchFixture()
	alerter.ops = &fingerprintOpsFailingStub{fingerprintOpsStub: ops, failEvent: errors.New("db down")}

	if err := alerter.NotifyMismatch(context.Background(), plan, acc, result); err != nil {
		t.Fatalf("发事件失败不应让整次执行失败: %v", err)
	}
}

type fingerprintOpsFailingStub struct {
	*fingerprintOpsStub
	failEvent error
}

func (s *fingerprintOpsFailingStub) CreateAlertEvent(ctx context.Context, event *OpsAlertEvent) (*OpsAlertEvent, error) {
	return nil, s.failEvent
}

func containsFingerprintSecret(text, secret string) bool {
	return secret != "" && len(secret) >= 8 && len(text) > 0 && (text != redactFingerprintSecrets(text, secret))
}
