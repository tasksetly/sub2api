package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type globalIPBlacklistCheckerFunc func(context.Context, string) (bool, error)

func (f globalIPBlacklistCheckerFunc) IsGlobalIPBlocked(ctx context.Context, clientIP string) (bool, error) {
	return f(ctx, clientIP)
}

func TestGlobalIPBlacklistCoversPagesAPIsAndPreflight(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.Use(GlobalIPBlacklist(globalIPBlacklistCheckerFunc(func(_ context.Context, clientIP string) (bool, error) {
		return clientIP == "203.0.113.10", nil
	})))
	router.Use(CORS(config.CORSConfig{AllowedOrigins: []string{"https://example.com"}}))
	called := false
	ok := func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	}
	paths := []string{"/", "/assets/app.js", "/api/v1/auth/register", "/api/v1/auth/login", "/api/v1/tickets", "/v1/messages", "/api/v1/admin/settings/global-ip-blacklist", "/health"}
	for _, path := range paths {
		router.Any(path, ok)
	}
	router.NoRoute(ok)
	for _, path := range append(paths, "/unknown") {
		for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodOptions} {
			request := httptest.NewRequest(method, path, nil)
			request.RemoteAddr = "203.0.113.10:1234"
			request.Header.Set("Origin", "https://example.com")
			request.Header.Set("Access-Control-Request-Method", "POST")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusForbidden, response.Code, method+" "+path)
			require.Contains(t, response.Body.String(), "IP_BLOCKED")
			require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		}
	}
	require.False(t, called)
}

func TestGlobalIPBlacklistUsesOnlyTrustedForwarding(t *testing.T) {
	for _, tc := range []struct {
		name    string
		proxies []string
		peer    string
		xff     string
		wantIP  string
	}{
		{"untrusted peer cannot spoof", nil, "203.0.113.10:1234", "192.0.2.1", "203.0.113.10"},
		{"trusted proxy uses client", []string{"10.0.0.0/8"}, "10.0.0.2:1234", "203.0.113.10", "203.0.113.10"},
		{"chain stops at untrusted hop", []string{"10.0.0.0/8"}, "10.0.0.2:1234", "192.0.2.1, 203.0.113.10, 10.0.0.3", "203.0.113.10"},
		{"unblocked direct peer", nil, "192.0.2.1:1234", "203.0.113.10", "192.0.2.1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gin.SetMode(gin.TestMode)
			router := gin.New()
			require.NoError(t, router.SetTrustedProxies(tc.proxies))
			router.Use(GlobalIPBlacklist(globalIPBlacklistCheckerFunc(func(_ context.Context, clientIP string) (bool, error) {
				require.Equal(t, tc.wantIP, clientIP)
				return clientIP == "203.0.113.10", nil
			})))
			router.GET("/", func(c *gin.Context) { c.Status(http.StatusOK) })
			request := httptest.NewRequest(http.MethodGet, "/", nil)
			request.RemoteAddr = tc.peer
			request.Header.Set("X-Forwarded-For", tc.xff)
			request.Header.Set("X-Real-IP", "192.0.2.99")
			request.Header.Set("CF-Connecting-IP", "192.0.2.99")
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if tc.wantIP == "203.0.113.10" {
				require.Equal(t, http.StatusForbidden, response.Code)
			} else {
				require.Equal(t, http.StatusOK, response.Code)
			}
		})
	}
}

func TestGlobalIPBlacklistFailsClosedWithoutPolicy(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.Use(GlobalIPBlacklist(globalIPBlacklistCheckerFunc(func(context.Context, string) (bool, error) {
		return false, errors.New("policy unavailable")
	})))
	called := false
	router.GET("/", func(c *gin.Context) {
		called = true
		c.Status(http.StatusOK)
	})
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusServiceUnavailable, response.Code)
	require.Contains(t, response.Body.String(), "IP_BLACKLIST_UNAVAILABLE")
	require.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	require.False(t, called)
}

func TestRegistrationIPContextOnlyRecordsAuthCreationContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	auth := router.Group("/auth", RegistrationIPContext())
	auth.POST("/register", func(c *gin.Context) {
		require.Equal(t, "203.0.113.10", service.RegistrationIPFromContext(c.Request.Context()))
		c.Status(http.StatusOK)
	})
	router.POST("/admin/users", func(c *gin.Context) {
		require.Empty(t, service.RegistrationIPFromContext(c.Request.Context()))
		c.Status(http.StatusOK)
	})
	for _, path := range []string{"/auth/register", "/admin/users"} {
		request := httptest.NewRequest(http.MethodPost, path, nil)
		request.RemoteAddr = "203.0.113.10:1234"
		request.Header.Set("X-Forwarded-For", "192.0.2.1")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		require.Equal(t, http.StatusOK, response.Code)
	}
}
