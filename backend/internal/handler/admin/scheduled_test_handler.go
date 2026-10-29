package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

// ScheduledTestHandler handles admin scheduled-test-plan management.
type ScheduledTestHandler struct {
	scheduledTestSvc *service.ScheduledTestService
	runner           *service.ScheduledTestRunnerService
}

// NewScheduledTestHandler creates a new ScheduledTestHandler.
func NewScheduledTestHandler(scheduledTestSvc *service.ScheduledTestService) *ScheduledTestHandler {
	return &ScheduledTestHandler{scheduledTestSvc: scheduledTestSvc}
}

func (h *ScheduledTestHandler) SetRunner(runner *service.ScheduledTestRunnerService) { h.runner = runner }

type createScheduledTestPlanRequest struct {
	AccountID      int64  `json:"account_id" binding:"required"`
	ModelID        string `json:"model_id"`
	ReasoningEffort string `json:"reasoning_effort"`
	PromptText     string `json:"prompt_text"`
	ExpectedAnswer string `json:"expected_answer"`
	CronExpression string `json:"cron_expression" binding:"required"`
	Enabled        *bool  `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover                *bool  `json:"auto_recover"`
	TempUnschedulableEnabled   *bool  `json:"temp_unschedulable_enabled"`
	TempUnschedulableErrorCodes []int `json:"temp_unschedulable_error_codes"`
	TempUnschedulableMinutes   *int   `json:"temp_unschedulable_minutes"`
	FailureRemoveGroupID       *int64 `json:"failure_remove_group_id"`
	FailureRemoveGroupIDs      []int64 `json:"failure_remove_group_ids"`
	FailureDisable             *bool  `json:"failure_disable"`
	FailureTempUnschedulableMinutes *int `json:"failure_temp_unschedulable_minutes"`
	SuccessAddGroupID          *int64 `json:"success_add_group_id"`
	SuccessEnable              *bool  `json:"success_enable"`
	SuccessRecover             *bool  `json:"success_recover"`
}

type updateScheduledTestPlanRequest struct {
	ModelID        string `json:"model_id"`
	ReasoningEffort *string `json:"reasoning_effort"`
	PromptText     string `json:"prompt_text"`
	ExpectedAnswer string `json:"expected_answer"`
	CronExpression string `json:"cron_expression"`
	Enabled        *bool  `json:"enabled"`
	MaxResults     int    `json:"max_results"`
	AutoRecover                *bool  `json:"auto_recover"`
	TempUnschedulableEnabled   *bool  `json:"temp_unschedulable_enabled"`
	TempUnschedulableErrorCodes []int `json:"temp_unschedulable_error_codes"`
	TempUnschedulableMinutes   *int   `json:"temp_unschedulable_minutes"`
	FailureRemoveGroupID       *int64 `json:"failure_remove_group_id"`
	FailureRemoveGroupIDs      []int64 `json:"failure_remove_group_ids"`
	FailureDisable             *bool  `json:"failure_disable"`
	FailureTempUnschedulableMinutes *int `json:"failure_temp_unschedulable_minutes"`
	SuccessAddGroupID          *int64 `json:"success_add_group_id"`
	SuccessEnable              *bool  `json:"success_enable"`
	SuccessRecover             *bool  `json:"success_recover"`
}

// ListByAccount GET /admin/accounts/:id/scheduled-test-plans
func (h *ScheduledTestHandler) ListByAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid account id")
		return
	}

	plans, err := h.scheduledTestSvc.ListPlansByAccount(c.Request.Context(), accountID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, plans)
}

// Create POST /admin/scheduled-test-plans
func (h *ScheduledTestHandler) Create(c *gin.Context) {
	var req createScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	plan := &service.ScheduledTestPlan{
		AccountID:      req.AccountID,
		ModelID:        req.ModelID,
		ReasoningEffort: req.ReasoningEffort,
		PromptText:     req.PromptText,
		ExpectedAnswer: req.ExpectedAnswer,
		CronExpression: req.CronExpression,
		Enabled:        true,
		MaxResults:     req.MaxResults,
	}
	if req.Enabled != nil {
		plan.Enabled = *req.Enabled
	}
	if req.AutoRecover != nil {
		plan.AutoRecover = *req.AutoRecover
	}
	if req.TempUnschedulableEnabled != nil {
		plan.TempUnschedulableEnabled = *req.TempUnschedulableEnabled
	}
	plan.TempUnschedulableErrorCodes = req.TempUnschedulableErrorCodes
	if req.TempUnschedulableMinutes != nil {
		plan.TempUnschedulableMinutes = *req.TempUnschedulableMinutes
	}
	plan.FailureRemoveGroupID = req.FailureRemoveGroupID
	plan.FailureRemoveGroupIDs = req.FailureRemoveGroupIDs
	if req.FailureDisable != nil { plan.FailureDisable = *req.FailureDisable }
	if req.FailureTempUnschedulableMinutes != nil { plan.FailureTempUnschedulableMinutes = *req.FailureTempUnschedulableMinutes }
	plan.SuccessAddGroupID = req.SuccessAddGroupID
	if req.SuccessEnable != nil { plan.SuccessEnable = *req.SuccessEnable }
	if req.SuccessRecover != nil { plan.SuccessRecover = *req.SuccessRecover }

	created, err := h.scheduledTestSvc.CreatePlan(c.Request.Context(), plan)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, created)
}

// Update PUT /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Update(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	existing, err := h.scheduledTestSvc.GetPlan(c.Request.Context(), planID)
	if err != nil {
		response.NotFound(c, "plan not found")
		return
	}

	var req updateScheduledTestPlanRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if req.ModelID != "" {
		existing.ModelID = req.ModelID
	}
	if req.ReasoningEffort != nil {
		existing.ReasoningEffort = *req.ReasoningEffort
	}
	if req.PromptText != "" || existing.PromptText != "" { existing.PromptText = req.PromptText }
	if req.ExpectedAnswer != "" || existing.ExpectedAnswer != "" { existing.ExpectedAnswer = req.ExpectedAnswer }
	if req.CronExpression != "" {
		existing.CronExpression = req.CronExpression
	}
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
	if req.MaxResults > 0 {
		existing.MaxResults = req.MaxResults
	}
	if req.AutoRecover != nil {
		existing.AutoRecover = *req.AutoRecover
	}
	if req.TempUnschedulableEnabled != nil {
		existing.TempUnschedulableEnabled = *req.TempUnschedulableEnabled
	}
	if req.TempUnschedulableErrorCodes != nil {
		existing.TempUnschedulableErrorCodes = req.TempUnschedulableErrorCodes
	}
	if req.TempUnschedulableMinutes != nil {
		existing.TempUnschedulableMinutes = *req.TempUnschedulableMinutes
	}
	if req.FailureRemoveGroupID != nil { if *req.FailureRemoveGroupID <= 0 {
			existing.FailureRemoveGroupID = nil
		} else {
			existing.FailureRemoveGroupID = req.FailureRemoveGroupID
		} }
	if req.FailureRemoveGroupIDs != nil { existing.FailureRemoveGroupIDs = req.FailureRemoveGroupIDs }
	if req.FailureDisable != nil { existing.FailureDisable = *req.FailureDisable }
	if req.FailureTempUnschedulableMinutes != nil { existing.FailureTempUnschedulableMinutes = *req.FailureTempUnschedulableMinutes }
	if req.SuccessAddGroupID != nil { if *req.SuccessAddGroupID <= 0 {
			existing.SuccessAddGroupID = nil
		} else {
			existing.SuccessAddGroupID = req.SuccessAddGroupID
		} }
	if req.SuccessEnable != nil { existing.SuccessEnable = *req.SuccessEnable }
	if req.SuccessRecover != nil { existing.SuccessRecover = *req.SuccessRecover }

	updated, err := h.scheduledTestSvc.UpdatePlan(c.Request.Context(), existing)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, updated)
}

// RunNow POST /admin/scheduled-test-plans/:id/run
func (h *ScheduledTestHandler) RunNow(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}
	if h.runner == nil {
		response.InternalError(c, "scheduled test runner unavailable")
		return
	}
	result, err := h.runner.RunPlanNow(c.Request.Context(), planID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, result)
}

// Delete DELETE /admin/scheduled-test-plans/:id
func (h *ScheduledTestHandler) Delete(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	if err := h.scheduledTestSvc.DeletePlan(c.Request.Context(), planID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ListResults GET /admin/scheduled-test-plans/:id/results
func (h *ScheduledTestHandler) ListResults(c *gin.Context) {
	planID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid plan id")
		return
	}

	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}

	results, err := h.scheduledTestSvc.ListResults(c.Request.Context(), planID, limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	c.JSON(http.StatusOK, results)
}
