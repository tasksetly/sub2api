package service

import (
	"context"
	"errors"
	"testing"
	"time"
)

// --- 测试替身 ---

type fingerprintPlanRepoStub struct {
	plans            map[int64]*FingerprintPlan
	created          *FingerprintPlan
	consecutiveCalls []int
	nextID           int64
}

func newFingerprintPlanRepoStub() *fingerprintPlanRepoStub {
	return &fingerprintPlanRepoStub{plans: map[int64]*FingerprintPlan{}, nextID: 1}
}

func (r *fingerprintPlanRepoStub) Create(ctx context.Context, plan *FingerprintPlan) (*FingerprintPlan, error) {
	copied := *plan
	copied.ID = r.nextID
	r.nextID++
	r.plans[copied.ID] = &copied
	r.created = &copied
	return &copied, nil
}

func (r *fingerprintPlanRepoStub) GetByID(ctx context.Context, id int64) (*FingerprintPlan, error) {
	if p, ok := r.plans[id]; ok {
		return p, nil
	}
	return nil, errors.New("not found")
}

func (r *fingerprintPlanRepoStub) ListByAccountID(ctx context.Context, accountID int64) ([]*FingerprintPlan, error) {
	out := make([]*FingerprintPlan, 0)
	for _, p := range r.plans {
		if p.AccountID == accountID {
			out = append(out, p)
		}
	}
	return out, nil
}

func (r *fingerprintPlanRepoStub) ListDue(ctx context.Context, now time.Time) ([]*FingerprintPlan, error) {
	return nil, nil
}

func (r *fingerprintPlanRepoStub) Update(ctx context.Context, plan *FingerprintPlan) (*FingerprintPlan, error) {
	copied := *plan
	r.plans[copied.ID] = &copied
	return &copied, nil
}

func (r *fingerprintPlanRepoStub) Delete(ctx context.Context, id int64) error {
	delete(r.plans, id)
	return nil
}

func (r *fingerprintPlanRepoStub) UpdateAfterRun(ctx context.Context, id int64, lastRunAt, nextRunAt time.Time) error {
	return nil
}

func (r *fingerprintPlanRepoStub) UpdateConsecutiveMismatches(ctx context.Context, planID int64, n int) error {
	r.consecutiveCalls = append(r.consecutiveCalls, n)
	if p, ok := r.plans[planID]; ok {
		p.ConsecutiveMismatches = n
	}
	return nil
}

type fingerprintResultRepoStub struct {
	created   []*FingerprintResult
	pruneKeep []int
}

func (r *fingerprintResultRepoStub) Create(ctx context.Context, result *FingerprintResult) (*FingerprintResult, error) {
	copied := *result
	copied.ID = int64(len(r.created) + 1)
	r.created = append(r.created, &copied)
	return &copied, nil
}

func (r *fingerprintResultRepoStub) ListByPlanID(ctx context.Context, planID int64, limit int) ([]*FingerprintResult, error) {
	return r.created, nil
}

func (r *fingerprintResultRepoStub) PruneOldResults(ctx context.Context, planID int64, keepCount int) error {
	r.pruneKeep = append(r.pruneKeep, keepCount)
	return nil
}

func (r *fingerprintResultRepoStub) SumEstimatedTokensSince(ctx context.Context, accountID int64, since time.Time) (int64, error) {
	return 0, nil
}

func (r *fingerprintResultRepoStub) CountSince(ctx context.Context, accountID int64, since time.Time) (int64, error) {
	return 0, nil
}

func (r *fingerprintResultRepoStub) CountAllSince(ctx context.Context, since time.Time) (int64, error) {
	return 0, nil
}

type fingerprintCatalogStub struct {
	models []FingerprintRefModel
	err    error
}

func (c *fingerprintCatalogStub) ListReferenceModels(ctx context.Context) ([]FingerprintRefModel, error) {
	return c.models, c.err
}

func newFingerprintServiceForTest() (*FingerprintService, *fingerprintPlanRepoStub, *fingerprintResultRepoStub) {
	planRepo := newFingerprintPlanRepoStub()
	resultRepo := &fingerprintResultRepoStub{}
	catalog := &fingerprintCatalogStub{models: []FingerprintRefModel{
		{ID: "claude-opus-5", DisplayName: "claude-opus-5", Family: "claude", FamilyName: "Claude"},
		{ID: "deepseek-v4-flash", DisplayName: "deepseek-v4-flash", Family: "deepseek", FamilyName: "DeepSeek"},
	}}
	return NewFingerprintService(planRepo, resultRepo, catalog), planRepo, resultRepo
}

func validFingerprintPlan() *FingerprintPlan {
	p := DefaultFingerprintPlan()
	p.AccountID = 42
	p.TestModel = "claude-opus-5-latest"
	p.ExpectedModel = "claude-opus-5"
	return &p
}

// --- 测试 ---

func TestSaveResultPrunesToMaxResults(t *testing.T) {
	svc, planRepo, resultRepo := newFingerprintServiceForTest()
	plan, _ := planRepo.Create(context.Background(), validFingerprintPlan())

	err := svc.SaveResult(context.Background(), plan, &FingerprintResult{Verdict: FingerprintVerdictMatch})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resultRepo.created) != 1 {
		t.Fatalf("created = %d, want 1", len(resultRepo.created))
	}
	if resultRepo.created[0].PlanID != plan.ID {
		t.Fatalf("PlanID = %d, want %d", resultRepo.created[0].PlanID, plan.ID)
	}
	if len(resultRepo.pruneKeep) != 1 || resultRepo.pruneKeep[0] != plan.MaxResults {
		t.Fatalf("pruneKeep = %v, want [%d]", resultRepo.pruneKeep, plan.MaxResults)
	}
}

func TestSaveResultMaintainsConsecutiveMismatchCounter(t *testing.T) {
	for _, tt := range []struct {
		name      string
		start     int
		verdict   string
		wantCalls []int
	}{
		{"不符累加", 1, FingerprintVerdictMismatch, []int{2}},
		{"相符清零", 2, FingerprintVerdictMatch, []int{0}},
		{"无法判定不动", 1, FingerprintVerdictInconclusive, nil},
		{"检测失败不动", 1, FingerprintVerdictError, nil},
	} {
		t.Run(tt.name, func(t *testing.T) {
			svc, planRepo, _ := newFingerprintServiceForTest()
			p := validFingerprintPlan()
			p.ConsecutiveMismatches = tt.start
			plan, _ := planRepo.Create(context.Background(), p)

			if err := svc.SaveResult(context.Background(), plan, &FingerprintResult{Verdict: tt.verdict}); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(planRepo.consecutiveCalls) != len(tt.wantCalls) {
				t.Fatalf("calls = %v, want %v", planRepo.consecutiveCalls, tt.wantCalls)
			}
			for i := range tt.wantCalls {
				if planRepo.consecutiveCalls[i] != tt.wantCalls[i] {
					t.Fatalf("calls = %v, want %v", planRepo.consecutiveCalls, tt.wantCalls)
				}
			}
			if len(tt.wantCalls) > 0 && plan.ConsecutiveMismatches != tt.wantCalls[0] {
				t.Fatalf("计数未回写到内存中的计划：%d, want %d", plan.ConsecutiveMismatches, tt.wantCalls[0])
			}
		})
	}
}

func TestSaveResultSkipsCounterForQuotaSkips(t *testing.T) {
	svc, planRepo, _ := newFingerprintServiceForTest()
	p := validFingerprintPlan()
	p.ConsecutiveMismatches = 1
	plan, _ := planRepo.Create(context.Background(), p)

	err := svc.SaveResult(context.Background(), plan, &FingerprintResult{
		Verdict:    FingerprintVerdictError,
		SkipReason: FingerprintSkipDailyGlobal,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(planRepo.consecutiveCalls) != 0 {
		t.Fatalf("配额跳过不该影响连续计数，calls = %v", planRepo.consecutiveCalls)
	}
}

func TestCreatePlanRejectsInvalidCron(t *testing.T) {
	svc, _, _ := newFingerprintServiceForTest()
	p := validFingerprintPlan()
	p.CronExpression = "not a cron"
	if _, err := svc.CreatePlan(context.Background(), p); err == nil {
		t.Fatal("非法 cron 应报错")
	}
}

func TestCreatePlanRejectsExpectedModelOutsideBank(t *testing.T) {
	svc, _, _ := newFingerprintServiceForTest()
	p := validFingerprintPlan()
	p.ExpectedModel = "some-finetuned-model"
	_, err := svc.CreatePlan(context.Background(), p)
	if err == nil {
		t.Fatal("参考库外的预期模型应被拒绝，否则每轮都判不符")
	}
}

func TestCreatePlanRejectsBadChallengeCount(t *testing.T) {
	svc, _, _ := newFingerprintServiceForTest()
	for _, n := range []int{0, -1, 4, 99} {
		p := validFingerprintPlan()
		p.ChallengeCount = n
		if _, err := svc.CreatePlan(context.Background(), p); err == nil {
			t.Fatalf("challenge count %d 应被拒绝", n)
		}
	}
}

func TestCreatePlanRejectsBadAPI(t *testing.T) {
	svc, _, _ := newFingerprintServiceForTest()
	p := validFingerprintPlan()
	p.API = "anthropic"
	if _, err := svc.CreatePlan(context.Background(), p); err == nil {
		t.Fatal("非 SDK 支持的 api 取值应被拒绝")
	}
}

func TestCreatePlanComputesNextRun(t *testing.T) {
	svc, _, _ := newFingerprintServiceForTest()
	created, err := svc.CreatePlan(context.Background(), validFingerprintPlan())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if created.NextRunAt == nil || !created.NextRunAt.After(time.Now().Add(-time.Minute)) {
		t.Fatalf("NextRunAt = %v, want 一个未来时间", created.NextRunAt)
	}
}

func TestKnownModelSetFallsBackToNilWhenCatalogFails(t *testing.T) {
	planRepo := newFingerprintPlanRepoStub()
	svc := NewFingerprintService(planRepo, &fingerprintResultRepoStub{}, &fingerprintCatalogStub{err: errors.New("offline")})

	got := svc.KnownModelSet(context.Background())
	if got != nil {
		t.Fatalf("清单不可用时应返回 nil（跳过参考库校验），got %v", got)
	}
}

func TestKnownModelSetBuildsLookup(t *testing.T) {
	svc, _, _ := newFingerprintServiceForTest()
	got := svc.KnownModelSet(context.Background())
	if !got["claude-opus-5"] || !got["deepseek-v4-flash"] {
		t.Fatalf("set = %v", got)
	}
	if got["nope"] {
		t.Fatal("不应包含库外模型")
	}
}
