package admin

import (
	"net/http"
	"strconv"

	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

func (h *IntelligenceTestHandler) RunAccount(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	var input service.AccountIntelligenceTestInput
	if err := c.ShouldBindJSON(&input); err != nil {
		response.BadRequest(c, "invalid intelligence test input")
		return
	}
	result, err := h.svc.QueueAccountRun(c.Request.Context(), accountID, input)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (h *IntelligenceTestHandler) ListAccountResults(c *gin.Context) {
	accountID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || accountID <= 0 {
		response.BadRequest(c, "invalid account id")
		return
	}
	beforeID, err := strconv.ParseInt(c.DefaultQuery("before_id", "0"), 10, 64)
	if err != nil || beforeID < 0 {
		response.BadRequest(c, "invalid result cursor")
		return
	}
	items, err := h.svc.ListAccountResults(c.Request.Context(), accountID, beforeID)
	if err != nil {
		response.ErrorFrom(c, err)
		return
	}
	c.JSON(http.StatusOK, items)
}
