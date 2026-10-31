//go:build unit

package service

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/pkg/openai_compat"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestIntelligenceTestUsesNativeOpenAIProtocol(t *testing.T) {
	for _, tc := range []struct {
		name       string
		mode       string
		path       string
		response   string
		statusCode int
	}{
		{"responses", "force_responses", "/v1/responses", "data: {\"type\":\"response.output_text.delta\",\"delta\":\"<svg/>\"}\n\ndata: {\"type\":\"response.completed\"}\n\n", http.StatusOK},
		{"chat", "force_chat_completions", "/v1/chat/completions", "data: {\"choices\":[{\"delta\":{\"content\":\"<svg/>\"},\"finish_reason\":\"stop\"}]}\n\ndata: [DONE]\n\n", http.StatusOK},
		{"forbidden", "force_responses", "/v1/responses", `{"error":{"message":"denied"}}`, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			groupID := int64(32)
			account := &Account{
				ID: 285, Platform: PlatformOpenAI, Type: AccountTypeAPIKey,
				Status: StatusActive, Schedulable: true, Concurrency: 1,
				Credentials: map[string]any{"api_key": "test-key", "base_url": "https://upstream.example"},
				Extra: map[string]any{openai_compat.ExtraKeyResponsesMode: tc.mode},
			}
			accountTest, upstream := adaptiveCNAccountTestService(account, newJSONResponse(tc.statusCode, tc.response))
			gateway := &GatewayService{
				accountRepo: &mockAccountRepoForPlatform{accounts: []Account{*account}, accountsByID: map[int64]*Account{account.ID: account}},
				groupRepo: &mockGroupRepoForGateway{groups: map[int64]*Group{groupID: {
					ID: groupID, Platform: PlatformOpenAI, Status: StatusActive, Hydrated: true,
				}}},
				cfg: testConfig(),
			}
			svc := NewIntelligenceTestService(nil, nil, nil, accountTest, gateway, nil)
			result, err := svc.runThroughGateway(context.Background(), &APIKey{GroupID: &groupID}, &IntelligenceTestConfig{
				ModelID: "test-model", PromptText: "Draw a bird", ReasoningEffort: "high",
			})
			require.NoError(t, err)
			require.Len(t, upstream.requests, 1)
			req := upstream.requests[0]
			require.Equal(t, tc.path, req.URL.Path)
			body, err := io.ReadAll(req.Body)
			require.NoError(t, err)
			require.Contains(t, string(body), intelligenceTestSVGSystemPrompt)
			require.Contains(t, string(body), "Draw a bird")
			if strings.Contains(tc.path, "/chat/") {
				require.Equal(t, "high", gjson.GetBytes(body, "reasoning_effort").String())
			} else {
				require.Equal(t, "high", gjson.GetBytes(body, "reasoning.effort").String())
			}
			if tc.statusCode == http.StatusOK {
				require.Equal(t, "success", result.Status)
				require.Equal(t, "<svg/>", result.ResponseText)
			} else {
				require.Equal(t, "failed", result.Status)
				require.Equal(t, tc.statusCode, result.ErrorCode)
			}
		})
	}
}
