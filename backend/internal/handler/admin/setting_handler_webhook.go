package admin

import (
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *SettingHandler) SetNotificationWebhookService(svc *service.NotificationWebhookService) {
	h.notificationWebhookService = svc
}

func (h *SettingHandler) GetNotificationWebhookSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	settings, err := h.notificationWebhookService.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) UpdateNotificationWebhookSettings(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	var input struct {
		Enabled     *bool   `json:"enabled" binding:"required"`
		URL         string  `json:"webhook_url" binding:"max=4096"`
		Secret      string  `json:"secret" binding:"max=512"`
		ClearSecret bool    `json:"clear_secret"`
		Version     *string `json:"version" binding:"required,max=64"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16<<10)
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "Invalid webhook settings")
		return
	}
	settings, err := h.notificationWebhookService.UpdateSettings(c.Request.Context(), service.UpdateNotificationWebhookSettings{
		Enabled: *input.Enabled, URL: input.URL, Secret: input.Secret, ClearSecret: input.ClearSecret, Version: *input.Version,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}

func (h *SettingHandler) ListNotificationWebhookDeliveries(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	items, err := h.notificationWebhookService.ListRecent(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}

func (h *SettingHandler) TestNotificationWebhook(c *gin.Context) {
	if err := h.notificationWebhookService.SendTest(c.Request.Context()); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"success": true})
}
