package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// TicketCreate 独立于面板限流设置，账号或 IP 任一维度每分钟最多提交一次。
func (p *PanelRateLimiter) TicketCreate() gin.HandlerFunc {
	return func(c *gin.Context) {
		subject, ok := GetAuthSubjectFromContext(c)
		if !ok || subject.UserID <= 0 {
			AbortWithError(c, http.StatusUnauthorized, "UNAUTHORIZED", "User not found in context")
			return
		}
		if p == nil || p.limiter == nil {
			AbortWithError(c, http.StatusServiceUnavailable, "TICKET_RATE_LIMIT_UNAVAILABLE", "Ticket submission is temporarily unavailable")
			return
		}
		for _, key := range []string{
			"ticket:create:user:" + strconv.FormatInt(subject.UserID, 10),
			"ticket:create:ip:" + SecurityClientIP(c),
		} {
			result, err := p.limiter.Allow(c.Request.Context(), key, 1, time.Minute)
			if err != nil {
				// Redis 故障时拒绝提交，避免限流失效后被刷工单。
				slog.Warn("ticket rate limit check failed", "error", err)
				AbortWithError(c, http.StatusServiceUnavailable, "TICKET_RATE_LIMIT_UNAVAILABLE", "Ticket submission is temporarily unavailable")
				return
			}
			if !result.Allowed {
				abortPanelRateLimited(c, result.RetryAfter)
				return
			}
		}
		c.Next()
	}
}
