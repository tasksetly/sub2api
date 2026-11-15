package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type globalIPBlacklistResponse struct {
	*service.GlobalIPBlacklistSettings
	CurrentIP string `json:"current_ip"`
	MaxRules  int    `json:"max_rules"`
}

func (h *SettingHandler) GetGlobalIPBlacklistSettings(c *gin.Context) {
	settings, err := h.settingService.GetGlobalIPBlacklistSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, globalIPBlacklistResponse{
		GlobalIPBlacklistSettings: settings,
		CurrentIP:                 ip.GetTrustedClientIP(c),
		MaxRules:                  service.GlobalIPBlacklistMaxRules,
	})
}

func (h *SettingHandler) UpdateGlobalIPBlacklistSettings(c *gin.Context) {
	var req struct {
		Enabled *bool     `json:"enabled" binding:"required"`
		Rules   *[]string `json:"rules" binding:"required"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	currentIP := ip.GetTrustedClientIP(c)
	settings, err := h.settingService.SetGlobalIPBlacklistSettings(c.Request.Context(), &service.GlobalIPBlacklistSettings{
		Enabled: *req.Enabled, Rules: *req.Rules,
	}, currentIP)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, globalIPBlacklistResponse{
		GlobalIPBlacklistSettings: settings,
		CurrentIP:                 currentIP,
		MaxRules:                  service.GlobalIPBlacklistMaxRules,
	})
}
