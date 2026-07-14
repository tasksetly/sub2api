package handler

import (
	"encoding/json"
	"log/slog"
	"mime"
	"net/http"
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

type createSupportTicketRequest struct {
	Subject     string          `json:"subject" binding:"required"`
	Category    string          `json:"category"`
	Priority    string          `json:"priority"`
	Content     string          `json:"content" binding:"required"`
	Attachments json.RawMessage `json:"attachments"`
}

type supportTicketReplyRequest struct {
	Content     string          `json:"content"`
	Attachments json.RawMessage `json:"attachments"`
}

func (h *SupportTicketHandler) AttachmentPolicy(c *gin.Context) {
	response.Success(c, h.service.AttachmentPolicy())
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

func (h *SupportTicketHandler) DownloadAttachment(c *gin.Context) {
	userID, ticketID, ok := supportTicketUserAndID(c)
	if !ok {
		return
	}
	attachmentID, ok := supportTicketAttachmentID(c)
	if !ok {
		return
	}
	download, err := h.service.DownloadAttachmentForUser(c.Request.Context(), userID, ticketID, attachmentID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	defer download.Body.Close()
	fileName := mime.FormatMediaType("attachment", map[string]string{"filename": download.Attachment.FileName})
	c.DataFromReader(http.StatusOK, download.Attachment.SizeBytes, download.Attachment.ContentType, download.Body, map[string]string{"Content-Disposition": fileName})
}

func (h *SupportTicketHandler) Create(c *gin.Context) {
	subject, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var req createSupportTicketRequest
	var uploads []service.SupportTicketAttachmentUpload
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		var err error
		uploads, err = ticketupload.Parse(c, h.service.AttachmentPolicy())
		if err != nil {
			slog.Warn("support_ticket_attachment multipart_parse_failed", "user_id", subject.UserID, "operation", "create", "error", err)
			response.ErrorFrom(c, err)
			return
		}
		slog.Info("support_ticket_attachment multipart_parsed", "user_id", subject.UserID, "operation", "create", "attachment_count", len(uploads))
		req = createSupportTicketRequest{
			Subject: c.PostForm("subject"), Category: c.PostForm("category"), Priority: c.PostForm("priority"), Content: c.PostForm("content"),
		}
	} else {
		slog.Info("support_ticket_attachment request_not_multipart", "user_id", subject.UserID, "operation", "create", "content_type", c.ContentType(), "content_length", c.Request.ContentLength)
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
		if hasJSONAttachments(req.Attachments) {
			response.BadRequest(c, "Support ticket attachments must be sent as multipart/form-data")
			return
		}
	}
	item, err := h.service.CreateForUser(c.Request.Context(), subject.UserID, service.CreateSupportTicketInput{
		Subject: req.Subject, Category: req.Category, Priority: req.Priority, Content: req.Content, Attachments: uploads,
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
	var uploads []service.SupportTicketAttachmentUpload
	if strings.HasPrefix(c.ContentType(), "multipart/form-data") {
		var err error
		uploads, err = ticketupload.Parse(c, h.service.AttachmentPolicy())
		if err != nil {
			slog.Warn("support_ticket_attachment multipart_parse_failed", "user_id", userID, "ticket_id", ticketID, "operation", "reply", "error", err)
			response.ErrorFrom(c, err)
			return
		}
		slog.Info("support_ticket_attachment multipart_parsed", "user_id", userID, "ticket_id", ticketID, "operation", "reply", "attachment_count", len(uploads))
		req.Content = c.PostForm("content")
	} else {
		slog.Info("support_ticket_attachment request_not_multipart", "user_id", userID, "ticket_id", ticketID, "operation", "reply", "content_type", c.ContentType(), "content_length", c.Request.ContentLength)
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "Invalid request: "+err.Error())
			return
		}
		if hasJSONAttachments(req.Attachments) {
			response.BadRequest(c, "Support ticket attachments must be sent as multipart/form-data")
			return
		}
	}
	item, err := h.service.ReplyAsUserWithAttachments(c.Request.Context(), userID, ticketID, req.Content, uploads)
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

func supportTicketAttachmentID(c *gin.Context) (int64, bool) {
	attachmentID, err := strconv.ParseInt(c.Param("attachmentID"), 10, 64)
	if err != nil || attachmentID <= 0 {
		response.BadRequest(c, "Invalid support ticket attachment ID")
		return 0, false
	}
	return attachmentID, true
}

func hasJSONAttachments(attachments json.RawMessage) bool {
	trimmed := strings.TrimSpace(string(attachments))
	return trimmed != "" && trimmed != "null"
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
