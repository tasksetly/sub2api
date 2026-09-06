package service

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOpenAIModelUnavailablePassthroughForward(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, accountType := range []string{AccountTypeAPIKey, AccountTypeOAuth, AccountTypeSetupToken} {
		t.Run(accountType, func(t *testing.T) {
			body := []byte(`{"model":"gpt-5.1","stream":false,"input":"hello"}`)
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
			c.Request.Header.Set("Content-Type", "application/json")
			account := &Account{
				ID: 5101, Platform: PlatformOpenAI, Type: accountType, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"api_key": "sk-test", "access_token": "test-token"},
				Extra:       map[string]any{"openai_passthrough": true},
			}
			repo := &tempUnschedulableOpenAIAccountRepo{}
			upstream := &httpUpstreamRecorder{resp: &http.Response{
				StatusCode: http.StatusBadRequest,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(bytes.NewReader([]byte(`{"error":{"code":"model_not_found","message":"unknown provider for model gpt-5.1"}}`))),
			}}
			svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: upstream, rateLimitService: &RateLimitService{accountRepo: repo}}

			result, err := svc.Forward(context.Background(), c, account, body)

			require.Nil(t, result)
			var failover *UpstreamFailoverError
			require.ErrorAs(t, err, &failover)
			require.True(t, failover.IsOpenAIModelUnavailable())
			require.True(t, failover.ShouldRetryNextAccount())
			require.False(t, failover.RetryableOnSameAccount)
			require.False(t, c.Writer.Written(), "handler must be able to select another account before responding")
			require.Len(t, upstream.requests, 1)
			require.Equal(t, account.ID, repo.modelRateLimitAccountID)
			require.Equal(t, "gpt-5.1", repo.modelRateLimitKey)
		})
	}
}

func TestOpenAIModelUnavailableCooldownUsesFinalModel(t *testing.T) {
	for _, rawModel := range []bool{false, true} {
		name := "forwarded canonical model"
		if rawModel {
			name = "unmapped request model"
		}
		t.Run(name, func(t *testing.T) {
			repo := &tempUnschedulableOpenAIAccountRepo{}
			rateLimits := &RateLimitService{accountRepo: repo}
			svc := &OpenAIGatewayService{rateLimitService: rateLimits}
			account := &Account{
				ID: 5102, Platform: PlatformOpenAI, Type: AccountTypeAPIKey, Status: StatusActive, Schedulable: true,
				Credentials: map[string]any{"model_mapping": map[string]any{"public-a": "upstream-a", "upstream-a": "upstream-b"}},
			}
			body := []byte(`{"error":{"code":"model_not_found","message":"unknown provider for model upstream-a"}}`)
			if rawModel {
				require.True(t, rateLimits.HandleUpstreamModelNotFound(context.Background(), account, "public-a", http.StatusBadRequest, body))
			} else {
				_, upstreamModel := resolveOpenAIForwardMappedModels(account, "public-a", false)
				require.Equal(t, "upstream-a", upstreamModel)
				failover := svc.failoverOpenAIUpstreamHTTPError(context.Background(), nil, account, &http.Response{StatusCode: http.StatusBadRequest}, body, "unknown provider for model upstream-a", upstreamModel)
				require.NotNil(t, failover)
				require.True(t, failover.ShouldRetryNextAccount())
			}
			require.Equal(t, "upstream-a", repo.modelRateLimitKey)
			account.Extra = map[string]any{modelRateLimitsKey: map[string]any{
				repo.modelRateLimitKey: map[string]any{"rate_limit_reset_at": time.Now().Add(30 * time.Minute).UTC().Format(time.RFC3339)},
			}}
			require.False(t, account.IsSchedulableForModel("public-a"), "failed model must be skipped on subsequent requests")
			require.True(t, account.IsSchedulableForModel("upstream-a"), "the other upstream model must remain available")
		})
	}
}
