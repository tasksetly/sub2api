package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type IntelligenceTestHandler struct{ svc *service.IntelligenceTestService }

func NewIntelligenceTestHandler(svc *service.IntelligenceTestService) *IntelligenceTestHandler {
	return &IntelligenceTestHandler{svc: svc}
}

type intelligenceTestConfigRequest struct {
	Name           string `json:"name"`
	ModelID        string `json:"model_id"`
	ModelName      string `json:"model_name"`
	ReasoningEffort string `json:"reasoning_effort"`
	PromptText     string `json:"prompt_text"`
	APIKeyID       int64  `json:"api_key_id"`
	CronExpression string `json:"cron_expression"`
	KeepCount      int    `json:"keep_count"`
	Enabled        *bool  `json:"enabled"`
}

func (h *IntelligenceTestHandler) ListConfigs(c *gin.Context) {
	items, err := h.svc.ListConfigs(c.Request.Context())
	if err != nil { response.InternalError(c, err.Error()); return }
	c.JSON(http.StatusOK, items)
}

func (h *IntelligenceTestHandler) ListAPIKeys(c *gin.Context) {
	items, err := h.svc.ListAPIKeyOptions(c.Request.Context())
	if err != nil { response.InternalError(c, err.Error()); return }
	c.JSON(http.StatusOK, items)
}

func (h *IntelligenceTestHandler) Create(c *gin.Context) {
	var req intelligenceTestConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil { response.BadRequest(c, err.Error()); return }
	item, err := h.svc.CreateConfig(c.Request.Context(), toIntelligenceTestInput(req))
	if err != nil { response.BadRequest(c, err.Error()); return }
	c.JSON(http.StatusOK, item)
}

func (h *IntelligenceTestHandler) Update(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil { response.BadRequest(c, "invalid config id"); return }
	var req intelligenceTestConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil { response.BadRequest(c, err.Error()); return }
	item, err := h.svc.UpdateConfig(c.Request.Context(), id, toIntelligenceTestInput(req))
	if err != nil { response.BadRequest(c, err.Error()); return }
	c.JSON(http.StatusOK, item)
}

func (h *IntelligenceTestHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil { response.BadRequest(c, "invalid config id"); return }
	if err := h.svc.DeleteConfig(c.Request.Context(), id); err != nil { response.InternalError(c, err.Error()); return }
	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

func (h *IntelligenceTestHandler) Run(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil { response.BadRequest(c, "invalid config id"); return }
	result, err := h.svc.RunNow(c.Request.Context(), id)
	if err != nil { response.InternalError(c, err.Error()); return }
	c.JSON(http.StatusOK, result)
}

func (h *IntelligenceTestHandler) ListResults(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil { response.BadRequest(c, "invalid config id"); return }
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := h.svc.ListResults(c.Request.Context(), id, limit)
	if err != nil { response.InternalError(c, err.Error()); return }
	c.JSON(http.StatusOK, items)
}

func (h *IntelligenceTestHandler) Public(c *gin.Context) {
	id, _ := strconv.ParseInt(c.Query("config_id"), 10, 64)
	result, err := h.svc.Public(c.Request.Context(), id)
	if err != nil { response.InternalError(c, err.Error()); return }
	c.JSON(http.StatusOK, result)
}

func toIntelligenceTestInput(req intelligenceTestConfigRequest) service.IntelligenceTestConfigInput {
	return service.IntelligenceTestConfigInput{Name: req.Name, ModelID: req.ModelID, ModelName: req.ModelName, ReasoningEffort: req.ReasoningEffort, PromptText: req.PromptText, APIKeyID: req.APIKeyID, CronExpression: req.CronExpression, KeepCount: req.KeepCount, Enabled: req.Enabled}
}
