package routes

import (
	"github.com/Wei-Shaw/sub2api/internal/handler"
	"github.com/gin-gonic/gin"
)

// RegisterIntelligenceTestRoutes exposes only the rendered benchmark data.
// API key credentials and account selection remain admin-only.
func RegisterIntelligenceTestRoutes(v1 *gin.RouterGroup, h *handler.Handlers) {
	v1.GET("/intelligence-test/history", h.Admin.IntelligenceTest.Public)
	v1.GET("/pelican-test/history", h.Admin.IntelligenceTest.Public)
}
