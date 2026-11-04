package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

// --- 测试替身 ---

type fpPlanRepo struct {
	service.FingerprintPlanRepository
	plan *service.FingerprintPlan
}

func (r *fpPlanRepo) Create(_ context.Context, plan *service.FingerprintPlan) (*service.FingerprintPlan, error) {
	plan.ID = 1
	r.plan = plan
	return plan, nil
}

func (r *fpPlanRepo) GetByID(context.Context, int64) (*service.FingerprintPlan, error) {
	return r.plan, nil
}

type fpResultRepo struct {
	service.FingerprintResultRepository
	results []*service.FingerprintResult
}

func (r *fpResultRepo) ListByPlanID(context.Context, int64, int) ([]*service.FingerprintResult, error) {
	return r.results, nil
}

func (r *fpResultRepo) SumEstimatedTokensSince(context.Context, int64, time.Time) (int64, error) {
	return 1234, nil
}

type fpCatalog struct{ models []service.FingerprintRefModel }

func (c *fpCatalog) ListReferenceModels(context.Context) ([]service.FingerprintRefModel, error) {
	return c.models, nil
}

type fpAccountRepo struct {
	service.AccountRepository
	account *service.Account
}

func (r *fpAccountRepo) GetByID(context.Context, int64) (*service.Account, error) {
	return r.account, nil
}

const fpTestAPIKey = "sk-handler-secret-key"

func fpAccount(accountType string) *service.Account {
	return &service.Account{
		ID:       42,
		Name:     "中转商 A",
		Type:     accountType,
		Platform: service.PlatformOpenAI,
		Status:   service.StatusActive,
		Credentials: map[string]any{
			"base_url": "https://api.example.com/v1",
			"api_key":  fpTestAPIKey,
		},
	}
}

func newFPHandler(account *service.Account, results []*service.FingerprintResult) *FingerprintHandler {
	planRepo := &fpPlanRepo{}
	resultRepo := &fpResultRepo{results: results}
	catalog := &fpCatalog{models: []service.FingerprintRefModel{
		{ID: "claude-opus-5", DisplayName: "claude-opus-5", Family: "claude", FamilyName: "Claude"},
	}}
	svc := service.NewFingerprintService(planRepo, resultRepo, catalog)
	return NewFingerprintHandler(svc, nil, &fpAccountRepo{account: account})
}

func postFPPlan(t *testing.T, h *FingerprintHandler, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := gin.New()
	router.POST("/fingerprint-plans", h.Create)
	req := httptest.NewRequest(http.MethodPost, "/fingerprint-plans", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

// --- 测试 ---

func TestCreateFingerprintPlanRejectsUnknownExpectedModel(t *testing.T) {
	h := newFPHandler(fpAccount(service.AccountTypeUpstream), nil)
	rec := postFPPlan(t, h, `{"account_id":42,"test_model":"opus-max","expected_model":"some-finetune"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400 (body=%s)", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "参考库") {
		t.Fatalf("body = %s, 应说明模型不在参考库", rec.Body.String())
	}
}

func TestCreateFingerprintPlanRejectsIneligibleAccount(t *testing.T) {
	for _, accountType := range []string{
		service.AccountTypeOAuth,
		service.AccountTypeSetupToken,
		service.AccountTypeBedrock,
		service.AccountTypeServiceAccount,
	} {
		h := newFPHandler(fpAccount(accountType), nil)
		rec := postFPPlan(t, h, `{"account_id":42,"test_model":"opus-max","expected_model":"claude-opus-5"}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("%s: code = %d, want 400", accountType, rec.Code)
		}
		if !strings.Contains(rec.Body.String(), service.FingerprintReasonUnsupportedType) {
			t.Fatalf("%s: body = %s, 应带上原因码供界面展示", accountType, rec.Body.String())
		}
	}
}

// Review Focus #2：base_url 带 query / 内嵌凭证时，必须在建计划时就报错。
func TestCreateFingerprintPlanRejectsUnusableBaseURL(t *testing.T) {
	for _, raw := range []string{
		"https://api.example.com/v1?key=abc",
		"https://user:pw@api.example.com/v1",
		"ftp://api.example.com/v1",
	} {
		account := fpAccount(service.AccountTypeUpstream)
		account.Credentials["base_url"] = raw
		h := newFPHandler(account, nil)
		rec := postFPPlan(t, h, `{"account_id":42,"test_model":"opus-max","expected_model":"claude-opus-5"}`)

		if rec.Code != http.StatusBadRequest {
			t.Fatalf("base_url %q: code = %d, want 400", raw, rec.Code)
		}
		if strings.Contains(rec.Body.String(), fpTestAPIKey) {
			t.Fatalf("base_url %q: 响应泄露了 API Key", raw)
		}
	}
}

func TestCreateFingerprintPlanAppliesSpecDefaults(t *testing.T) {
	h := newFPHandler(fpAccount(service.AccountTypeUpstream), nil)
	rec := postFPPlan(t, h, `{"account_id":42,"test_model":"opus-max","expected_model":"claude-opus-5"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var plan service.FingerprintPlan
	if err := json.Unmarshal(rec.Body.Bytes(), &plan); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if plan.CronExpression != service.FingerprintDefaultCron {
		t.Fatalf("cron = %q, want %q", plan.CronExpression, service.FingerprintDefaultCron)
	}
	if plan.ChallengeCount != service.FingerprintDefaultChallengeCount {
		t.Fatalf("challenge count = %d", plan.ChallengeCount)
	}
	if plan.MismatchActionThreshold != service.FingerprintDefaultMismatchThreshold {
		t.Fatalf("threshold = %d, want %d", plan.MismatchActionThreshold, service.FingerprintDefaultMismatchThreshold)
	}
	if plan.FailureDisable {
		t.Fatal("失败动作必须默认关闭")
	}
}

// Review Focus #4：结果接口会被前端频繁拉取，绝不能带出凭证。
func TestListResultsNeverReturnsCredentials(t *testing.T) {
	prob := 0.9
	h := newFPHandler(fpAccount(service.AccountTypeUpstream), []*service.FingerprintResult{{
		ID: 1, PlanID: 1,
		Verdict:        service.FingerprintVerdictMismatch,
		Decision:       "scored",
		PredictedModel: "deepseek-v4-flash",
		Probability:    &prob,
		AnalysisJSON:   `{"decision":"scored","prediction":"deepseek-v4-flash"}`,
	}})

	router := gin.New()
	router.GET("/fingerprint-plans/:id/results", h.ListResults)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/fingerprint-plans/1/results", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if strings.Contains(body, fpTestAPIKey) || strings.Contains(body, "api_key") {
		t.Fatalf("结果响应含凭证: %s", body)
	}
	if !strings.Contains(body, "deepseek-v4-flash") {
		t.Fatalf("结果响应缺少判定内容: %s", body)
	}
}

func TestRunNowWithoutRunnerFailsLoudly(t *testing.T) {
	h := newFPHandler(fpAccount(service.AccountTypeUpstream), nil)
	router := gin.New()
	router.POST("/fingerprint-plans/:id/run", h.RunNow)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/fingerprint-plans/1/run", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("code = %d, want 500（执行器不可用要说出来，不能装作成功）", rec.Code)
	}
}

func TestRunNowRejectsBadPlanID(t *testing.T) {
	h := newFPHandler(fpAccount(service.AccountTypeUpstream), nil)
	router := gin.New()
	router.POST("/fingerprint-plans/:id/run", h.RunNow)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/fingerprint-plans/abc/run", nil))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code = %d, want 400", rec.Code)
	}
}

func TestOverviewReportsEligibilityAndProxyBypass(t *testing.T) {
	proxyID := int64(3)
	account := fpAccount(service.AccountTypeUpstream)
	account.ProxyID = &proxyID
	h := newFPHandler(account, nil)

	router := gin.New()
	router.GET("/accounts/:id/fingerprint-overview", h.Overview)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/accounts/42/fingerprint-overview", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("code = %d, want 200 (body=%s)", rec.Code, rec.Body.String())
	}
	var out struct {
		Eligibility struct {
			Supported     bool `json:"supported"`
			ProxyBypassed bool `json:"proxy_bypassed"`
		} `json:"eligibility"`
		MonthlyEstimatedTokens int64 `json:"monthly_estimated_tokens"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Eligibility.Supported {
		t.Fatal("upstream 账号应判为支持")
	}
	if !out.Eligibility.ProxyBypassed {
		t.Fatal("配了代理必须回报 proxy_bypassed，界面据此提示结论仅供参考")
	}
	if out.MonthlyEstimatedTokens != 1234 {
		t.Fatalf("monthly tokens = %d, want 1234", out.MonthlyEstimatedTokens)
	}
	if strings.Contains(rec.Body.String(), fpTestAPIKey) {
		t.Fatal("概览响应泄露了 API Key")
	}
}
