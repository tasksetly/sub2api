package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) GetUsageRetentionSettings(c *gin.Context) {
	policy, err := h.settingService.GetUsageRetentionSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}

func (h *SettingHandler) UpdateUsageRetentionSettings(c *gin.Context) {
	var req struct {
		Enabled       *bool `json:"enabled" binding:"required"`
		RetentionDays int   `json:"retention_days" binding:"required,min=1,max=3650"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "Unauthorized")
		return
	}
	policy := service.UsageRetentionSettings{Enabled: *req.Enabled, RetentionDays: req.RetentionDays}
	if err := h.settingService.SetUsageRetentionSettings(c.Request.Context(), policy, subject.UserID); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, policy)
}
