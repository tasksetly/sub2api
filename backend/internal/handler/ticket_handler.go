package handler

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
)

type TicketHandler struct{ service *service.TicketService }

func NewTicketHandler(s *service.TicketService) *TicketHandler { return &TicketHandler{s} }

type createTicketRequest struct {
	Subject  string `json:"subject" binding:"required"`
	Category string `json:"category" binding:"required"`
	Content  string `json:"content" binding:"required"`
}
type ticketMessageRequest struct {
	Content string `json:"content" binding:"required"`
}

func servicePagination(page, pageSize int) pagination.PaginationParams {
	return pagination.PaginationParams{Page: page, PageSize: pageSize}
}
func ticketID(c *gin.Context) (int64, bool) {
	id, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || id <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return 0, false
	}
	return id, true
}
func (h *TicketHandler) List(c *gin.Context) {
	sub, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	p, ps := response.ParsePagination(c)
	items, page, e := h.service.ListForUser(c, sub.UserID, servicePagination(p, ps))
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Paginated(c, items, page.Total, p, ps)
}
func (h *TicketHandler) Create(c *gin.Context) {
	sub, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	var r createTicketRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		response.BadRequest(c, "Invalid request: "+e.Error())
		return
	}
	out, e := h.service.Create(c, sub.UserID, service.CreateTicketInput{Subject: r.Subject, Category: r.Category, Content: r.Content})
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, out)
}
func (h *TicketHandler) Get(c *gin.Context) {
	sub, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := ticketID(c)
	if !ok {
		return
	}
	out, e := h.service.GetForUser(c, sub.UserID, id)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, out)
}
func (h *TicketHandler) AddMessage(c *gin.Context) {
	sub, ok := middleware2.GetAuthSubjectFromContext(c)
	if !ok {
		response.Unauthorized(c, "User not found in context")
		return
	}
	id, ok := ticketID(c)
	if !ok {
		return
	}
	var r ticketMessageRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		response.BadRequest(c, "Invalid request: "+e.Error())
		return
	}
	out, e := h.service.AddUserMessage(c, sub.UserID, id, r.Content)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, out)
}
