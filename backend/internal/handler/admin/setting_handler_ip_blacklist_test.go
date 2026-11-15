package admin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestGlobalIPBlacklistHandlerValidationAndSelfBlock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTestSettingRepo()
	original := `{"enabled":true,"rules":["198.51.100.1"]}`
	repo.values[service.SettingKeyGlobalIPBlacklist] = original
	h := &SettingHandler{settingService: service.NewSettingService(repo, &config.Config{})}
	snapshot, err := h.settingService.GetGlobalIPBlacklistSettings(context.Background())
	require.NoError(t, err)
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.PUT("/settings", h.UpdateGlobalIPBlacklistSettings)
	for _, body := range []string{
		`{}`, `{"enabled":false}`, `{"rules":[]}`, `{"enabled":null,"rules":[]}`,
		`{"enabled":true,"rules":null}`, `{"enabled":true,"rules":["example.com"]}`,
		`{"enabled":true,"rules":["203.0.113.0/24"]}`,
	} {
		t.Run(body, func(t *testing.T) {
			// A valid version ensures these cases still exercise field/rule and
			// trusted-client-IP validation instead of failing on a missing token.
			versioned := strings.TrimSuffix(body, "}")
			if versioned != "{" {
				versioned += ","
			}
			versioned += `"version":"` + snapshot.Version + `"}`
			request := httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(versioned))
			request.RemoteAddr = "203.0.113.10:1234"
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set("X-Forwarded-For", "192.0.2.1") // Cannot evade the self-lock guard.
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			require.Equal(t, http.StatusBadRequest, response.Code, response.Body.String())
			require.Equal(t, original, repo.values[service.SettingKeyGlobalIPBlacklist])
		})
	}

	// Explicit false and [] must work even though missing/null fields are rejected.
	request := httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(`{"enabled":false,"rules":[],"version":"`+snapshot.Version+`"}`))
	request.RemoteAddr = "203.0.113.10:1234"
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	var result struct {
		Data globalIPBlacklistResponse `json:"data"`
	}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &result))
	require.NotNil(t, result.Data.GlobalIPBlacklistSettings)
	require.False(t, result.Data.Enabled)
	require.NotNil(t, result.Data.Rules)
	require.Empty(t, result.Data.Rules)
	require.Equal(t, "203.0.113.10", result.Data.CurrentIP)
	require.Equal(t, service.GlobalIPBlacklistMaxRules, result.Data.MaxRules)
	require.Regexp(t, "^[a-f0-9]{64}$", result.Data.Version)
	require.NotEqual(t, snapshot.Version, result.Data.Version)
}

func TestGlobalIPBlacklistHandlerVersionValidationAndConflict(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newTestSettingRepo()
	h := &SettingHandler{settingService: service.NewSettingService(repo, &config.Config{})}
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.GET("/settings", h.GetGlobalIPBlacklistSettings)
	router.PUT("/settings", h.UpdateGlobalIPBlacklistSettings)
	request := func(method, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/settings", strings.NewReader(body))
		req.RemoteAddr = "192.0.2.1:1234"
		req.Header.Set("Content-Type", "application/json")
		res := httptest.NewRecorder()
		router.ServeHTTP(res, req)
		return res
	}
	read := func() globalIPBlacklistResponse {
		res := request(http.MethodGet, "")
		require.Equal(t, http.StatusOK, res.Code, res.Body.String())
		require.Equal(t, "no-store", res.Header().Get("Cache-Control"))
		var result struct {
			Data globalIPBlacklistResponse `json:"data"`
		}
		require.NoError(t, json.Unmarshal(res.Body.Bytes(), &result))
		require.NotNil(t, result.Data.GlobalIPBlacklistSettings)
		return result.Data
	}
	stale := read()
	require.Regexp(t, "^[a-f0-9]{64}$", stale.Version)
	for _, suffix := range []string{"", `,"version":null`, `,"version":""`, `,"version":"short"`, `,"version":"` + strings.Repeat("z", 64) + `"`} {
		res := request(http.MethodPut, `{"enabled":false,"rules":[]`+suffix+`}`)
		require.Equal(t, http.StatusBadRequest, res.Code, res.Body.String())
		require.Empty(t, repo.values)
	}
	// Simulate another operation committing a new ban after this page loaded.
	latest := `{"enabled":true,"rules":["203.0.113.10"]}`
	repo.values[service.SettingKeyGlobalIPBlacklist] = latest
	res := request(http.MethodPut, `{"enabled":false,"rules":[],"version":"`+stale.Version+`"}`)
	require.Equal(t, http.StatusConflict, res.Code, res.Body.String())
	var failure struct {
		Reason string `json:"reason"`
	}
	require.NoError(t, json.Unmarshal(res.Body.Bytes(), &failure))
	require.Equal(t, "IP_BLACKLIST_VERSION_CONFLICT", failure.Reason)
	require.Equal(t, latest, repo.values[service.SettingKeyGlobalIPBlacklist])
	fresh := read()
	require.True(t, fresh.Enabled)
	require.Equal(t, []string{"203.0.113.10"}, fresh.Rules)
	require.NotEqual(t, stale.Version, fresh.Version)
	res = request(http.MethodPut, `{"enabled":false,"rules":[],"version":"`+fresh.Version+`"}`)
	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
}
