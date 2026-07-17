package admin

import (
	"mime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type TicketHandler struct {
	service *service.TicketService
}

func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{service: ticketService}
}

func (h *TicketHandler) List(c *gin.Context) {
	page, pageSize := response.ParsePagination(c)
	search := strings.TrimSpace(c.Query("search"))
	if runes := []rune(search); len(runes) > 200 {
		search = string(runes[:200])
	}
	items, result, err := h.service.ListAdmin(c.Request.Context(), pagination.PaginationParams{
		Page: page, PageSize: pageSize, SortBy: c.DefaultQuery("sort_by", "last_message_at"), SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.TicketListFilters{
		Status: strings.TrimSpace(c.Query("status")), Priority: strings.TrimSpace(c.Query("priority")),
		Category: strings.TrimSpace(c.Query("category")), Search: search,
	})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, dto.TicketsFromService(items, true), result.Total, page, pageSize)
}

func (h *TicketHandler) Get(c *gin.Context) {
	ticketID, actorID, ok := adminTicketRequestIDs(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), ticketID, actorID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item, true))
}

func (h *TicketHandler) Reply(c *gin.Context) {
	ticketID, actorID, ok := adminTicketRequestIDs(c)
	if !ok {
		return
	}
	_, _, content, uploads, err := dto.ParseTicketMultipart(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Reply(c.Request.Context(), ticketID, actorID, true, content, uploads)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item, true))
}

type updateTicketRequest struct {
	Status   string `json:"status"`
	Priority string `json:"priority"`
}

func (h *TicketHandler) Update(c *gin.Context) {
	ticketID, _, ok := adminTicketRequestIDs(c)
	if !ok {
		return
	}
	var req updateTicketRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	item, err := h.service.AdminUpdate(c.Request.Context(), ticketID, strings.TrimSpace(req.Status), strings.TrimSpace(req.Priority))
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item, true))
}

func (h *TicketHandler) Attachment(c *gin.Context) {
	ticketID, actorID, ok := adminTicketRequestIDs(c)
	if !ok {
		return
	}
	messageID, err := strconv.ParseInt(c.Param("message_id"), 10, 64)
	if err != nil || messageID <= 0 {
		response.BadRequest(c, "Invalid message ID")
		return
	}
	index, err := strconv.Atoi(c.Param("index"))
	if err != nil || index < 0 {
		response.BadRequest(c, "Invalid attachment index")
		return
	}
	reader, attachment, err := h.service.DownloadAttachment(c.Request.Context(), ticketID, messageID, index, actorID, true)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer func() { _ = reader.Close() }()
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": attachment.Name})
	c.DataFromReader(200, attachment.Size, attachment.ContentType, reader, map[string]string{
		"Content-Disposition": disposition,
		"Cache-Control":       "private, max-age=300",
	})
}

func (h *TicketHandler) GetStorageConfig(c *gin.Context) {
	cfg, err := h.service.GetStorageConfig(c.Request.Context())
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *TicketHandler) UpdateStorageConfig(c *gin.Context) {
	var req service.TicketStorageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	cfg, err := h.service.UpdateStorageConfig(c.Request.Context(), req)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, cfg)
}

func (h *TicketHandler) TestStorage(c *gin.Context) {
	var req service.TicketStorageConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "Invalid request: "+err.Error())
		return
	}
	if err := h.service.TestStorage(c.Request.Context(), req); err != nil {
		response.Success(c, gin.H{"ok": false, "message": err.Error()})
		return
	}
	response.Success(c, gin.H{"ok": true, "message": "connection successful"})
}

func adminTicketRequestIDs(c *gin.Context) (int64, int64, bool) {
	subject, ok := middleware.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return 0, 0, false
	}
	ticketID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || ticketID <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return 0, 0, false
	}
	return ticketID, subject.UserID, true
}
