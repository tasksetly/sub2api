package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
)

// --- 测试替身 ---

type fingerprintAccountRepoStub struct {
	AccountRepository
	account *Account

	statusUpdates  []string
	tempUnsched    []time.Duration
	removedGroups  []int64
	addedGroups    []int64
	clearedError   bool
	setSchedulable []bool
}

func (r *fingerprintAccountRepoStub) GetByID(ctx context.Context, id int64) (*Account, error) {
	if r.account == nil {
		return nil, errors.New("not found")
	}
	return r.account, nil
}

func (r *fingerprintAccountRepoStub) Update(ctx context.Context, a *Account) error {
	r.statusUpdates = append(r.statusUpdates, a.Status)
	return nil
}

func (r *fingerprintAccountRepoStub) SetTempUnschedulable(ctx context.Context, id int64, until time.Time, reason string) error {
	r.tempUnsched = append(r.tempUnsched, time.Until(until))
	return nil
}

func (r *fingerprintAccountRepoStub) ClearError(ctx context.Context, id int64) error {
	r.clearedError = true
	return nil
}

func (r *fingerprintAccountRepoStub) SetSchedulable(ctx context.Context, id int64, schedulable bool) error {
	r.setSchedulable = append(r.setSchedulable, schedulable)
	return nil
}

func (r *fingerprintAccountRepoStub) AddToGroup(ctx context.Context, accountID, groupID int64, priority int) error {
	r.addedGroups = append(r.addedGroups, groupID)
	return nil
}

func (r *fingerprintAccountRepoStub) RemoveFromGroup(ctx context.Context, accountID, groupID int64) error {
	r.removedGroups = append(r.removedGroups, groupID)
	return nil
}

type fingerprintRunnerHarness struct {
	runner      *FingerprintRunnerService
	planRepo    *fingerprintPlanRepoStub
	resultRepo  *fingerprintResultRepoStub
	accountRepo *fingerprintAccountRepoStub
	plan        *FingerprintPlan
}

func newFingerprintRunnerHarness(t *testing.T, detect fingerprintDetectFunc) *fingerprintRunnerHarness {
	t.Helper()

	planRepo := newFingerprintPlanRepoStub()
	resultRepo := &fingerprintResultRepoStub{}
	catalog := &fingerprintCatalogStub{models: []FingerprintRefModel{
		{ID: "claude-opus-5", FamilyName: "Claude"},
		{ID: "deepseek-v4-flash", FamilyName: "DeepSeek"},
	}}
	svc := NewFingerprintService(planRepo, resultRepo, catalog)

	acc := fingerprintTestAccount(AccountTypeUpstream, nil)
	acc.ID = 42
	acc.Schedulable = true
	accountRepo := &fingerprintAccountRepoStub{account: acc}

	plan, _ := planRepo.Create(context.Background(), validFingerprintPlan())
	plan.FailureDisable = true
	plan.FailureTempUnschedulableMinutes = 30
	plan.FailureRemoveGroupIDs = []int64{11}
	plan.SuccessAddGroupIDs = []int64{22}

	quota := newFingerprintQuotaForTest(newFingerprintLockCacheStub(), &fingerprintCounterStub{},
		FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 100, DailyLimitAccount: 100})

	runner := NewFingerprintRunnerService(planRepo, svc, accountRepo, nil, quota, nil, nil, nil)
	runner.detectFn = detect

	return &fingerprintRunnerHarness{runner: runner, planRepo: planRepo, resultRepo: resultRepo, accountRepo: accountRepo, plan: plan}
}

func fingerprintDetectStub(decision, prediction string) fingerprintDetectFunc {
	return func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
		return fingerprintDetectResult(decision, prediction), nil
	}
}

func (h *fingerprintRunnerHarness) lastResult(t *testing.T) *FingerprintResult {
	t.Helper()
	if len(h.resultRepo.created) == 0 {
		t.Fatal("没有写入任何结果")
	}
	return h.resultRepo.created[len(h.resultRepo.created)-1]
}

func (h *fingerprintRunnerHarness) touchedAccount() bool {
	r := h.accountRepo
	return len(r.statusUpdates) > 0 || len(r.tempUnsched) > 0 || len(r.removedGroups) > 0
}

// --- 测试 ---

func TestRunnerAppliesAccountActionsOnlyAfterTwoMismatches(t *testing.T) {
	h := newFingerprintRunnerHarness(t, fingerprintDetectStub("scored", "deepseek-v4-flash"))
	ctx := context.Background()

	if _, err := h.runner.executePlan(ctx, h.plan, true); err != nil {
		t.Fatalf("第 1 次执行出错: %v", err)
	}
	if h.lastResult(t).Verdict != FingerprintVerdictMismatch {
		t.Fatalf("第 1 次应判 mismatch")
	}
	if h.touchedAccount() {
		t.Fatal("第 1 次不符就动账号——单次误判会停掉好账号")
	}

	if _, err := h.runner.executePlan(ctx, h.plan, true); err != nil {
		t.Fatalf("第 2 次执行出错: %v", err)
	}
	if !h.touchedAccount() {
		t.Fatal("连续第 2 次不符应执行已开启的动账号动作")
	}
	if len(h.accountRepo.statusUpdates) == 0 || h.accountRepo.statusUpdates[0] != StatusDisabled {
		t.Fatalf("statusUpdates = %v, want 含 %q", h.accountRepo.statusUpdates, StatusDisabled)
	}
	if len(h.accountRepo.removedGroups) != 1 || h.accountRepo.removedGroups[0] != 11 {
		t.Fatalf("removedGroups = %v, want [11]", h.accountRepo.removedGroups)
	}
	if len(h.accountRepo.tempUnsched) != 1 {
		t.Fatalf("tempUnsched = %v, want 一次", h.accountRepo.tempUnsched)
	}
}

func TestRunnerNeverTouchesAccountOnInconclusiveOrError(t *testing.T) {
	for _, tt := range []struct {
		name   string
		detect fingerprintDetectFunc
	}{
		{"unscorable", fingerprintDetectStub("unscorable", "")},
		{"not_model", fingerprintDetectStub("not_model", "")},
		{"429 限流", func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
			return nil, errors.New("429 Too Many Requests")
		}},
		{"上游返回非法响应", func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
			msg := "The server did not return valid JSON or SSE."
			return &lmfpd.DetectResult{Error: &msg}, nil
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			h := newFingerprintRunnerHarness(t, tt.detect)
			// 先把计数推到阈值边缘，证明即便计数已满也不会被这两态触发。
			h.plan.ConsecutiveMismatches = h.plan.MismatchActionThreshold

			if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
				t.Fatalf("执行出错: %v", err)
			}
			got := h.lastResult(t).Verdict
			if got == FingerprintVerdictMismatch || got == FingerprintVerdictMatch {
				t.Fatalf("Verdict = %q, want inconclusive 或 error", got)
			}
			if h.touchedAccount() {
				t.Fatalf("%s 不得动账号——一次限流就停号正是要避免的事", tt.name)
			}
		})
	}
}

func TestRunnerSkipsInactiveAccountsOnSchedule(t *testing.T) {
	called := false
	detect := func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
		called = true
		return fingerprintDetectResult("scored", "claude-opus-5"), nil
	}
	h := newFingerprintRunnerHarness(t, detect)
	h.accountRepo.account.Status = StatusDisabled

	if _, err := h.runner.executePlan(context.Background(), h.plan, false); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	if called {
		t.Fatal("已停用账号的定期抽查不应发出任何请求")
	}
	if h.lastResult(t).SkipReason != FingerprintSkipAccountInactive {
		t.Fatalf("SkipReason = %q, want %q（跳过要留痕）", h.lastResult(t).SkipReason, FingerprintSkipAccountInactive)
	}
}

func TestRunnerManualRunIgnoresInactiveAccount(t *testing.T) {
	called := false
	detect := func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
		called = true
		return fingerprintDetectResult("scored", "claude-opus-5"), nil
	}
	h := newFingerprintRunnerHarness(t, detect)
	h.accountRepo.account.Status = StatusDisabled

	if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	if !called {
		t.Fatal("管理员手动点了按钮就该跑，不受账号活跃状态限制")
	}
}

func TestRunnerSuccessActionsDoNotEnableAccount(t *testing.T) {
	h := newFingerprintRunnerHarness(t, fingerprintDetectStub("scored", "claude-opus-5"))

	if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	if h.lastResult(t).Verdict != FingerprintVerdictMatch {
		t.Fatalf("Verdict = %q, want match", h.lastResult(t).Verdict)
	}
	if len(h.accountRepo.addedGroups) != 1 || h.accountRepo.addedGroups[0] != 22 {
		t.Fatalf("addedGroups = %v, want [22]", h.accountRepo.addedGroups)
	}
	if len(h.accountRepo.statusUpdates) != 0 {
		t.Fatalf("相符不得改账号状态：一次相符把管理员手工停用的账号自动启用是意外副作用，got %v", h.accountRepo.statusUpdates)
	}
	if h.accountRepo.clearedError || len(h.accountRepo.setSchedulable) != 0 {
		t.Fatal("指纹计划不提供自动恢复动作")
	}
}

func TestRunnerRecordsSkipReasonWhenQuotaExhausted(t *testing.T) {
	called := false
	detect := func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
		called = true
		return nil, nil
	}
	h := newFingerprintRunnerHarness(t, detect)
	h.runner.quota = newFingerprintQuotaForTest(newFingerprintLockCacheStub(),
		&fingerprintCounterStub{global: 10},
		FingerprintSettings{MaxConcurrency: 3, DailyLimitGlobal: 10, DailyLimitAccount: 100})

	if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	if called {
		t.Fatal("配额用尽后不应再发请求")
	}
	res := h.lastResult(t)
	if res.SkipReason != FingerprintSkipDailyGlobal {
		t.Fatalf("SkipReason = %q, want %q", res.SkipReason, FingerprintSkipDailyGlobal)
	}
	if res.Verdict != FingerprintVerdictError {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, FingerprintVerdictError)
	}
}

func TestRunnerWritesResultForIneligibleAccount(t *testing.T) {
	h := newFingerprintRunnerHarness(t, fingerprintDetectStub("scored", "claude-opus-5"))
	h.accountRepo.account.Type = AccountTypeOAuth

	if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	res := h.lastResult(t)
	if res.Verdict != FingerprintVerdictError {
		t.Fatalf("Verdict = %q, want %q", res.Verdict, FingerprintVerdictError)
	}
	if !strings.Contains(res.ErrorMessage, FingerprintReasonUnsupportedType) {
		t.Fatalf("ErrorMessage = %q, 应说明不支持的原因", res.ErrorMessage)
	}
	if h.touchedAccount() {
		t.Fatal("账号类型不支持不得动账号")
	}
}

// Review Focus #4：凭证绝不能进落库的错误信息。
func TestRunnerErrorMessageNeverLeaksCredentials(t *testing.T) {
	const secret = "sk-secret-value-do-not-leak"
	detect := func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
		// 模拟一个把配置回显进错误的上游库行为。
		return nil, errors.New("request failed for key " + secret + " at " + cfg.BaseURL)
	}
	h := newFingerprintRunnerHarness(t, detect)

	if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	res := h.lastResult(t)
	if strings.Contains(res.ErrorMessage, secret) {
		t.Fatalf("落库的 ErrorMessage 泄露了 API Key: %q", res.ErrorMessage)
	}
	if res.ErrorMessage == "" {
		t.Fatal("脱敏不能变成清空——界面上要能看到失败原因")
	}
}

func TestRedactFingerprintSecrets(t *testing.T) {
	const secret = "sk-abcdef0123456789"
	got := redactFingerprintSecrets("boom: "+secret+" done", secret)
	if strings.Contains(got, secret) {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("got %q, want 含 [REDACTED]", got)
	}
	// 空密钥不应把整串文本替换掉。
	if out := redactFingerprintSecrets("nothing secret", ""); out != "nothing secret" {
		t.Fatalf("got %q", out)
	}
}

func TestRunnerRecordsLatency(t *testing.T) {
	h := newFingerprintRunnerHarness(t, func(ctx context.Context, cfg lmfpd.DetectConfig) (*lmfpd.DetectResult, error) {
		time.Sleep(2 * time.Millisecond)
		return fingerprintDetectResult("scored", "claude-opus-5"), nil
	})
	if _, err := h.runner.executePlan(context.Background(), h.plan, true); err != nil {
		t.Fatalf("执行出错: %v", err)
	}
	if h.lastResult(t).LatencyMs <= 0 {
		t.Fatal("应记录检测耗时")
	}
}
