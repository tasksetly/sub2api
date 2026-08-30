package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"

	"github.com/gin-gonic/gin"
)

// UpstreamProviderHandler 管理上游 sub2api 供应商。
type UpstreamProviderHandler struct {
	svc *service.UpstreamProviderService
}

func NewUpstreamProviderHandler(svc *service.UpstreamProviderService) *UpstreamProviderHandler {
	return &UpstreamProviderHandler{svc: svc}
}

// CreateUpstreamProviderRequest 新增上游。
type CreateUpstreamProviderRequest struct {
	Name    string `json:"name" binding:"required"`
	BaseURL string `json:"base_url" binding:"required"`
	// Username 仍必填：即使只用 token，列表页也要有个标识看是哪个上游账号。
	Username string `json:"username" binding:"required"`
	// Password 与 Token 二选一，由 service 校验：上游做了 CF 校验时密码登不上去。
	Password string `json:"password"`
	// Token 是管理员从浏览器里拿到的上游 JWT，直接写入会话缓存。
	Token string `json:"token"`
	// RateCorrection 充值比例修正系数（充值 10 倍填 0.1）。省略按 1.0 处理。
	RateCorrection *float64 `json:"rate_correction" binding:"omitempty,gt=0"`
	TotpSecret     string   `json:"totp_secret"`
	Notes          *string  `json:"notes"`
	SyncEnabled    *bool    `json:"sync_enabled"`
}

// UpdateUpstreamProviderRequest 编辑上游。
// Password/TotpSecret/Token 留空表示不修改。
type UpdateUpstreamProviderRequest struct {
	Name     string `json:"name" binding:"required"`
	BaseURL  string `json:"base_url" binding:"required"`
	Username string `json:"username" binding:"required"`
	Password string `json:"password"`
	// Token 非空时顶掉缓存的会话；与 Password 同时填时以 Token 为准。
	Token string `json:"token"`
	// RateCorrection 省略表示不修改
	RateCorrection *float64 `json:"rate_correction" binding:"omitempty,gt=0"`
	TotpSecret     string   `json:"totp_secret"`
	Notes          *string  `json:"notes"`
	Status         string   `json:"status" binding:"omitempty,oneof=active inactive"`
	SyncEnabled    *bool    `json:"sync_enabled"`
}

// ProvisionAccountsRequest 勾选上游分组后创建 Key 与本地账号。
type ProvisionAccountsRequest struct {
	RemoteGroupIDs []int64 `json:"remote_group_ids" binding:"required,min=1"`
	LocalGroupIDs  []int64 `json:"local_group_ids"`
	Concurrency    int     `json:"concurrency" binding:"omitempty,min=1"`
	Priority       int     `json:"priority" binding:"omitempty,min=0"`
	KeyNamePrefix  string  `json:"key_name_prefix"`
}

// List 列出上游供应商（含余额、并发、分组倍率区间与本地实际成本）。
// GET /api/v1/admin/upstream-providers
func (h *UpstreamProviderHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	status := c.Query("status")
	search := strings.TrimSpace(c.Query("search"))
	if len(search) > 100 {
		search = search[:100]
	}

	params := pagination.PaginationParams{Page: page, PageSize: pageSize}
	providers, result, err := h.svc.List(c.Request.Context(), params, status, search)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}

	out := make([]dto.UpstreamProviderWithStats, 0, len(providers))
	for i := range providers {
		out = append(out, *dto.UpstreamProviderWithStatsFromService(&providers[i]))
	}
	total := int64(0)
	if result != nil {
		total = result.Total
	}
	response.Paginated(c, out, total, page, pageSize)
}

// GetByID 取单个上游。
// GET /api/v1/admin/upstream-providers/:id
func (h *UpstreamProviderHandler) GetByID(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	provider, err := h.svc.GetByID(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UpstreamProviderFromService(provider))
}

// Create 新增上游。
// POST /api/v1/admin/upstream-providers
func (h *UpstreamProviderHandler) Create(c *gin.Context) {
	var req CreateUpstreamProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	syncEnabled := true
	if req.SyncEnabled != nil {
		syncEnabled = *req.SyncEnabled
	}

	// 省略即 0，由 service.NormalizeRateCorrection 收敛成 1.0（不修正）
	rateCorrection := float64(0)
	if req.RateCorrection != nil {
		rateCorrection = *req.RateCorrection
	}

	provider, err := h.svc.Create(c.Request.Context(), service.CreateUpstreamProviderInput{
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		Username:       req.Username,
		Password:       req.Password,
		Token:          req.Token,
		RateCorrection: rateCorrection,
		TotpSecret:     req.TotpSecret,
		Notes:          req.Notes,
		SyncEnabled:    syncEnabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.UpstreamProviderFromService(provider))
}

// Update 编辑上游。
// PUT /api/v1/admin/upstream-providers/:id
func (h *UpstreamProviderHandler) Update(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	var req UpdateUpstreamProviderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	syncEnabled := true
	if req.SyncEnabled != nil {
		syncEnabled = *req.SyncEnabled
	}

	provider, err := h.svc.Update(c.Request.Context(), id, service.UpdateUpstreamProviderInput{
		Name:           req.Name,
		BaseURL:        req.BaseURL,
		Username:       req.Username,
		Password:       req.Password,
		Token:          req.Token,
		RateCorrection: req.RateCorrection,
		TotpSecret:     req.TotpSecret,
		Notes:          req.Notes,
		Status:         req.Status,
		SyncEnabled:    syncEnabled,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UpstreamProviderFromService(provider))
}

// Delete 删除上游。已创建的本地账号不受影响（外键 ON DELETE SET NULL）。
// DELETE /api/v1/admin/upstream-providers/:id
func (h *UpstreamProviderHandler) Delete(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(c.Request.Context(), id); err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"message": "Upstream provider deleted successfully"})
}

// TestConnection 验证账号密码能否登录上游。
// POST /api/v1/admin/upstream-providers/:id/test
func (h *UpstreamProviderHandler) TestConnection(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	profile, err := h.svc.TestConnection(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UpstreamProfileFromService(profile))
}

// Sync 手动同步余额、并发与分组倍率。
// POST /api/v1/admin/upstream-providers/:id/sync
func (h *UpstreamProviderHandler) Sync(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	provider, err := h.svc.Sync(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.UpstreamProviderFromService(provider))
}

// CompareGroups 跨上游拉平所有分组做横向比价，按倍率升序。
// GET /api/v1/admin/upstream-providers/groups/compare?platform=anthropic
func (h *UpstreamProviderHandler) CompareGroups(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	groups, result, err := h.svc.CompareGroups(
		c.Request.Context(), c.Query("platform"),
		pagination.PaginationParams{Page: page, PageSize: pageSize},
	)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UpstreamGroupComparison, 0, len(groups))
	for i := range groups {
		out = append(out, *dto.UpstreamGroupComparisonFromService(&groups[i]))
	}
	total := int64(0)
	if result != nil {
		total = result.Total
	}
	response.Paginated(c, out, total, page, pageSize)
}

// SyncAll 手动刷新全部启用了同步的上游。
//
// 与定时任务同一套逻辑，单个失败不影响其余，失败原因各自落 last_sync_error。
// POST /api/v1/admin/upstream-providers/sync-all
func (h *UpstreamProviderHandler) SyncAll(c *gin.Context) {
	succeeded, failed := h.svc.SyncAll(c.Request.Context())
	response.Success(c, gin.H{
		"succeeded": succeeded,
		"failed":    failed,
	})
}

// ListGroups 列出该上游同步下来的分组（含倍率与限额），供勾选建号。
// GET /api/v1/admin/upstream-providers/:id/groups
func (h *UpstreamProviderHandler) ListGroups(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	groups, err := h.svc.ListGroups(c.Request.Context(), id)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	out := make([]dto.UpstreamGroup, 0, len(groups))
	for i := range groups {
		out = append(out, *dto.UpstreamGroupFromService(&groups[i]))
	}
	response.Success(c, out)
}

// ProvisionAccounts 对勾选的上游分组创建 API Key 并落地本地账号。
// POST /api/v1/admin/upstream-providers/:id/provision
func (h *UpstreamProviderHandler) ProvisionAccounts(c *gin.Context) {
	id, ok := parseUpstreamProviderID(c)
	if !ok {
		return
	}
	var req ProvisionAccountsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}

	results, err := h.svc.ProvisionAccounts(c.Request.Context(), service.ProvisionAccountInput{
		ProviderID:     id,
		RemoteGroupIDs: req.RemoteGroupIDs,
		LocalGroupIDs:  req.LocalGroupIDs,
		Concurrency:    req.Concurrency,
		Priority:       req.Priority,
		KeyNamePrefix:  req.KeyNamePrefix,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, gin.H{"results": results})
}

func parseUpstreamProviderID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid upstream provider ID")
		return 0, false
	}
	return id, true
}
