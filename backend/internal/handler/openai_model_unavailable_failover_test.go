package handler

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

type modelUnavailableFailoverUpstream struct {
	service.HTTPUpstream
	accountIDs []int64
	statusCode int
	allFail    bool
}

func (u *modelUnavailableFailoverUpstream) Do(_ *http.Request, _ string, accountID int64, _ int) (*http.Response, error) {
	u.accountIDs = append(u.accountIDs, accountID)
	status := u.statusCode
	body := `{"error":{"code":"model_not_found","message":"unknown provider for model gpt-5.2"}}`
	if status == 529 {
		body = `{"error":{"message":"server overloaded","type":"server_error"}}`
	}
	if accountID == 9911 && !u.allFail {
		status = http.StatusOK
		body = `{"id":"resp_healthy","object":"response","model":"gpt-5.2","status":"completed","usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`
	}
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}, nil
}

func TestOpenAIResponsesPassthroughModelUnavailableFailover(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tt := range []struct {
		name       string
		status     int
		allFail    bool
		wantStatus int
	}{
		{"model unavailable switches to healthy account", 400, false, 200},
		{"model unavailable exhausts accounts", 400, true, 400},
		{"529 switches to healthy account", 529, false, 200},
		{"529 exhausts accounts", 529, true, 503},
	} {
		t.Run(tt.name, func(t *testing.T) {
			groupID := int64(4203)
			accounts := make([]service.Account, 0, 2)
			for i, id := range []int64{9910, 9911} {
				accounts = append(accounts, service.Account{
					ID: id, Name: "test-account", Platform: service.PlatformOpenAI,
					Type: service.AccountTypeAPIKey, Status: service.StatusActive, Schedulable: true, Priority: i + 1,
					Credentials: map[string]any{"api_key": "sk-test", "base_url": "https://api.example.test"},
					Extra:       map[string]any{"openai_passthrough": true},
				})
			}
			cfg := &config.Config{RunMode: config.RunModeSimple}
			cfg.Default.RateMultiplier = 1
			cfg.Gateway.MaxAccountSwitches = 1
			upstream := &modelUnavailableFailoverUpstream{statusCode: tt.status, allFail: tt.allFail}
			billingCache := service.NewBillingCacheService(nil, nil, nil, nil, nil, nil, cfg, nil)
			t.Cleanup(billingCache.Stop)
			gateway := service.NewOpenAIGatewayService(
				&openAIWSFailoverHandlerAccountRepoStub{accounts: accounts},
				nil, nil, nil, nil, nil, nil, cfg, nil, nil,
				service.NewBillingService(cfg, nil), nil, billingCache, upstream, &service.DeferredService{},
				nil, nil, nil, nil, nil, nil, nil,
			)
			h := NewOpenAIGatewayHandler(gateway, service.NewConcurrencyService(nil), billingCache,
				service.NewAPIKeyService(nil, nil, nil, nil, nil, nil, cfg), nil, nil, nil, nil, cfg)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/openai/v1/responses", strings.NewReader(`{"model":"gpt-5.2","input":"hello","stream":false}`))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyAPIKey), &service.APIKey{
				ID: 1803, GroupID: &groupID,
				User:  &service.User{ID: 1703, Status: service.StatusActive},
				Group: &service.Group{ID: groupID, Platform: service.PlatformOpenAI, Status: service.StatusActive},
			})
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 1703, Concurrency: 0})

			h.Responses(c)

			require.Equal(t, []int64{9910, 9911}, upstream.accountIDs)
			require.Equal(t, tt.wantStatus, recorder.Code)
			if !tt.allFail {
				require.Equal(t, "resp_healthy", gjson.Get(recorder.Body.String(), "id").String())
			} else if tt.status == http.StatusBadRequest {
				require.Equal(t, service.OpenAIModelUnavailableCode, gjson.Get(recorder.Body.String(), "error.code").String())
			}
		})
	}
}
