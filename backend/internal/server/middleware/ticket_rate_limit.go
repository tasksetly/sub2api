package middleware

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// TicketSend 独立于面板限流设置，新建和回复共享账号及 IP 每分钟一次的额度。
func (p *PanelRateLimiter) TicketSend() gin.HandlerFunc {
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
		// 沿用原有 Redis key，避免升级时重置尚未到期的新建工单额度。
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
