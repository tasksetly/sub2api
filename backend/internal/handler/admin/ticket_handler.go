package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	middleware2 "github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"strconv"
	"strings"
)

type TicketHandler struct{ service *service.TicketService }

func NewTicketHandler(s *service.TicketService) *TicketHandler { return &TicketHandler{s} }

type messageRequest struct {
	Content string `json:"content" binding:"required"`
}
type updateRequest struct {
	Status   *string `json:"status"`
	Priority *string `json:"priority"`
}

func id(c *gin.Context) (int64, bool) {
	v, e := strconv.ParseInt(c.Param("id"), 10, 64)
	if e != nil || v <= 0 {
		response.BadRequest(c, "Invalid ticket ID")
		return 0, false
	}
	return v, true
}
func (h *TicketHandler) List(c *gin.Context) {
	p, ps := response.ParsePagination(c)
	items, page, e := h.service.ListForAdmin(c, pagination.PaginationParams{Page: p, PageSize: ps}, service.TicketListFilters{Status: strings.TrimSpace(c.Query("status")), Category: strings.TrimSpace(c.Query("category")), Priority: strings.TrimSpace(c.Query("priority")), Search: strings.TrimSpace(c.Query("search"))})
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Paginated(c, items, page.Total, p, ps)
}
func (h *TicketHandler) Get(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	out, e := h.service.GetForAdmin(c, v)
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
	v, ok := id(c)
	if !ok {
		return
	}
	var r messageRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		response.BadRequest(c, "Invalid request: "+e.Error())
		return
	}
	out, e := h.service.AddAdminMessage(c, sub.UserID, v, r.Content)
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, out)
}
func (h *TicketHandler) Update(c *gin.Context) {
	v, ok := id(c)
	if !ok {
		return
	}
	var r updateRequest
	if e := c.ShouldBindJSON(&r); e != nil {
		response.BadRequest(c, "Invalid request: "+e.Error())
		return
	}
	out, e := h.service.UpdateByAdmin(c, v, service.UpdateTicketInput{Status: r.Status, Priority: r.Priority})
	if e != nil {
		response.ErrorFrom(c, e)
		return
	}
	response.Success(c, out)
}
