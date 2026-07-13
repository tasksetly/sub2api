package handler

import (
	"strconv"
	"strings"

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

type createSupportTicketRequest struct {
	Subject  string `json:"subject" binding:"required"`
	Category string `json:"category"`
	Priority string `json:"priority"`
	Content  string `json:"content" binding:"required"`
}

type supportTicketReplyRequest struct {
	Content string `json:"content" binding:"required"`
}

func (h *SupportTicketHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListForUser(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page: page, PageSize: pageSize, SortBy: c.DefaultQuery("sort_by", "last_message_at"), SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, SupportTicketFiltersFromQuery(c))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, items, result.Total, page, pageSize)
}

func (h *SupportTicketHandler) Get(c *gin.Context) {
	userID, ticketID, ok := supportTicketUserAndID(c)
	if !ok {
		return
	}
	item, err := h.service.GetForUser(c.Request.Context(), userID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *SupportTicketHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req createSupportTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.CreateForUser(c.Request.Context(), subject.UserID, service.CreateSupportTicketInput{
		Subject: req.Subject, Category: req.Category, Priority: req.Priority, Content: req.Content,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *SupportTicketHandler) Reply(c *gin.Context) {
	userID, ticketID, ok := supportTicketUserAndID(c)
	if !ok {
		return
	}
	var req supportTicketReplyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.ReplyAsUser(c.Request.Context(), userID, ticketID, req.Content)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *SupportTicketHandler) Close(c *gin.Context) {
	userID, ticketID, ok := supportTicketUserAndID(c)
	if !ok {
		return
	}
	item, err := h.service.CloseAsUser(c.Request.Context(), userID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func (h *SupportTicketHandler) Reopen(c *gin.Context) {
	userID, ticketID, ok := supportTicketUserAndID(c)
	if !ok {
		return
	}
	item, err := h.service.ReopenAsUser(c.Request.Context(), userID, ticketID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, item)
}

func supportTicketUserAndID(c *gin.Context) (int64, int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return 0, 0, false
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid support ticket ID")
		return 0, 0, false
	}
	return subject.UserID, ticketID, true
}

func SupportTicketFiltersFromQuery(c *gin.Context) service.SupportTicketListFilters {
	search := strings.TrimSpace(c.Query("search"))
	if len([]rune(search)) > 200 {
		search = string([]rune(search)[:200])
	}
	return service.SupportTicketListFilters{
		Status: strings.TrimSpace(c.Query("status")), Category: strings.TrimSpace(c.Query("category")),
		Priority: strings.TrimSpace(c.Query("priority")), Search: search,
	}
}
