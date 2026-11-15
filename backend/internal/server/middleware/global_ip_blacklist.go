package middleware

import (
	"context"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/ip"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
)

type globalIPBlacklistChecker interface {
	IsGlobalIPBlocked(context.Context, string) (bool, error)
}

func RegistrationIPContext() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(service.WithRegistrationIP(c.Request.Context(), ip.GetTrustedClientIP(c)))
		c.Next()
	}
}

// GlobalIPBlacklist runs before CORS and frontend serving, with no route or
// administrator exemptions. Only the configured trusted proxy chain can supply
// a forwarded client address; arbitrary forwarding headers never bypass a ban.
func GlobalIPBlacklist(checker globalIPBlacklistChecker) gin.HandlerFunc {
	return func(c *gin.Context) {
		blocked, err := checker.IsGlobalIPBlocked(c.Request.Context(), ip.GetTrustedClientIP(c))
		if err != nil {
			c.Header("Cache-Control", "no-store")
			AbortWithError(c, http.StatusServiceUnavailable, "IP_BLACKLIST_UNAVAILABLE", "Access policy is temporarily unavailable")
			return
		}
		if blocked {
			c.Header("Cache-Control", "no-store")
			AbortWithError(c, http.StatusForbidden, "IP_BLOCKED", "Access denied")
			return
		}
		c.Next()
	}
}
