package service

import (
	"strings"
	"testing"

	lmfpd "github.com/Ikaleio/lm-detector/sdk/go"
)

func fingerprintTestAccount(accountType string, creds map[string]any) *Account {
	if creds == nil {
		creds = map[string]any{}
	}
	if _, ok := creds["base_url"]; !ok {
		creds["base_url"] = "https://api.example.com/v1"
	}
	if _, ok := creds["api_key"]; !ok {
		creds["api_key"] = "sk-secret-value-do-not-leak"
	}
	return &Account{
		ID:          1,
		Type:        accountType,
		Platform:    PlatformOpenAI,
		Status:      StatusActive,
		Credentials: creds,
	}
}

func TestFingerprintEligibilityByAccountType(t *testing.T) {
	for _, tt := range []struct {
		name string
		typ  string
		want bool
	}{
		{"upstream 支持", AccountTypeUpstream, true},
		{"apikey 支持", AccountTypeAPIKey, true},
		{"oauth 不支持", AccountTypeOAuth, false},
		{"setup-token 不支持", AccountTypeSetupToken, false},
		{"bedrock 不支持", AccountTypeBedrock, false},
		{"service_account 不支持", AccountTypeServiceAccount, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckFingerprintEligibility(fingerprintTestAccount(tt.typ, nil))
			if got.Supported != tt.want {
				t.Fatalf("Supported = %v, want %v (reason %q)", got.Supported, tt.want, got.Reason)
			}
			if !tt.want && got.Reason == "" {
				t.Fatal("不支持时必须给出可展示的原因，否则界面上像是功能坏了")
			}
		})
	}
}

func TestFingerprintEligibilityRejectsHeaderOverrides(t *testing.T) {
	acc := fingerprintTestAccount(AccountTypeUpstream, map[string]any{
		"header_override_enabled": true,
		"header_overrides":        map[string]any{"X-Tenant": "t-1"},
	})
	got := CheckFingerprintEligibility(acc)
	if got.Supported {
		t.Fatal("配了自定义请求头的账号，SDK 自建请求大概率失败，应判为不支持")
	}
	if got.Reason != FingerprintReasonHeaderOverrides {
		t.Fatalf("Reason = %q, want %q", got.Reason, FingerprintReasonHeaderOverrides)
	}
}

func TestFingerprintEligibilityFlagsProxyBypass(t *testing.T) {
	proxyID := int64(7)
	acc := fingerprintTestAccount(AccountTypeUpstream, nil)
	acc.ProxyID = &proxyID

	got := CheckFingerprintEligibility(acc)
	if !got.Supported {
		t.Fatalf("配了代理不应导致不支持，只是结论要加提示；reason=%q", got.Reason)
	}
	if !got.ProxyBypassed {
		t.Fatal("SDK 的 HTTP 客户端不可注入，配了代理必须标记 ProxyBypassed 以便界面提示")
	}
}

func TestFingerprintEligibilityRequiresCredentials(t *testing.T) {
	for _, tt := range []struct {
		name       string
		creds      map[string]any
		wantReason string
	}{
		{"缺 base_url", map[string]any{"base_url": "", "api_key": "sk-x"}, FingerprintReasonMissingBaseURL},
		{"缺 api_key", map[string]any{"base_url": "https://api.example.com/v1", "api_key": ""}, FingerprintReasonMissingAPIKey},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CheckFingerprintEligibility(fingerprintTestAccount(AccountTypeUpstream, tt.creds))
			if got.Supported {
				t.Fatal("凭证不全时应判为不支持")
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
		})
	}
}

// Review Focus #2：base_url 带 query / fragment / 内嵌凭证时，SDK 在发请求前就会拒绝。
// 必须在建计划时拦住，而不是等到抽查时才炸。
func TestBuildDetectConfigRejectsUnusableBaseURL(t *testing.T) {
	const secret = "sk-secret-value-do-not-leak"
	for _, raw := range []string{
		"https://api.example.com/v1?key=abc",
		"https://user:pw@api.example.com/v1",
		"https://api.example.com/v1#frag",
		"ftp://api.example.com/v1",
		"not-a-url",
		"",
	} {
		acc := fingerprintTestAccount(AccountTypeUpstream, map[string]any{"base_url": raw, "api_key": secret})
		plan := DefaultFingerprintPlan()
		plan.TestModel = "claude-opus-5"

		_, err := BuildDetectConfig(acc, plan)
		if err == nil {
			t.Fatalf("base_url %q 应被拒绝", raw)
		}
		if strings.Contains(err.Error(), secret) {
			t.Fatalf("错误信息泄露了 API Key: %v", err)
		}
	}
}

func TestBuildDetectConfigMapsPlanToSDKConfig(t *testing.T) {
	acc := fingerprintTestAccount(AccountTypeUpstream, nil)
	plan := DefaultFingerprintPlan()
	plan.TestModel = "claude-opus-5"
	plan.API = string(FingerprintAPIMessages)
	plan.ChallengeCount = 2

	cfg, err := BuildDetectConfig(acc, plan)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.BaseURL != "https://api.example.com/v1" {
		t.Fatalf("BaseURL = %q", cfg.BaseURL)
	}
	if cfg.APIKey != "sk-secret-value-do-not-leak" {
		t.Fatalf("APIKey 未正确映射")
	}
	if cfg.Model != "claude-opus-5" {
		t.Fatalf("Model = %q, want 计划里的测试模型", cfg.Model)
	}
	if cfg.API != lmfpd.APIMessages {
		t.Fatalf("API = %q, want %q", cfg.API, lmfpd.APIMessages)
	}
	if cfg.Count != 2 {
		t.Fatalf("Count = %d, want 2", cfg.Count)
	}
	if cfg.Parallel > cfg.Count {
		t.Fatalf("Parallel %d 不得超过 Count %d", cfg.Parallel, cfg.Count)
	}
}

func TestBuildDetectConfigClampsChallengeCount(t *testing.T) {
	acc := fingerprintTestAccount(AccountTypeUpstream, nil)
	for _, tt := range []struct{ in, want int }{
		{0, FingerprintDefaultChallengeCount},
		{-1, FingerprintDefaultChallengeCount},
		{1, 1},
		{3, 3},
		{9, FingerprintDefaultChallengeCount},
	} {
		plan := DefaultFingerprintPlan()
		plan.TestModel = "claude-opus-5"
		plan.ChallengeCount = tt.in
		cfg, err := BuildDetectConfig(acc, plan)
		if err != nil {
			t.Fatalf("challenge count %d: unexpected error %v", tt.in, err)
		}
		if cfg.Count != tt.want {
			t.Fatalf("challenge count %d → Count = %d, want %d", tt.in, cfg.Count, tt.want)
		}
	}
}

func TestBuildDetectConfigRequiresTestModel(t *testing.T) {
	acc := fingerprintTestAccount(AccountTypeUpstream, nil)
	plan := DefaultFingerprintPlan()
	plan.TestModel = "   "
	if _, err := BuildDetectConfig(acc, plan); err == nil {
		t.Fatal("测试模型为空时应报错")
	}
}

func TestResolveFingerprintAPIFallsBackToAccountProtocol(t *testing.T) {
	for _, tt := range []struct {
		name     string
		protocol string
		want     lmfpd.API
	}{
		{"anthropic 协议走 messages", APIProtocolAnthropic, lmfpd.APIMessages},
		{"responses 协议走 responses", APIProtocolResponses, lmfpd.APIResponses},
		{"chat_completions 协议走 chat_completions", APIProtocolChatCompletions, lmfpd.APIChatCompletions},
		{"未知协议回落 chat_completions", "something-else", lmfpd.APIChatCompletions},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveFingerprintAPI("", tt.protocol); got != tt.want {
				t.Fatalf("resolveFingerprintAPI(\"\", %q) = %q, want %q", tt.protocol, got, tt.want)
			}
		})
	}
}
