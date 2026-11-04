package middleware

import (
	"errors"
	"net/http"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestTicketCreateRateLimitByUserOrIP(t *testing.T) {
	for _, tc := range []struct {
		name string
		user int64
		ip   string
		want int
	}{
		{"same user different IP", 1, "198.51.100.2:1000", http.StatusTooManyRequests},
		{"different user same IP", 2, "203.0.113.1:1000", http.StatusTooManyRequests},
		{"different user different IP", 2, "198.51.100.2:1000", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// 无 SettingService，确保不依赖面板限流开关及管理员豁免。
			p := &PanelRateLimiter{limiter: &fakePanelAllower{}}
			first := newPanelTestRouter(p.TicketCreate(), &panelTestIdentity{userID: 1, role: service.RoleAdmin})
			require.Equal(t, http.StatusOK, performPanelRequest(first, "203.0.113.1:1000").Code)
			second := newPanelTestRouter(p.TicketCreate(), &panelTestIdentity{userID: tc.user, role: service.RoleAdmin})
			response := performPanelRequest(second, tc.ip)
			require.Equal(t, tc.want, response.Code)
			if tc.want == http.StatusTooManyRequests {
				require.Equal(t, "60", response.Header().Get("Retry-After"))
				require.Contains(t, response.Body.String(), "RATE_LIMITED")
			}
		})
	}
}

func TestTicketCreateRateLimitFailsClosed(t *testing.T) {
	for _, p := range []*PanelRateLimiter{
		nil,
		{},
		{limiter: &fakePanelAllower{err: errors.New("redis unavailable")}},
	} {
		router := newPanelTestRouter(p.TicketCreate(), &panelTestIdentity{userID: 1})
		response := performPanelRequest(router, "203.0.113.1:1000")
		require.Equal(t, http.StatusServiceUnavailable, response.Code)
		require.Contains(t, response.Body.String(), "TICKET_RATE_LIMIT_UNAVAILABLE")
	}
}

func TestTicketCreateRateLimitRequiresUser(t *testing.T) {
	p := &PanelRateLimiter{limiter: &fakePanelAllower{}}
	router := newPanelTestRouter(p.TicketCreate(), nil)
	require.Equal(t, http.StatusUnauthorized, performPanelRequest(router, "203.0.113.1:1000").Code)
}
