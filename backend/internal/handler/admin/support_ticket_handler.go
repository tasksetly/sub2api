package admin

import (
	"strconv"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/handler/ticketupload"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type SupportTicketHandler struct {
	service *service.SupportTicketService
}

func NewSupportTicketHandler(ticketService *service.SupportTicketService) *SupportTicketHandler {
	return &SupportTicketHandler{service: ticketService}
}

type adminSupportTicketReplyRequest struct {
	Content string `json:"content"`
}

func (h *SupportTicketHandler) AttachmentPolicy(c *gin.Context) {
	response.Success(c, h.service.AttachmentPolicy())
}

type adminSupportTicketUpdateRequest struct {
	Status   *string `json:"status"`
	Priority *string `json:"priority"`
}

func (h *SupportTicketHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListForAdmin(c.Request.Context(), pagination.PaginationParams{
		Page: page, PageSize: pageSize, SortBy: c.DefaultQuery("sort_by", "last_message_at"), SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, adminSupportTicketFiltersFromQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *SupportTicketHandler) Get(c *gin.Context) {
	ticketID, ok := adminSupportTicketID(c)
	if !ok {
		return
	}
	item, err := h.service.GetForAdmin(c.Request.Context(), ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *SupportTicketHandler) Reply(c *gin.Context) {
	ticketID, ok := adminSupportTicketID(c)
	if !ok {
		return
	}
	var req adminSupportTicketReplyRequest
	var uploads []service.SupportTicketAttachmentUpload
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		var err error
		uploads, err = ticketupload.Parse(c, h.service.AttachmentPolicy())
		if err != nil {
			response.ErrorFrom(c, err)
			return
		}
		req.Content = c.PostForm("content")
	} else {
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	item, err := h.service.ReplyAsAdminWithAttachments(c.Request.Context(), subject.UserID, ticketID, req.Content, uploads)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *SupportTicketHandler) Update(c *gin.Context) {
	ticketID, ok := adminSupportTicketID(c)
	if !ok {
		return
	}
	var req adminSupportTicketUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	item, err := h.service.UpdateAsAdmin(c.Request.Context(), subject.UserID, ticketID, service.UpdateSupportTicketInput{Status: req.Status, Priority: req.Priority})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func adminSupportTicketID(c *gin.Context) (int64, bool) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		response.BadRequest(c, "Invalid support ticket ID")
		return 0, false
	}
	return id, true
}

func adminSupportTicketFiltersFromQuery(c *gin.Context) service.SupportTicketListFilters {
	search := strings.TrimSpace(c.Query("search"))
	if len([]rune(search)) > 200 {
		search = string([]rune(search)[:200])
	}
	return service.SupportTicketListFilters{
		Status: strings.TrimSpace(c.Query("status")), Category: strings.TrimSpace(c.Query("category")),
		Priority: strings.TrimSpace(c.Query("priority")), Search: search,
	}
}
