//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// upstreamEnvelopeJSON 包装成上游 response.Success 的格式。
func upstreamEnvelopeJSON(t *testing.T, data any) string {
	t.Helper()
	encoded, err := json.Marshal(map[string]any{
		"code":    0,
		"message": "success",
		"data":    data,
	})
	require.NoError(t, err)
	return string(encoded)
}

func TestUpstreamClientLoginReturnsToken(t *testing.T) {
	var gotPath, gotBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		gotBody = string(buf)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"access_token":  "jwt-token",
			"refresh_token": "refresh-token",
			"expires_in":    3600,
			"token_type":    "Bearer",
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	result, err := client.Login(context.Background(), server.URL, "admin@example.com", "secret")
	require.NoError(t, err)
	require.Equal(t, "jwt-token", result.AccessToken)
	require.Equal(t, "refresh-token", result.RefreshToken)
	require.False(t, result.Requires2FA)
	require.Equal(t, "/api/v1/auth/login", gotPath)
	require.Contains(t, gotBody, "admin@example.com")
	// expires_in 应换算成绝对过期时间
	require.WithinDuration(t, time.Now().Add(time.Hour), result.ExpiresAt, time.Minute)
}

func TestUpstreamClientLoginDetects2FA(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"requires_2fa": true,
			"temp_token":   "temp-abc",
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	result, err := client.Login(context.Background(), server.URL, "a@b.c", "pw")
	require.NoError(t, err)
	require.True(t, result.Requires2FA)
	require.Equal(t, "temp-abc", result.TempToken)
	require.Empty(t, result.AccessToken)
}

// 验证码必须能被单独识别出来：它意味着自动登录彻底不可行，
// 和「密码错了」是完全不同的处置方式。
func TestUpstreamClientLoginClassifiesCaptcha(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"code":40001,"message":"captcha verification failed"}`))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	_, err := client.Login(context.Background(), server.URL, "a@b.c", "pw")
	require.ErrorIs(t, err, ErrUpstreamProviderCaptchaRequired)
}

func TestUpstreamClientLoginClassifiesTotp(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":40101,"message":"totp code required"}`))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	_, err := client.Login(context.Background(), server.URL, "a@b.c", "pw")
	require.ErrorIs(t, err, ErrUpstreamProviderTotpRequired)
}

func TestUpstreamClientLoginClassifiesBadPassword(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":40100,"message":"invalid email or password"}`))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	_, err := client.Login(context.Background(), server.URL, "a@b.c", "wrong")
	require.ErrorIs(t, err, ErrUpstreamProviderUnauthorized)
}

func TestUpstreamClientRefreshToken(t *testing.T) {
	var gotBody map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/auth/refresh", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"access_token":  "new-jwt-token",
			"refresh_token": "new-refresh-token",
			"expires_in":    7200,
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	result, err := client.RefreshToken(context.Background(), server.URL, "old-refresh-token")
	require.NoError(t, err)
	require.Equal(t, "old-refresh-token", gotBody["refresh_token"])
	require.Equal(t, "new-jwt-token", result.AccessToken)
	require.Equal(t, "new-refresh-token", result.RefreshToken)
	require.WithinDuration(t, time.Now().Add(2*time.Hour), result.ExpiresAt, time.Minute)
}

func TestUpstreamClientGetProfile(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/user/profile", r.URL.Path)
		require.Equal(t, "Bearer jwt-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"id":             42,
			"email":          "admin@example.com",
			"balance":        123.45,
			"frozen_balance": 6.78,
			"concurrency":    16,
			"status":         "active",
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	profile, err := client.GetProfile(context.Background(), server.URL, "jwt-token")
	require.NoError(t, err)
	require.Equal(t, int64(42), profile.UserID)
	require.InDelta(t, 123.45, profile.Balance, 0.0001)
	require.InDelta(t, 6.78, profile.FrozenBalance, 0.0001)
	require.Equal(t, 16, profile.Concurrency)
}

func TestUpstreamClientListGroups(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/groups/available", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, []map[string]any{
			{
				"id":                   7,
				"name":                 "claude-standard",
				"platform":             "anthropic",
				"subscription_type":    "standard",
				"rate_multiplier":      0.15,
				"peak_rate_enabled":    true,
				"peak_start":           "14:00",
				"peak_end":             "22:00",
				"peak_rate_multiplier": 1.5,
				"daily_limit_usd":      20.0,
			},
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	groups, err := client.ListGroups(context.Background(), server.URL, "jwt-token")
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, int64(7), groups[0].ID)
	require.InDelta(t, 0.15, groups[0].RateMultiplier, 0.0001)
	require.True(t, groups[0].PeakRateEnabled)
	require.NotNil(t, groups[0].DailyLimitUSD)
	require.InDelta(t, 20.0, *groups[0].DailyLimitUSD, 0.0001)
}

// 老版本上游没有 /groups/rates，404 不该让整次同步失败。
func TestUpstreamClientGetGroupRatesTolerates404(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	rates, err := client.GetGroupRates(context.Background(), server.URL, "jwt-token")
	require.NoError(t, err)
	require.Empty(t, rates)
}

// 上游返回 map[int64]float64，JSON 里 key 是字符串，需要转回来。
func TestUpstreamClientGetGroupRatesParsesStringKeys(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]float64{
			"7":  0.12,
			"11": 0.2,
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	rates, err := client.GetGroupRates(context.Background(), server.URL, "jwt-token")
	require.NoError(t, err)
	require.InDelta(t, 0.12, rates[7], 0.0001)
	require.InDelta(t, 0.2, rates[11], 0.0001)
}

// rates 返回 null 或非预期结构时按「无覆盖」处理，不阻断同步。
func TestUpstreamClientGetGroupRatesToleratesNull(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":0,"message":"success","data":null}`))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	rates, err := client.GetGroupRates(context.Background(), server.URL, "jwt-token")
	require.NoError(t, err)
	require.Empty(t, rates)
}

func TestUpstreamClientCreateAPIKey(t *testing.T) {
	var gotBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/keys", r.URL.Path)
		require.Equal(t, http.MethodPost, r.Method)
		require.NoError(t, json.NewDecoder(r.Body).Decode(&gotBody))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"id":       99,
			"key":      "sk-upstream-xxx",
			"name":     "prov-claude",
			"group_id": 7,
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	created, err := client.CreateAPIKey(context.Background(), server.URL, "jwt-token", "prov-claude", 7)
	require.NoError(t, err)
	require.Equal(t, "sk-upstream-xxx", created.Key)
	require.Equal(t, "prov-claude", gotBody["name"])
	require.EqualValues(t, 7, gotBody["group_id"])
}

// 上游返回空 key 时必须报错：否则会建出一个不可用的本地账号。
func TestUpstreamClientCreateAPIKeyRejectsEmptyKey(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"id":  99,
			"key": "",
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	_, err := client.CreateAPIKey(context.Background(), server.URL, "jwt-token", "n", 7)
	require.Error(t, err)
}

func TestUpstreamClientUnreachable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	baseURL := server.URL
	server.Close() // 立刻关掉，制造连不上的情况

	client := NewUpstreamProviderClient()
	_, err := client.GetProfile(context.Background(), baseURL, "jwt-token")
	require.ErrorIs(t, err, ErrUpstreamProviderUnreachable)
}

func TestUpstreamProviderHasValidToken(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	soon := now.Add(time.Minute) // 在 5 分钟安全边界内

	tests := []struct {
		name     string
		provider UpstreamProvider
		want     bool
	}{
		{"newapi relay API key", UpstreamProvider{ProviderType: UpstreamProviderTypeNewAPI, Token: "sk-personal"}, true},
		{"no token", UpstreamProvider{TokenExpiresAt: &future}, false},
		{"no expiry", UpstreamProvider{Token: "t"}, false},
		{"valid", UpstreamProvider{Token: "t", TokenExpiresAt: &future}, true},
		// 边界内视为无效，避免拿着刚好失效的 token 发请求
		{"within skew", UpstreamProvider{Token: "t", TokenExpiresAt: &soon}, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, tc.provider.HasValidToken(now))
		})
	}
}

// 比价用的倍率：有专属倍率优先，否则回退基础倍率。
func TestUpstreamGroupComparableRate(t *testing.T) {
	exclusive := 0.08
	withOverride := UpstreamGroup{RateMultiplier: 0.15, EffectiveRateMultiplier: &exclusive}
	require.InDelta(t, 0.08, withOverride.ComparableRate(), 0.0001)

	withoutOverride := UpstreamGroup{RateMultiplier: 0.15}
	require.InDelta(t, 0.15, withoutOverride.ComparableRate(), 0.0001)
}

// 手填 token 的到期时间：能解出 exp 就用它，否则给兜底有效期。
func TestUpstreamTokenExpiry(t *testing.T) {
	// 只解不验签，所以签名内容无所谓，用随便一个 key 签出来即可
	signed := func(t *testing.T, claims jwt.MapClaims) string {
		t.Helper()
		token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
			SignedString([]byte("irrelevant-key"))
		require.NoError(t, err)
		return token
	}

	t.Run("uses exp from jwt", func(t *testing.T) {
		exp := time.Now().Add(48 * time.Hour).Truncate(time.Second)
		got := upstreamTokenExpiry(signed(t, jwt.MapClaims{"exp": exp.Unix()}))
		require.WithinDuration(t, exp, got, time.Second)
	})

	// 已过期的 token 照原样落库，让 ensureToken 报 TOKEN_EXPIRED，
	// 而不是套上兜底有效期假装还能用
	t.Run("keeps past exp", func(t *testing.T) {
		exp := time.Now().Add(-time.Hour).Truncate(time.Second)
		got := upstreamTokenExpiry(signed(t, jwt.MapClaims{"exp": exp.Unix()}))
		require.WithinDuration(t, exp, got, time.Second)
		require.True(t, got.Before(time.Now()))
	})

	t.Run("falls back without exp", func(t *testing.T) {
		got := upstreamTokenExpiry(signed(t, jwt.MapClaims{"sub": "42"}))
		require.WithinDuration(t, time.Now().Add(upstreamProviderManualTokenTTL), got, time.Minute)
	})

	// 不透明 token 也接受：上游不一定发 JWT，给兜底有效期而不是直接拒
	t.Run("falls back for opaque token", func(t *testing.T) {
		got := upstreamTokenExpiry("not-a-jwt-at-all")
		require.WithinDuration(t, time.Now().Add(upstreamProviderManualTokenTTL), got, time.Minute)
	})
}

func TestStableNewAPIGroupIDIsDeterministicAndJavaScriptSafe(t *testing.T) {
	first := stableNewAPIGroupID("claude-sonnet")
	require.Positive(t, first)
	require.Less(t, first, int64(1<<53))
	require.Equal(t, first, stableNewAPIGroupID("claude-sonnet"))
	require.NotEqual(t, first, stableNewAPIGroupID("gpt-4o"))
}

func TestUpstreamClientGetNewAPIProfileUsesTokenUsage(t *testing.T) {
	var paths []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		require.Equal(t, "Bearer sk-ordinary", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":true,"message":"ok","data":{"total_available":1000000,"user_quota":{"total_available":3000000}}}`))
	}))
	defer server.Close()

	profile, err := NewUpstreamProviderClient().GetNewAPIProfile(context.Background(), server.URL, "sk-ordinary")
	require.NoError(t, err)
	require.Equal(t, 6.0, profile.Balance)
	require.True(t, profile.BalanceKnown)
	require.Equal(t, []string{"/api/usage/token/"}, paths)
}

func TestUpstreamClientListNewAPIGroupsPreservesNamesAndOmitsAuto(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/groups", r.URL.Path)
		require.Empty(t, r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"auto":{"ratio":"auto","desc":"automatic"},"vip":{"ratio":1.5,"desc":"VIP"}}}`))
	}))
	defer server.Close()

	groups, err := NewUpstreamProviderClient().ListNewAPIGroups(context.Background(), server.URL, "sk-ordinary")
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "vip", groups[0].Name)
	require.Equal(t, 1.5, groups[0].Ratio)
}

func TestUpstreamClientListNewAPIGroupsUsesDashboardPAT(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/user/self/groups", r.URL.Path)
		require.Equal(t, "Bearer nap-test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"data":{"vip":{"ratio":0.75,"desc":"VIP"}}}`))
	}))
	defer server.Close()

	groups, err := NewUpstreamProviderClient().ListNewAPIGroups(context.Background(), server.URL, "nap-test-token")
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "vip", groups[0].Name)
	require.Equal(t, 0.75, groups[0].Ratio)
}

func TestUpstreamClientGetNewAPIProfileUsesDashboardPAT(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		require.Equal(t, "Bearer nap-test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"id":42,"username":"alice","email":"alice@example.com","quota":1500000,"status":1}}`))
	}))
	defer server.Close()

	profile, err := NewUpstreamProviderClient().GetNewAPIProfile(context.Background(), server.URL, "nap-test-token")
	require.NoError(t, err)
	require.Equal(t, "/api/user/self", gotPath)
	require.Equal(t, int64(42), profile.UserID)
	require.Equal(t, "alice@example.com", profile.Email)
	require.Equal(t, 3.0, profile.Balance)
	require.True(t, profile.BalanceKnown)
}

func TestUpstreamClientCreateNewAPIRelayKey(t *testing.T) {
	var createdName string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer nap-test-token", r.Header.Get("Authorization"))
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/":
			var payload struct {
				Name  string `json:"name"`
				Group string `json:"group"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
			require.Equal(t, "vip", payload.Group)
			createdName = payload.Name
			_, _ = w.Write([]byte(`{"success":true,"message":""}`))
		case r.Method == http.MethodGet && r.URL.Path == "/api/token/":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"items":[{"id":7,"name":"` + createdName + `"}]}}`))
		case r.Method == http.MethodPost && r.URL.Path == "/api/token/7/key":
			_, _ = w.Write([]byte(`{"success":true,"message":"","data":{"key":"sk-created-relay-key"}}`))
		default:
			t.Fatalf("unexpected NewAPI request: %s %s", r.Method, r.URL.RequestURI())
		}
	}))
	defer server.Close()

	created, err := NewUpstreamProviderClient().CreateNewAPIRelayKey(
		context.Background(), server.URL, "nap-test-token", "provider-vip", "vip",
	)
	require.NoError(t, err)
	require.Equal(t, "sk-created-relay-key", created.Key)
	require.Equal(t, "vip", created.Group)
}
