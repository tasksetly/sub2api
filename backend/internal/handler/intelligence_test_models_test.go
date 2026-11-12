package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/handler/admin"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

type intelligenceModelsKeyRepo struct {
	service.APIKeyRepository
	key *service.APIKey
}

func (r *intelligenceModelsKeyRepo) GetByID(context.Context, int64) (*service.APIKey, error) {
	return r.key, nil
}

func TestIntelligenceModelsUseSelectedKeyGroupAndAllowlist(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(31)
	key := &service.APIKey{
		ID: 9, Key: "private-key-must-not-leak", Status: service.StatusActive, GroupID: &groupID,
		Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI,
			ModelAllowlist: service.GroupModelAllowlist{Enabled: true, Models: []string{"allowed-model"}}},
	}
	svc := service.NewIntelligenceTestService(nil, &intelligenceModelsKeyRepo{key: key}, nil, nil, nil, nil)
	adminHandler := admin.NewIntelligenceTestHandler(svc)
	gateway := newGatewayModelsHandlerForTest(&gatewayModelsAccountRepoStub{byGroup: map[int64][]service.Account{
		groupID: {{ID: 1, Platform: service.PlatformOpenAI, Credentials: map[string]any{
			"model_mapping": map[string]any{"allowed-model": "upstream", "hidden-model": "upstream"},
		}}},
		32: {{ID: 2, Platform: service.PlatformOpenAI, Credentials: map[string]any{
			"model_mapping": map[string]any{"other-group-model": "upstream"},
		}}},
	}})
	router := gin.New()
	router.GET("/api-keys/:id/models", adminHandler.ModelListKey, gateway.Models)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/api-keys/9/models", nil))
	require.Equal(t, http.StatusOK, recorder.Code)
	var result gatewayModelsResponseForTest
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &result))
	require.Equal(t, []string{"allowed-model"}, modelIDsForTest(result.Data))
	require.NotContains(t, recorder.Body.String(), key.Key)
}

func TestIntelligenceModelsRejectInvalidKeysBeforeGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	groupID := int64(31)
	expired := time.Now().Add(-time.Hour)
	for name, key := range map[string]*service.APIKey{
		"missing":       nil,
		"disabled":      {Status: service.StatusAPIKeyDisabled, GroupID: &groupID, Group: &service.Group{ID: groupID}},
		"expired":       {Status: service.StatusActive, ExpiresAt: &expired, GroupID: &groupID, Group: &service.Group{ID: groupID}},
		"unbound":       {Status: service.StatusActive},
		"missing group": {Status: service.StatusActive, GroupID: &groupID},
	} {
		t.Run(name, func(t *testing.T) {
			svc := service.NewIntelligenceTestService(nil, &intelligenceModelsKeyRepo{key: key}, nil, nil, nil, nil)
			h := admin.NewIntelligenceTestHandler(svc)
			router := gin.New()
			called := false
			router.GET("/api-keys/:id/models", h.ModelListKey, func(c *gin.Context) { called = true })
			for _, path := range []string{"/api-keys/9/models", "/api-keys/0/models", "/api-keys/invalid/models"} {
				recorder := httptest.NewRecorder()
				router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusBadRequest, recorder.Code)
				require.False(t, called)
			}
		})
	}
}
