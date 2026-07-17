package handler

import (
	"mime"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Wei-Shaw/sub2api/internal/handler/dto"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type TicketHandler struct {
	service *service.TicketService
}

func NewTicketHandler(ticketService *service.TicketService) *TicketHandler {
	return &TicketHandler{service: ticketService}
}

func (h *TicketHandler) List(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	page, pageSize := response.ParsePagination(c)
	items, result, err := h.service.ListForUser(c.Request.Context(), subject.UserID, pagination.PaginationParams{
		Page: page, PageSize: pageSize, SortBy: c.DefaultQuery("sort_by", "last_message_at"), SortOrder: c.DefaultQuery("sort_order", "desc"),
	}, service.TicketListFilters{Status: strings.TrimSpace(c.Query("status")), Category: strings.TrimSpace(c.Query("category"))})
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Paginated(c, dto.TicketsFromService(items, false), result.Total, page, pageSize)
}

func (h *TicketHandler) Get(c *gin.Context) {
	ticketID, actorID, ok := ticketRequestIDs(c)
	if !ok {
		return
	}
	item, err := h.service.Get(c.Request.Context(), ticketID, actorID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item, false))
}

func (h *TicketHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	title, category, content, uploads, err := dto.ParseTicketMultipart(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Create(c.Request.Context(), subject.UserID, title, category, content, uploads)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Created(c, dto.TicketFromService(item, false))
}

func (h *TicketHandler) Reply(c *gin.Context) {
	ticketID, actorID, ok := ticketRequestIDs(c)
	if !ok {
		return
	}
	_, _, content, uploads, err := dto.ParseTicketMultipart(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	item, err := h.service.Reply(c.Request.Context(), ticketID, actorID, false, content, uploads)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item, false))
}

func (h *TicketHandler) Close(c *gin.Context) {
	ticketID, actorID, ok := ticketRequestIDs(c)
	if !ok {
		return
	}
	item, err := h.service.Close(c.Request.Context(), ticketID, actorID, false)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	response.Success(c, dto.TicketFromService(item, false))
}

func (h *TicketHandler) Attachment(c *gin.Context) {
	ticketID, actorID, ok := ticketRequestIDs(c)
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
	reader, attachment, err := h.service.DownloadAttachment(c.Request.Context(), ticketID, messageID, index, actorID, false)
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

func ticketRequestIDs(c *gin.Context) (int64, int64, bool) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
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
