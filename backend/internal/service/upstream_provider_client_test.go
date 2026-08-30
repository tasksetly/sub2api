//go:build unit

package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
			"access_token": "jwt-token",
			"expires_in":   3600,
			"token_type":   "Bearer",
		})))
	}))
	defer server.Close()

	client := NewUpstreamProviderClient()
	result, err := client.Login(context.Background(), server.URL, "admin@example.com", "secret")
	require.NoError(t, err)
	require.Equal(t, "jwt-token", result.AccessToken)
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
