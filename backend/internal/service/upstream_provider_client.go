package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/golang-jwt/jwt/v5"
)

// 上游 sub2api / NewAPI 管理客户端。
// NewAPI 使用用户自己生成的 API token（通常是 sk-）作为 Bearer token；
// 该 token 不是 JWT，也不支持账号密码登录或 refresh。
const (
	upstreamProviderRequestTimeout = 15 * time.Second
	upstreamProviderMaxBodyBytes   = 512 * 1024

	// token 提前过期的安全边界：避免边界时刻拿着刚好失效的 token 请求
	upstreamProviderTokenSkew = 5 * time.Minute
	// 上游未回传 expires_in 时的保守默认有效期
	upstreamProviderDefaultTokenTTL = 30 * time.Minute
	// 手填 token 解不出 exp 时的兜底有效期。比自动登录那份长得多：
	// 手填是因为自动登录走不通（CF 校验），到期后没有任何自愈手段，
	// 给太短只会让管理员反复贴。
	upstreamProviderManualTokenTTL = 7 * 24 * time.Hour
)

var (
	// ErrUpstreamProviderCaptchaRequired 表示上游登录要求验证码，
	// 自动登录无法继续，只能由管理员手动粘贴 token。
	ErrUpstreamProviderCaptchaRequired = infraerrors.BadRequest(
		"UPSTREAM_PROVIDER_CAPTCHA_REQUIRED",
		"upstream login requires a captcha; automatic login is not possible for this provider",
	)
	// ErrUpstreamProviderTotpRequired 表示上游开启了 2FA 但本地没存 TOTP 密钥。
	ErrUpstreamProviderTotpRequired = infraerrors.BadRequest(
		"UPSTREAM_PROVIDER_TOTP_REQUIRED",
		"upstream login requires 2FA; save the TOTP secret for this provider first",
	)
	ErrUpstreamProviderUnauthorized = infraerrors.BadRequest(
		"UPSTREAM_PROVIDER_UNAUTHORIZED", "upstream rejected the credentials",
	)
	ErrUpstreamProviderUnreachable = infraerrors.ServiceUnavailable(
		"UPSTREAM_PROVIDER_UNREACHABLE", "upstream sub2api instance is unreachable",
	)
)

// upstreamEnvelope 对应上游 response.Success 的统一包装：
// {"code":0,"message":"success","data":{...}}
type upstreamEnvelope struct {
	Code    json.RawMessage `json:"code"`
	Success *bool           `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// UpstreamLoginResult 是登录成功后拿到的会话信息。
type UpstreamLoginResult struct {
	AccessToken  string
	RefreshToken string
	ExpiresAt    time.Time
	// Requires2FA 为真时 AccessToken 为空，需要带 TOTP 码走第二步
	Requires2FA bool
	TempToken   string
}

// UpstreamProfile 是上游账户/额度查询返回的关键字段。
type UpstreamProfile struct {
	UserID           int64   `json:"id"`
	Email            string  `json:"email"`
	Username         string  `json:"username"`
	Balance          float64 `json:"balance"`
	FrozenBalance    float64 `json:"frozen_balance"`
	Concurrency      int     `json:"concurrency"`
	ConcurrencyKnown bool    `json:"-"`
	BalanceKnown     bool    `json:"-"`
	Status           string  `json:"status"`
}

// NewAPIGroupInfo 是 NewAPI 按名称返回的可用分组。NewAPI 没有 sub2api
// 那样的数字分组 ID，因此 Name 是真实的远程键。
type NewAPIGroupInfo struct {
	Name  string
	Ratio float64
	Desc  string
}

// NewAPI 的个人 API token 不是 JWT 登录会话，也不是 dashboard credential。
// NewAPIQuotaPerUnit 是 NewAPI 默认的额度单位：500000 quota = $1。
// 这是 NewAPI 的内部单位，不影响本地账号计费。
const NewAPIQuotaPerUnit = 500000.0

// UpstreamGroupInfo 是上游 GET /groups/available 的单个分组。
// 字段名与上游 dto.Group 对齐，多余字段忽略。
type UpstreamGroupInfo struct {
	ID                 int64    `json:"id"`
	Name               string   `json:"name"`
	Platform           string   `json:"platform"`
	SubscriptionType   string   `json:"subscription_type"`
	RateMultiplier     float64  `json:"rate_multiplier"`
	PeakRateEnabled    bool     `json:"peak_rate_enabled"`
	PeakStart          string   `json:"peak_start"`
	PeakEnd            string   `json:"peak_end"`
	PeakRateMultiplier float64  `json:"peak_rate_multiplier"`
	DailyLimitUSD      *float64 `json:"daily_limit_usd"`
	WeeklyLimitUSD     *float64 `json:"weekly_limit_usd"`
	MonthlyLimitUSD    *float64 `json:"monthly_limit_usd"`
	Status             string   `json:"status"`
}

// UpstreamCreatedKey 是本地账号要使用的上游 API Key；sub2api 会现场创建，
// NewAPI 则直接复用用户提供的个人 API token。
type UpstreamCreatedKey struct {
	ID      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	GroupID *int64 `json:"group_id"`
	Group   string `json:"group"`
}

// UpstreamProviderClient 封装对单个 sub2api / NewAPI 上游站点的 HTTP 调用。
// 无状态：sub2api access/refresh token 和 NewAPI API token 由调用方传入。
type UpstreamProviderClient struct {
	httpClient *http.Client
}

func NewUpstreamProviderClient() *UpstreamProviderClient {
	return &UpstreamProviderClient{
		httpClient: &http.Client{Timeout: upstreamProviderRequestTimeout},
	}
}

// Login 用账号密码登录上游，拿到 JWT。
//
// 上游若开启验证码，这里会明确返回 ErrUpstreamProviderCaptchaRequired 而不是
// 含糊的 400，方便前端提示管理员改用手动 token。
func (c *UpstreamProviderClient) Login(ctx context.Context, baseURL, username, password string) (*UpstreamLoginResult, error) {
	payload := map[string]string{"email": username, "password": password}
	body, err := c.do(ctx, baseURL, http.MethodPost, "/api/v1/auth/login", "", payload)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
		Requires2FA  bool   `json:"requires_2fa"`
		TempToken    string `json:"temp_token"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse upstream login response: %w", err)
	}

	if resp.Requires2FA {
		return &UpstreamLoginResult{Requires2FA: true, TempToken: resp.TempToken}, nil
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return nil, ErrUpstreamProviderUnauthorized
	}
	return &UpstreamLoginResult{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    tokenExpiry(resp.ExpiresIn),
	}, nil
}

// RefreshToken 用上游 refresh token 换发新的 access/refresh token。
// 上游采用 refresh token 轮转，因此调用方必须持久化响应中的新 refresh token。
func (c *UpstreamProviderClient) RefreshToken(ctx context.Context, baseURL, refreshToken string) (*UpstreamLoginResult, error) {
	payload := map[string]string{"refresh_token": refreshToken}
	body, err := c.do(ctx, baseURL, http.MethodPost, "/api/v1/auth/refresh", "", payload)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse upstream refresh response: %w", err)
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return nil, ErrUpstreamProviderUnauthorized
	}
	return &UpstreamLoginResult{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    tokenExpiry(resp.ExpiresIn),
	}, nil
}

// Login2FA 用 TOTP 码完成两步登录。
func (c *UpstreamProviderClient) Login2FA(ctx context.Context, baseURL, tempToken, totpCode string) (*UpstreamLoginResult, error) {
	payload := map[string]string{"temp_token": tempToken, "totp_code": totpCode}
	body, err := c.do(ctx, baseURL, http.MethodPost, "/api/v1/auth/login/2fa", "", payload)
	if err != nil {
		return nil, err
	}

	var resp struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresIn    int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse upstream 2fa response: %w", err)
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return nil, ErrUpstreamProviderUnauthorized
	}
	return &UpstreamLoginResult{
		AccessToken:  resp.AccessToken,
		RefreshToken: resp.RefreshToken,
		ExpiresAt:    tokenExpiry(resp.ExpiresIn),
	}, nil
}

// GetNewAPIProfile reads the quota available to the supplied personal API token.
// NewAPI API tokens are relay credentials, not dashboard sessions, so the
// token-usage endpoint is the authoritative profile/quota endpoint.
func (c *UpstreamProviderClient) GetNewAPIProfile(ctx context.Context, baseURL, token string) (*UpstreamProfile, error) {
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/usage/token/", token, nil)
	if err != nil {
		return nil, err
	}
	var usage map[string]any
	if err := json.Unmarshal(body, &usage); err != nil {
		return nil, fmt.Errorf("parse NewAPI token usage: %w", err)
	}
	var available any
	availableKnown := false
	if userQuota, ok := usage["user_quota"].(map[string]any); ok {
		available, availableKnown = userQuota["total_available"]
	}
	if !availableKnown {
		available, availableKnown = usage["total_available"]
	}
	availableNumber := jsonNumber(available)
	return &UpstreamProfile{
		Balance:          availableNumber / NewAPIQuotaPerUnit,
		FrozenBalance:    0,
		Status:           "active",
		BalanceKnown:     availableKnown,
		ConcurrencyKnown: false,
	}, nil
}

// ListNewAPIGroups reads NewAPI's public named-group map. The special "auto" group
// is intentionally omitted because it cannot be used as a concrete token group.
// Personal API tokens cannot authenticate dashboard group endpoints, so no
// dashboard credential is used.
func (c *UpstreamProviderClient) ListNewAPIGroups(ctx context.Context, baseURL string) ([]NewAPIGroupInfo, error) {
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/user/groups", "", nil)
	if err != nil {
		return nil, err
	}
	var raw map[string]map[string]any
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse NewAPI groups: %w", err)
	}
	groups := make([]NewAPIGroupInfo, 0, len(raw))
	names := make([]string, 0, len(raw))
	for name := range raw {
		if strings.TrimSpace(name) != "" && name != "auto" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		item := raw[name]
		groups = append(groups, NewAPIGroupInfo{Name: name, Ratio: jsonNumber(item["ratio"]), Desc: newAPIStringValue(item["desc"])})
	}
	return groups, nil
}

func jsonNumber(value any) float64 {
	switch v := value.(type) {
	case float64:
		return v
	case json.Number:
		f, _ := v.Float64()
		return f
	case string:
		f, _ := strconv.ParseFloat(v, 64)
		return f
	default:
		return 0
	}
}

func newAPIStringValue(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func (c *UpstreamProviderClient) GetProfile(ctx context.Context, baseURL, token string) (*UpstreamProfile, error) {
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/v1/user/profile", token, nil)
	if err != nil {
		return nil, err
	}
	var profile UpstreamProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("parse upstream profile: %w", err)
	}
	profile.ConcurrencyKnown = true
	profile.BalanceKnown = true
	return &profile, nil
}

// ListGroups 取上游可绑定的分组（含倍率与限额）。
func (c *UpstreamProviderClient) ListGroups(ctx context.Context, baseURL, token string) ([]UpstreamGroupInfo, error) {
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/v1/groups/available", token, nil)
	if err != nil {
		return nil, err
	}
	var groups []UpstreamGroupInfo
	if err := json.Unmarshal(body, &groups); err != nil {
		return nil, fmt.Errorf("parse upstream groups: %w", err)
	}
	return groups, nil
}

// GetGroupRates 取当前账号的专属分组倍率覆盖（group_id -> rate）。
//
// 这个接口在老版本上游可能不存在，所以 404 不算错误：返回空 map，
// 让调用方回退到分组基础倍率。
func (c *UpstreamProviderClient) GetGroupRates(ctx context.Context, baseURL, token string) (map[int64]float64, error) {
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/v1/groups/rates", token, nil)
	if err != nil {
		if infraerrors.IsNotFound(err) {
			return map[int64]float64{}, nil
		}
		return nil, err
	}
	// 上游返回 map[int64]float64，JSON 里 key 是字符串
	raw := map[string]float64{}
	if err := json.Unmarshal(body, &raw); err != nil {
		// null 或非预期结构都按「无覆盖」处理，不阻断整次同步
		return map[int64]float64{}, nil
	}
	rates := make(map[int64]float64, len(raw))
	for key, value := range raw {
		var id int64
		if _, err := fmt.Sscanf(key, "%d", &id); err == nil {
			rates[id] = value
		}
	}
	return rates, nil
}

// CreateAPIKey 在上游创建一个绑定到指定分组的 API Key。
func (c *UpstreamProviderClient) CreateAPIKey(
	ctx context.Context, baseURL, token, name string, groupID int64,
) (*UpstreamCreatedKey, error) {
	payload := map[string]any{"name": name, "group_id": groupID}
	body, err := c.do(ctx, baseURL, http.MethodPost, "/api/v1/keys", token, payload)
	if err != nil {
		return nil, err
	}
	var created UpstreamCreatedKey
	if err := json.Unmarshal(body, &created); err != nil {
		return nil, fmt.Errorf("parse upstream created key: %w", err)
	}
	if strings.TrimSpace(created.Key) == "" {
		return nil, fmt.Errorf("upstream returned an empty api key")
	}
	return &created, nil
}

// do 发一次请求并解开上游的 {code,message,data} 包装，返回 data 原文。
func (c *UpstreamProviderClient) do(
	ctx context.Context, baseURL, method, path, token string, payload any,
) (json.RawMessage, error) {
	var reqBody io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, fmt.Errorf("encode upstream request: %w", err)
		}
		reqBody = bytes.NewReader(encoded)
	}

	reqCtx, cancel := context.WithTimeout(ctx, upstreamProviderRequestTimeout)
	defer cancel()

	url := strings.TrimSuffix(baseURL, "/") + path
	req, err := http.NewRequestWithContext(reqCtx, method, url, reqBody)
	if err != nil {
		return nil, fmt.Errorf("build upstream request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, ErrUpstreamProviderUnreachable
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, upstreamProviderMaxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("read upstream response: %w", err)
	}

	switch {
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return nil, classifyUpstreamAuthFailure(body)
	case resp.StatusCode == http.StatusNotFound:
		return nil, infraerrors.NotFound(
			"UPSTREAM_PROVIDER_ENDPOINT_NOT_FOUND", "upstream endpoint not found: "+path,
		)
	case resp.StatusCode < 200 || resp.StatusCode >= 300:
		return nil, upstreamHTTPError(resp.StatusCode, body)
	}

	var envelope upstreamEnvelope
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, fmt.Errorf("parse upstream envelope: %w", err)
	}
	if envelope.Success != nil && !*envelope.Success {
		return nil, classifyUpstreamAuthFailure(body)
	}
	if len(envelope.Code) > 0 && string(envelope.Code) != "null" {
		var code int
		if err := json.Unmarshal(envelope.Code, &code); err == nil && code != 0 {
			return nil, classifyUpstreamAuthFailure(body)
		}
		var ok bool
		if err := json.Unmarshal(envelope.Code, &ok); err == nil && !ok {
			return nil, classifyUpstreamAuthFailure(body)
		}
	}
	return envelope.Data, nil
}

// classifyUpstreamAuthFailure 把上游的失败信息翻译成可操作的错误。
// 验证码和 2FA 必须能被区分出来：前者自动登录彻底不可行，后者补个密钥就能修。
func classifyUpstreamAuthFailure(body []byte) error {
	lowered := strings.ToLower(string(body))
	switch {
	case strings.Contains(lowered, "captcha"), strings.Contains(lowered, "turnstile"):
		return ErrUpstreamProviderCaptchaRequired
	case strings.Contains(lowered, "2fa"), strings.Contains(lowered, "totp"):
		return ErrUpstreamProviderTotpRequired
	default:
		return ErrUpstreamProviderUnauthorized
	}
}

func upstreamHTTPError(status int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if len(message) > 300 {
		message = message[:300]
	}
	// 验证码/2FA 提示也可能以 400 返回，先做一次归类
	if status == http.StatusBadRequest {
		lowered := strings.ToLower(message)
		if strings.Contains(lowered, "captcha") || strings.Contains(lowered, "turnstile") {
			return ErrUpstreamProviderCaptchaRequired
		}
	}
	return infraerrors.ServiceUnavailable(
		"UPSTREAM_PROVIDER_HTTP_ERROR",
		fmt.Sprintf("upstream returned %d: %s", status, message),
	)
}

func tokenExpiry(expiresIn int64) time.Time {
	if expiresIn <= 0 {
		return time.Now().Add(upstreamProviderDefaultTokenTTL)
	}
	return time.Now().Add(time.Duration(expiresIn) * time.Second)
}

// upstreamTokenExpiry 推断管理员手填 token 的到期时间。
//
// 只解不验签：签名密钥在上游手里，本地无从校验，我们要的只是 exp 好让
// 调度知道什么时候该提醒续期。解不出 exp（不是 JWT，或是不透明 token）
// 就给一个保守的兜底有效期——写 0/永久都更糟：永久会让过期 token 一直被
// 当成有效而每次同步都 401，0 则让刚贴进来的 token 立刻失效。兜底期内真
// 失效了，同步失败原因会落到 last_sync_error 上，管理员看得到。
func upstreamTokenExpiry(token string) time.Time {
	fallback := time.Now().Add(upstreamProviderManualTokenTTL)

	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	claims := jwt.MapClaims{}
	if _, _, err := parser.ParseUnverified(strings.TrimSpace(token), claims); err != nil {
		return fallback
	}
	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return fallback
	}
	// 已经过期的 token 照原样落库：ensureToken 会因此报 TOKEN_EXPIRED，
	// 比默默当成有效、等到同步 401 才暴露要直接。
	return exp.Time
}
