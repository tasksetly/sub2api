package handler

import (
	"context"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/httputil"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type CCTestHandler struct {
	svc         *service.CCTestService
	gateway     *service.GatewayService
	concurrency *service.ConcurrencyService
}

func NewCCTestHandler(svc *service.CCTestService, gateway *service.GatewayService, concurrency *service.ConcurrencyService) *CCTestHandler {
	return &CCTestHandler{svc: svc, gateway: gateway, concurrency: concurrency}
}

func (h *CCTestHandler) Settings(c *gin.Context) {
	settings, err := h.svc.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
func (h *CCTestHandler) Availability(c *gin.Context) {
	settings, err := h.svc.GetSettings(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"enabled": settings.Enabled && settings.AllowUsers})
}
func (h *CCTestHandler) SaveSettings(c *gin.Context) {
	var input service.CCTestSettingsInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid CCTest settings")
		return
	}
	settings, err := h.svc.SaveSettings(c.Request.Context(), input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, settings)
}
func (h *CCTestHandler) CreateAdmin(c *gin.Context) { h.create(c, true) }
func (h *CCTestHandler) CreateUser(c *gin.Context)  { h.create(c, false) }
func (h *CCTestHandler) create(c *gin.Context, admin bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	var input service.CCTestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid CCTest input")
		return
	}
	items, err := h.svc.Create(c.Request.Context(), subject.UserID, admin, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Accepted(c, items)
}
func (h *CCTestHandler) ListAdmin(c *gin.Context) { h.list(c, true) }
func (h *CCTestHandler) ListUser(c *gin.Context)  { h.list(c, false) }
func (h *CCTestHandler) list(c *gin.Context, admin bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	items, err := h.svc.List(c.Request.Context(), subject.UserID, admin, c.Query("before"))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, items)
}
func (h *CCTestHandler) CancelAdmin(c *gin.Context) { h.cancel(c, true) }
func (h *CCTestHandler) CancelUser(c *gin.Context)  { h.cancel(c, false) }
func (h *CCTestHandler) cancel(c *gin.Context, admin bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	if err := h.svc.Cancel(c.Request.Context(), c.Param("id"), subject.UserID, admin); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"cancelled": true})
}

// Relay runs before normal gateway authentication. Only the server substitutes the
// user's key; the usual auth, quota, rate, model allowlist and billing handlers follow.
func (h *CCTestHandler) Relay(c *gin.Context) {
	token := c.GetHeader("x-api-key")
	if token == "" {
		token = strings.TrimPrefix(c.GetHeader("Authorization"), "Bearer ")
	}
	body, err := io.ReadAll(http.MaxBytesReader(c.Writer, c.Request.Body, 4<<20))
	if err != nil {
		c.AbortWithStatusJSON(http.StatusRequestEntityTooLarge, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "test request too large"}})
		return
	}
	parsed, err := service.ParseGatewayRequest(service.NewRequestBodyRef(body), service.PlatformAnthropic)
	if err != nil || parsed.Model == "" || parsed.MaxTokens <= 0 || parsed.MaxTokens > 16384 {
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{"error": gin.H{"type": "invalid_request_error", "message": "test requests require a model and max_tokens between 1 and 16384"}})
		return
	}
	_, account, key, err := h.svc.ResolveRelay(c.Request.Context(), token, parsed.Model)
	if err != nil {
		response.ErrorFrom(c, err)
		c.Abort()
		return
	}
	c.Request.Body = httputil.NewPrereadBody(body)
	c.Request.ContentLength = int64(len(body))
	c.Request.Header.Del("x-api-key")
	c.Request.Header.Del("Authorization")
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Minute)
	defer cancel()
	c.Request = c.Request.WithContext(ctx)
	if key != nil {
		c.Request.Header.Set("Authorization", "Bearer "+key.Key)
		c.Next()
		return
	}
	// Admin probes deliberately pin this one account; a failure must not fail over
	// to another account and attribute its score to the selected target.
	c.Abort()
	slot, err := h.concurrency.AcquireAccountSlot(ctx, account.ID, account.Concurrency)
	if err != nil || slot == nil || !slot.Acquired {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"type": "rate_limit_error", "message": "test account is busy"}})
		return
	}
	defer slot.ReleaseFunc()
	_, err = h.gateway.Forward(ctx, c, account, parsed)
	if err != nil && !c.Writer.Written() {
		c.JSON(http.StatusBadGateway, gin.H{"error": gin.H{"type": "api_error", "message": "test upstream failed"}})
	}
}
