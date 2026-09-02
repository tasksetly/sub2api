//go:build unit

package service

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/require"
)

// fakeUpstreamProviderRepo 只实现手填 token 这条路径需要的方法，
// 其余方法留空以满足接口。
type fakeUpstreamProviderRepo struct {
	stored                *UpstreamProvider
	created               *UpstreamProvider
	updated               *UpstreamProvider
	sessionToken          string
	sessionRefreshToken   string
	sessionTokenExpiresAt time.Time
}

func (f *fakeUpstreamProviderRepo) Create(_ context.Context, provider *UpstreamProvider) error {
	provider.ID = 1
	clone := *provider
	f.created = &clone
	return nil
}

func (f *fakeUpstreamProviderRepo) GetByID(_ context.Context, _ int64) (*UpstreamProvider, error) {
	if f.stored == nil {
		return nil, ErrUpstreamProviderNotFound
	}
	clone := *f.stored
	return &clone, nil
}

func (f *fakeUpstreamProviderRepo) Update(_ context.Context, provider *UpstreamProvider) error {
	clone := *provider
	f.updated = &clone
	return nil
}

func (f *fakeUpstreamProviderRepo) Delete(context.Context, int64) error { return nil }

func (f *fakeUpstreamProviderRepo) List(
	context.Context, pagination.PaginationParams, string, string,
) ([]UpstreamProviderWithStats, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func (f *fakeUpstreamProviderRepo) ListSyncable(context.Context) ([]UpstreamProvider, error) {
	return nil, nil
}

func (f *fakeUpstreamProviderRepo) ExistsByName(context.Context, string, int64) (bool, error) {
	return false, nil
}

func (f *fakeUpstreamProviderRepo) ListNamesByIDs(
	context.Context, []int64,
) (map[int64]string, error) {
	return map[int64]string{}, nil
}

func (f *fakeUpstreamProviderRepo) UpdateSession(
	_ context.Context, _ int64, token, refreshToken string, expiresAt time.Time,
) error {
	f.sessionToken = token
	f.sessionRefreshToken = refreshToken
	f.sessionTokenExpiresAt = expiresAt
	return nil
}

func (f *fakeUpstreamProviderRepo) UpdateSyncSnapshot(
	context.Context, int64, UpstreamSyncSnapshot,
) error {
	return nil
}

func (f *fakeUpstreamProviderRepo) MarkSyncFailed(context.Context, int64, string, time.Time) error {
	return nil
}

func (f *fakeUpstreamProviderRepo) ReplaceGroups(context.Context, int64, []UpstreamGroup) error {
	return nil
}

func (f *fakeUpstreamProviderRepo) ListGroups(context.Context, int64) ([]UpstreamGroup, error) {
	return nil, nil
}

func (f *fakeUpstreamProviderRepo) GetGroupByRemoteID(
	context.Context, int64, int64,
) (*UpstreamGroup, error) {
	return nil, ErrUpstreamGroupNotFound
}

func (f *fakeUpstreamProviderRepo) ListAllGroupsForComparison(
	context.Context, string, pagination.PaginationParams,
) ([]UpstreamGroupComparison, *pagination.PaginationResult, error) {
	return nil, nil, nil
}

func newManualTokenService(repo *fakeUpstreamProviderRepo) *UpstreamProviderService {
	// 关掉白名单校验，测试只关心 token 落库，不测 SSRF 那套
	cfg := &config.Config{}
	cfg.Security.URLAllowlist.Enabled = false
	return NewUpstreamProviderService(repo, nil, cfg)
}

func signedToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).
		SignedString([]byte("irrelevant-key"))
	require.NoError(t, err)
	return token
}

// 上游做了 CF 校验、账号密码登不上去时，只贴 token 也要能建上游。
func TestCreateUpstreamProviderAcceptsTokenWithoutPassword(t *testing.T) {
	repo := &fakeUpstreamProviderRepo{}
	svc := newManualTokenService(repo)

	exp := time.Now().Add(24 * time.Hour).Truncate(time.Second)
	token := signedToken(t, jwt.MapClaims{"exp": exp.Unix()})

	provider, err := svc.Create(context.Background(), CreateUpstreamProviderInput{
		Name:     "cf-blocked",
		BaseURL:  "https://upstream.example.com",
		Username: "admin@example.com",
		Token:    token,
	})
	require.NoError(t, err)
	require.Equal(t, token, provider.Token)
	require.NotNil(t, provider.TokenExpiresAt)
	// 有效期取自 JWT 的 exp，不是兜底值
	require.WithinDuration(t, exp, *provider.TokenExpiresAt, time.Second)

	require.NotNil(t, repo.created)
	require.Equal(t, token, repo.created.Token)
	require.Empty(t, repo.created.Password)
}

func TestCreateUpstreamProviderAcceptsRefreshTokenWithoutAccessToken(t *testing.T) {
	repo := &fakeUpstreamProviderRepo{}
	svc := newManualTokenService(repo)

	provider, err := svc.Create(context.Background(), CreateUpstreamProviderInput{
		Name:         "refresh-only",
		BaseURL:      "https://upstream.example.com",
		Username:     "admin@example.com",
		RefreshToken: "refresh-token",
	})
	require.NoError(t, err)
	require.Empty(t, provider.Token)
	require.Equal(t, "refresh-token", provider.RefreshToken)
	require.NotNil(t, repo.created)
	require.Equal(t, "refresh-token", repo.created.RefreshToken)
}

// 密码、access token 和 refresh token 都没给才算缺凭据。
func TestCreateUpstreamProviderRequiresCredentials(t *testing.T) {
	svc := newManualTokenService(&fakeUpstreamProviderRepo{})

	_, err := svc.Create(context.Background(), CreateUpstreamProviderInput{
		Name:     "no-creds",
		BaseURL:  "https://upstream.example.com",
		Username: "admin@example.com",
	})
	require.ErrorIs(t, err, ErrUpstreamProviderCredentialsRequired)
}

func TestUpdateUpstreamProviderManualToken(t *testing.T) {
	existingExpiry := time.Now().Add(12 * time.Hour)
	baseInput := UpdateUpstreamProviderInput{
		Name:     "cf-blocked",
		BaseURL:  "https://upstream.example.com",
		Username: "admin@example.com",
	}
	storedProvider := func() *UpstreamProvider {
		return &UpstreamProvider{
			ID:             1,
			Name:           "cf-blocked",
			BaseURL:        "https://upstream.example.com",
			Username:       "admin@example.com",
			Token:          "stored-token",
			RefreshToken:   "stored-refresh",
			TokenExpiresAt: &existingExpiry,
			Status:         StatusActive,
		}
	}

	// 贴新 token：顶掉缓存的会话，有效期重新按 exp 解析
	t.Run("pasted token replaces session", func(t *testing.T) {
		repo := &fakeUpstreamProviderRepo{stored: storedProvider()}
		svc := newManualTokenService(repo)

		exp := time.Now().Add(48 * time.Hour).Truncate(time.Second)
		fresh := signedToken(t, jwt.MapClaims{"exp": exp.Unix()})

		input := baseInput
		input.Token = fresh
		provider, err := svc.Update(context.Background(), 1, input)
		require.NoError(t, err)
		require.Equal(t, fresh, provider.Token)
		require.WithinDuration(t, exp, *provider.TokenExpiresAt, time.Second)
		require.Equal(t, fresh, repo.updated.Token)
		require.Empty(t, provider.RefreshToken)
		require.Empty(t, repo.updated.RefreshToken)
	})

	// 单独补填 refresh token 时保留原有 access token。
	t.Run("refresh token can be added separately", func(t *testing.T) {
		repo := &fakeUpstreamProviderRepo{stored: storedProvider()}
		svc := newManualTokenService(repo)

		input := baseInput
		input.RefreshToken = "new-refresh"
		provider, err := svc.Update(context.Background(), 1, input)
		require.NoError(t, err)
		require.Empty(t, repo.updated.Token, "repo 应收到空 token 表示不改 access 会话")
		require.Equal(t, "stored-token", provider.Token)
		require.Equal(t, "new-refresh", provider.RefreshToken)
		require.NotNil(t, provider.TokenExpiresAt)
	})

	// 没填 token 或 refresh token 时仓储层要收到空值（表示不改会话），
	// 但返回给前端的 provider 得保留存量凭据，否则 has_token/has_refresh_token 假报成 false
	t.Run("blank tokens keep session and still report it", func(t *testing.T) {
		repo := &fakeUpstreamProviderRepo{stored: storedProvider()}
		svc := newManualTokenService(repo)

		provider, err := svc.Update(context.Background(), 1, baseInput)
		require.NoError(t, err)
		require.Empty(t, repo.updated.Token, "repo 应收到空 token 表示不改会话")
		require.Equal(t, "stored-token", provider.Token)
		require.Equal(t, "stored-refresh", provider.RefreshToken)
		require.NotNil(t, provider.TokenExpiresAt)
	})

	// 同时填了密码和 token 以 token 为准：这类上游用密码重登只会失败，
	// 把刚贴进来的会话作废反而更糟
	t.Run("token wins over password", func(t *testing.T) {
		repo := &fakeUpstreamProviderRepo{stored: storedProvider()}
		svc := newManualTokenService(repo)

		fresh := signedToken(t, jwt.MapClaims{"exp": time.Now().Add(time.Hour).Unix()})
		input := baseInput
		input.Password = "new-password"
		input.Token = fresh

		provider, err := svc.Update(context.Background(), 1, input)
		require.NoError(t, err)
		require.Equal(t, fresh, provider.Token)
		require.Equal(t, fresh, repo.updated.Token)
		require.Equal(t, "new-password", repo.updated.Password)
	})

	// 只改密码：仓储层会作废缓存 token，响应里也不该再报有 token
	t.Run("password change clears reported token", func(t *testing.T) {
		repo := &fakeUpstreamProviderRepo{stored: storedProvider()}
		svc := newManualTokenService(repo)

		input := baseInput
		input.Password = "new-password"
		provider, err := svc.Update(context.Background(), 1, input)
		require.NoError(t, err)
		require.Empty(t, provider.Token)
		require.Nil(t, provider.TokenExpiresAt)
	})
}

// access JWT 进入过期边界后，应先用 refresh token 换发新 token 对。
func TestEnsureTokenRefreshesAccessTokenWithRefreshToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/v1/auth/refresh", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upstreamEnvelopeJSON(t, map[string]any{
			"access_token": "renewed-access",
			"expires_in":   3600,
			// 兼容旧上游：不返回 refresh_token 时应继续沿用旧值。
		})))
	}))
	defer server.Close()

	repo := &fakeUpstreamProviderRepo{}
	svc := newManualTokenService(repo)
	expired := time.Now().Add(-time.Minute)
	provider := &UpstreamProvider{
		ID:             1,
		BaseURL:        server.URL,
		Token:          "expired-access",
		RefreshToken:   "stable-refresh",
		TokenExpiresAt: &expired,
	}

	token, err := svc.ensureToken(context.Background(), provider)
	require.NoError(t, err)
	require.Equal(t, "renewed-access", token)
	require.Equal(t, "renewed-access", provider.Token)
	require.Equal(t, "stable-refresh", provider.RefreshToken)
	require.True(t, provider.TokenExpiresAt.After(time.Now()))
	require.Equal(t, "renewed-access", repo.sessionToken)
	require.Equal(t, "stable-refresh", repo.sessionRefreshToken)
	require.True(t, repo.sessionTokenExpiresAt.After(time.Now()))
}

// 只有 token 的上游过期后没有自动续期手段，报错要说清楚是「重贴 token」
// 而不是「补密码」——后者对这类上游是死路。
func TestEnsureTokenDistinguishesExpiredManualToken(t *testing.T) {
	svc := newManualTokenService(&fakeUpstreamProviderRepo{})
	expired := time.Now().Add(-time.Hour)

	_, err := svc.ensureToken(context.Background(), &UpstreamProvider{
		ID:             1,
		BaseURL:        "https://upstream.example.com",
		Token:          "expired-token",
		TokenExpiresAt: &expired,
	})
	require.ErrorIs(t, err, ErrUpstreamProviderTokenExpired)

	// 从没存过 token 且没密码，仍然是「缺凭据」
	_, err = svc.ensureToken(context.Background(), &UpstreamProvider{
		ID:      2,
		BaseURL: "https://upstream.example.com",
	})
	require.ErrorIs(t, err, ErrUpstreamProviderMissingCredentials)
}
