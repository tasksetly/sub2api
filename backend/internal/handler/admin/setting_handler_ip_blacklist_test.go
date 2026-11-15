package admin

import (
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
	router := gin.New()
	require.NoError(t, router.SetTrustedProxies(nil))
	router.PUT("/settings", h.UpdateGlobalIPBlacklistSettings)
	for _, body := range []string{
		`{}`, `{"enabled":false}`, `{"rules":[]}`, `{"enabled":null,"rules":[]}`,
		`{"enabled":true,"rules":null}`, `{"enabled":true,"rules":["example.com"]}`,
		`{"enabled":true,"rules":["203.0.113.0/24"]}`,
	} {
		t.Run(body, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(body))
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
	request := httptest.NewRequest(http.MethodPut, "/settings", strings.NewReader(`{"enabled":false,"rules":[]}`))
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
}
