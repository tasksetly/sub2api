package admin

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

// FingerprintHandler 管理账号指纹校对的计划与结果。
type FingerprintHandler struct {
	fpSvc       *service.FingerprintService
	detector    *service.FingerprintDetectorProvider
	accountRepo service.AccountRepository
	runner      *service.FingerprintRunnerService
}

// NewFingerprintHandler 创建 handler。
func NewFingerprintHandler(
	fpSvc *service.FingerprintService,
	detector *service.FingerprintDetectorProvider,
	accountRepo service.AccountRepository,
) *FingerprintHandler {
	return &FingerprintHandler{fpSvc: fpSvc, detector: detector, accountRepo: accountRepo}
}

// SetRunner 注入执行器（装配时调用，打破 handler 与 runner 的循环依赖）。
func (h *FingerprintHandler) SetRunner(runner *service.FingerprintRunnerService) { h.runner = runner }

type fingerprintPlanRequest struct {
	AccountID                       int64   `json:"account_id"`
	TestModel                       string  `json:"test_model"`
	ExpectedModel                   string  `json:"expected_model"`
	API                             string  `json:"api"`
	ChallengeCount                  int     `json:"challenge_count"`
	CronExpression                  string  `json:"cron_expression"`
	Enabled                         *bool   `json:"enabled"`
	MaxResults                      int     `json:"max_results"`
	MismatchActionThreshold         int     `json:"mismatch_action_threshold"`
	FailureDisable                  *bool   `json:"failure_disable"`
	FailureTempUnschedulableMinutes int     `json:"failure_temp_unschedulable_minutes"`
	FailureRemoveGroupIDs           []int64 `json:"failure_remove_group_ids"`
	SuccessAddGroupIDs              []int64 `json:"success_add_group_ids"`
}

// applyTo 把请求填进计划，未提供的字段用规格默认值。
func (r *fingerprintPlanRequest) applyTo(plan *service.FingerprintPlan) {
	plan.TestModel = r.TestModel
	plan.ExpectedModel = r.ExpectedModel

	plan.API = r.API
	if plan.API == "" {
		plan.API = string(service.FingerprintAPIChatCompletions)
	}
	plan.ChallengeCount = r.ChallengeCount
	if plan.ChallengeCount == 0 {
		plan.ChallengeCount = service.FingerprintDefaultChallengeCount
	}
	plan.CronExpression = r.CronExpression
	if plan.CronExpression == "" {
		plan.CronExpression = service.FingerprintDefaultCron
	}
	if r.Enabled != nil {
		plan.Enabled = *r.Enabled
	}
	if r.MaxResults > 0 {
		plan.MaxResults = r.MaxResults
	}
	if r.MismatchActionThreshold > 0 {
		plan.MismatchActionThreshold = r.MismatchActionThreshold
	}
	if r.FailureDisable != nil {
		plan.FailureDisable = *r.FailureDisable
	}
	plan.FailureTempUnschedulableMinutes = r.FailureTempUnschedulableMinutes
	plan.FailureRemoveGroupIDs = r.FailureRemoveGroupIDs
	plan.SuccessAddGroupIDs = r.SuccessAddGroupIDs
}

// fingerprintOverviewResponse 是面板头部要的一次性上下文。
type fingerprintOverviewResponse struct {
	Eligibility            service.FingerprintEligibility `json:"eligibility"`
	MonthlyEstimatedTokens int64                          `json:"monthly_estimated_tokens"`
	ReferenceBuiltAt       string                         `json:"reference_built_at"`
	ReferenceError         string                         `json:"reference_error,omitempty"`
}

// ListReferenceModels GET /admin/fingerprint/reference-models
//
// 预期模型只能从这个清单里选，所以清单拉不到时必须把原因如实返回，
// 否则前端下拉空着像是功能坏了。
func (h *FingerprintHandler) ListReferenceModels(c *gin.Context) {
	models, err := h.detector.ListReferenceModels(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, models)
}

// Overview GET /admin/accounts/:id/fingerprint-overview
func (h *FingerprintHandler) Overview(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	account, err := h.accountRepo.GetByID(c.Request.Context(), accountID)
	if err != nil || account == nil {
		response.NotFound(c, "account not found")
		return
	}

	out := fingerprintOverviewResponse{
		Eligibility: service.CheckFingerprintEligibility(account),
	}
	if usage, err := h.fpSvc.MonthlyTokenUsage(c.Request.Context(), accountID); err == nil {
		out.MonthlyEstimatedTokens = usage
	}
	if builtAt, err := h.detector.BuiltAt(c.Request.Context()); err == nil {
		out.ReferenceBuiltAt = builtAt
	} else {
		out.ReferenceError = err.Error()
	}
	c.JSON(http.StatusOK, out)
}

// ListByAccount GET /admin/accounts/:id/fingerprint-plans
func (h *FingerprintHandler) ListByAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}
	plans, err := h.fpSvc.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

// Create POST /admin/fingerprint-plans
func (h *FingerprintHandler) Create(c *gin.Context) {
	var req fingerprintPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if req.AccountID <= 0 {
		response.BadRequest(c, "account_id is required")
		return
	}

	account, err := h.accountRepo.GetByID(c.Request.Context(), req.AccountID)
	if err != nil || account == nil {
		response.NotFound(c, "account not found")
		return
	}
	// 建计划时就拦住够不着的账号，而不是等到抽查时才失败。
	if eligibility := service.CheckFingerprintEligibility(account); !eligibility.Supported {
		response.BadRequest(c, "账号不支持指纹校对: "+eligibility.Reason)
		return
	}

	plan := service.DefaultFingerprintPlan()
	plan.AccountID = req.AccountID
	req.applyTo(&plan)

	// 这里也校验一遍检测参数：base_url 带 query/凭证时 SDK 会在发请求前报错，
	// 提前拦住才能给管理员可读的提示。
	if _, err := service.BuildDetectConfig(account, plan); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	created, err := h.fpSvc.CreatePlan(c.Request.Context(), &plan)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

// Update PUT /admin/fingerprint-plans/:id
func (h *FingerprintHandler) Update(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	existing, err := h.fpSvc.GetPlan(c.Request.Context(), planID)
	if err != nil || existing == nil {
		response.NotFound(c, "plan not found")
		return
	}

	var req fingerprintPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	req.applyTo(existing)

	updated, err := h.fpSvc.UpdatePlan(c.Request.Context(), existing)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

// Delete DELETE /admin/fingerprint-plans/:id
func (h *FingerprintHandler) Delete(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if err := h.fpSvc.DeletePlan(c.Request.Context(), planID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListResults GET /admin/fingerprint-plans/:id/results
func (h *FingerprintHandler) ListResults(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	limit, _ := strconv.Atoi(c.Query("limit"))

	results, err := h.fpSvc.ListResults(c.Request.Context(), planID, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}

// RunNow POST /admin/fingerprint-plans/:id/run
func (h *FingerprintHandler) RunNow(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if h.runner == nil {
		response.InternalError(c, "fingerprint runner unavailable")
		return
	}
	if err := h.runner.StartPlanNow(c.Request.Context(), planID); err != nil {
		c.JSON(http.StatusConflict, gin.H{"message": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"status": "running"})
}
