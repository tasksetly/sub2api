package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

// 上游 sub2api 实例的管理客户端。
//
// 与 upstream_billing_probe.go 的区别：那里用「账号自带的 API Key」探
// /v1/sub2api/billing，只能看到该 Key 已绑定分组的倍率。这里用「后台登录
// 账号密码」拿 JWT，因此能看到余额、并发、全部可用分组，并能创建新 Key。
const (
	upstreamProviderRequestTimeout = 15 * time.Second
	upstreamProviderMaxBodyBytes   = 512 * 1024

	// token 提前过期的安全边界：避免边界时刻拿着刚好失效的 token 请求
	upstreamProviderTokenSkew = 5 * time.Minute
	// 上游未回传 expires_in 时的保守默认有效期
	upstreamProviderDefaultTokenTTL = 30 * time.Minute
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
	Code    int             `json:"code"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// UpstreamLoginResult 是登录成功后拿到的会话信息。
type UpstreamLoginResult struct {
	AccessToken string
	ExpiresAt   time.Time
	// Requires2FA 为真时 AccessToken 为空，需要带 TOTP 码走第二步
	Requires2FA bool
	TempToken   string
}

// UpstreamProfile 是上游 GET /user/profile 的关键字段。
type UpstreamProfile struct {
	UserID        int64   `json:"id"`
	Email         string  `json:"email"`
	Username      string  `json:"username"`
	Balance       float64 `json:"balance"`
	FrozenBalance float64 `json:"frozen_balance"`
	Concurrency   int     `json:"concurrency"`
	Status        string  `json:"status"`
}

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

// UpstreamCreatedKey 是在上游创建 API Key 后返回的结果。
type UpstreamCreatedKey struct {
	ID      int64  `json:"id"`
	Key     string `json:"key"`
	Name    string `json:"name"`
	GroupID *int64 `json:"group_id"`
}

// UpstreamProviderClient 封装对单个上游 sub2api 站点的 HTTP 调用。
// 无状态：token 由调用方（UpstreamProviderService）缓存与传入。
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
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
		Requires2FA bool   `json:"requires_2fa"`
		TempToken   string `json:"temp_token"`
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
		AccessToken: resp.AccessToken,
		ExpiresAt:   tokenExpiry(resp.ExpiresIn),
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
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse upstream 2fa response: %w", err)
	}
	if strings.TrimSpace(resp.AccessToken) == "" {
		return nil, ErrUpstreamProviderUnauthorized
	}
	return &UpstreamLoginResult{
		AccessToken: resp.AccessToken,
		ExpiresAt:   tokenExpiry(resp.ExpiresIn),
	}, nil
}

// GetProfile 取上游账户的余额与并发限制。
func (c *UpstreamProviderClient) GetProfile(ctx context.Context, baseURL, token string) (*UpstreamProfile, error) {
	body, err := c.do(ctx, baseURL, http.MethodGet, "/api/v1/user/profile", token, nil)
	if err != nil {
		return nil, err
	}
	var profile UpstreamProfile
	if err := json.Unmarshal(body, &profile); err != nil {
		return nil, fmt.Errorf("parse upstream profile: %w", err)
	}
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
	if envelope.Code != 0 {
		return nil, classifyUpstreamAuthFailure(body)
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
