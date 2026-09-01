package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/util/urlvalidator"
	"github.com/pquerna/otp/totp"
)

var (
	ErrUpstreamProviderNotFound = infraerrors.NotFound(
		"UPSTREAM_PROVIDER_NOT_FOUND", "upstream provider not found",
	)
	ErrUpstreamGroupNotFound = infraerrors.NotFound(
		"UPSTREAM_GROUP_NOT_FOUND", "upstream group not found; sync the provider first",
	)
	ErrUpstreamProviderNameExists = infraerrors.Conflict(
		"UPSTREAM_PROVIDER_NAME_EXISTS", "an upstream provider with this name already exists",
	)
	ErrUpstreamProviderMissingCredentials = infraerrors.BadRequest(
		"UPSTREAM_PROVIDER_MISSING_CREDENTIALS", "upstream provider has no stored password; edit it and re-enter the password",
	)
	// ErrUpstreamProviderCredentialsRequired 是新增上游时既没给密码也没给 token。
	// 两者给一个就行：能自动登录的填密码，被 CF 挡住的直接粘 token。
	ErrUpstreamProviderCredentialsRequired = infraerrors.BadRequest(
		"UPSTREAM_PROVIDER_CREDENTIALS_REQUIRED", "either password or token is required",
	)
	// ErrUpstreamProviderTokenExpired 专指「只有手填 token、且它已过期」。
	// 与 MISSING_CREDENTIALS 分开：这类上游（CF 校验）本来就登不上，
	// 提示管理员补密码是死路，只能让他再贴一个新 token。
	ErrUpstreamProviderTokenExpired = infraerrors.BadRequest(
		"UPSTREAM_PROVIDER_TOKEN_EXPIRED",
		"the manually supplied upstream token has expired; edit the provider and paste a fresh one",
	)
)

// UpstreamProviderService 管理上游 sub2api 供应商。
//
// 职责边界：
//   - 登录并缓存 token（优先用 refresh token 续期，失败后用存的密码重新登录）
//   - 同步余额/并发/分组倍率，只读落库供比价，不自动改本地账号
//   - 按管理员勾选的分组在上游建 API Key，并落地成本地账号
type UpstreamProviderService struct {
	repo         UpstreamProviderRepository
	client       *UpstreamProviderClient
	adminService AdminService
	cfg          *config.Config
}

func NewUpstreamProviderService(
	repo UpstreamProviderRepository,
	adminService AdminService,
	cfg *config.Config,
) *UpstreamProviderService {
	return &UpstreamProviderService{
		repo:         repo,
		client:       NewUpstreamProviderClient(),
		adminService: adminService,
		cfg:          cfg,
	}
}

// CreateUpstreamProviderInput 是新增上游的入参。
type CreateUpstreamProviderInput struct {
	Name     string
	BaseURL  string
	Username string
	Password string
	// RateCorrection 是充值比例修正系数；0/缺省按「不修正」处理（1.0）。
	RateCorrection float64
	TotpSecret     string
	Notes          *string
	SyncEnabled    bool
	// Token 是管理员手填的上游 JWT。上游做了 CF 校验时账号密码登不上去，
	// 直接贴一个浏览器里拿到的 token 就能同步/建号。有效期从 JWT 的 exp 解析。
	Token string
}

// UpdateUpstreamProviderInput 是编辑上游的入参。
// Password/TotpSecret 为空表示不修改。
type UpdateUpstreamProviderInput struct {
	Name     string
	BaseURL  string
	Username string
	Password string
	// RateCorrection 是充值比例修正系数；nil 表示不修改。
	RateCorrection *float64
	TotpSecret     string
	Notes          *string
	Status         string
	SyncEnabled    bool
	// Token 为空表示不修改；非空表示用这个手填 JWT 顶掉缓存的会话。
	Token string
}

func (s *UpstreamProviderService) Create(
	ctx context.Context, input CreateUpstreamProviderInput,
) (*UpstreamProvider, error) {
	normalizedURL, err := s.validateBaseURL(input.BaseURL)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_PROVIDER_NAME_REQUIRED", "name is required")
	}
	// 密码和 token 二选一即可：能自动登录的填密码，被 CF 挡住的直接粘 token。
	token := strings.TrimSpace(input.Token)
	if strings.TrimSpace(input.Password) == "" && token == "" {
		return nil, ErrUpstreamProviderCredentialsRequired
	}

	exists, err := s.repo.ExistsByName(ctx, name, 0)
	if err != nil {
		return nil, fmt.Errorf("check upstream provider name: %w", err)
	}
	if exists {
		return nil, ErrUpstreamProviderNameExists
	}

	provider := &UpstreamProvider{
		Name:           name,
		BaseURL:        normalizedURL,
		Username:       strings.TrimSpace(input.Username),
		Password:       input.Password,
		RateCorrection: NormalizeRateCorrection(input.RateCorrection),
		TotpSecret:     strings.TrimSpace(input.TotpSecret),
		Notes:          input.Notes,
		Status:         StatusActive,
		SyncEnabled:    input.SyncEnabled,
	}
	if token != "" {
		expiresAt := upstreamTokenExpiry(token)
		provider.Token = token
		provider.TokenExpiresAt = &expiresAt
	}
	if err := s.repo.Create(ctx, provider); err != nil {
		return nil, fmt.Errorf("create upstream provider: %w", err)
	}
	return provider, nil
}

func (s *UpstreamProviderService) Update(
	ctx context.Context, id int64, input UpdateUpstreamProviderInput,
) (*UpstreamProvider, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	normalizedURL, err := s.validateBaseURL(input.BaseURL)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return nil, infraerrors.BadRequest("UPSTREAM_PROVIDER_NAME_REQUIRED", "name is required")
	}
	exists, err := s.repo.ExistsByName(ctx, name, id)
	if err != nil {
		return nil, fmt.Errorf("check upstream provider name: %w", err)
	}
	if exists {
		return nil, ErrUpstreamProviderNameExists
	}

	provider.Name = name
	provider.BaseURL = normalizedURL
	provider.Username = strings.TrimSpace(input.Username)
	provider.Notes = input.Notes
	provider.SyncEnabled = input.SyncEnabled
	if input.Status != "" {
		provider.Status = input.Status
	}
	// nil 表示不修改，保留库里已有的系数
	if input.RateCorrection != nil {
		provider.RateCorrection = NormalizeRateCorrection(*input.RateCorrection)
	}
	// 空值表示不修改，交由仓储层判断
	provider.Password = input.Password
	provider.TotpSecret = strings.TrimSpace(input.TotpSecret)

	// 仓储层靠空 Token 判断「不改会话」，所以这里先按「不改」清空，
	// 存量会话留一份用于回填响应——provider 会直接被序列化成 DTO，
	// 不回填的话 has_token/has_refresh_token 会假报成 false。
	existingToken, existingRefreshToken, existingExpiry := provider.Token, provider.RefreshToken, provider.TokenExpiresAt
	provider.Token = ""
	provider.RefreshToken = ""
	provider.TokenExpiresAt = nil

	// 手填 token 优先于改密码：同时填了以粘贴的 token 为准，因为这类上游
	// （CF 校验）本来就登不上，用密码重登只会把刚贴进来的会话作废。
	token := strings.TrimSpace(input.Token)
	if token != "" {
		expiresAt := upstreamTokenExpiry(token)
		provider.Token = token
		provider.TokenExpiresAt = &expiresAt
	}

	if err := s.repo.Update(ctx, provider); err != nil {
		return nil, fmt.Errorf("update upstream provider: %w", err)
	}
	// 既没贴新 token 也没改密码时会话原样保留，把存量值还回去。
	// 改了密码则仓储层已作废缓存 token，保持清空才是真实状态。
	if token == "" && input.Password == "" {
		provider.Token = existingToken
		provider.RefreshToken = existingRefreshToken
		provider.TokenExpiresAt = existingExpiry
	}
	return provider, nil
}

func (s *UpstreamProviderService) Delete(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s *UpstreamProviderService) GetByID(ctx context.Context, id int64) (*UpstreamProvider, error) {
	return s.repo.GetByID(ctx, id)
}

func (s *UpstreamProviderService) List(
	ctx context.Context, params pagination.PaginationParams, status, search string,
) ([]UpstreamProviderWithStats, *pagination.PaginationResult, error) {
	return s.repo.List(ctx, params, status, search)
}

func (s *UpstreamProviderService) ListGroups(ctx context.Context, providerID int64) ([]UpstreamGroup, error) {
	return s.repo.ListGroups(ctx, providerID)
}

// ProviderNamesByIDs 批量取 id → 名称，供账号列表标出账号来自哪个上游。
func (s *UpstreamProviderService) ProviderNamesByIDs(
	ctx context.Context, ids []int64,
) (map[int64]string, error) {
	if s == nil || s.repo == nil || len(ids) == 0 {
		return map[int64]string{}, nil
	}
	return s.repo.ListNamesByIDs(ctx, ids)
}

// CompareGroups 跨上游拉平所有分组做横向比价，按倍率升序。
// platform 为空表示不过滤。
func (s *UpstreamProviderService) CompareGroups(
	ctx context.Context, platform string, params pagination.PaginationParams,
) ([]UpstreamGroupComparison, *pagination.PaginationResult, error) {
	return s.repo.ListAllGroupsForComparison(ctx, strings.TrimSpace(platform), params)
}

// TestConnection 只验证能否登录，不落任何快照。用于新增/编辑时的「测试连接」。
func (s *UpstreamProviderService) TestConnection(ctx context.Context, id int64) (*UpstreamProfile, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	token, err := s.ensureToken(ctx, provider)
	if err != nil {
		return nil, err
	}
	profile, err := s.client.GetProfile(ctx, provider.BaseURL, token)
	if err != nil {
		return nil, err
	}
	return profile, nil
}

// Sync 拉取上游余额、并发与分组倍率，整体落库。
//
// 只读：同步到的并发/倍率仅用于展示比价，不会自动写回本地账号的
// concurrency/rate_multiplier——上游改倍率不该让本地调度和计费口径悄悄变。
func (s *UpstreamProviderService) Sync(ctx context.Context, id int64) (*UpstreamProvider, error) {
	provider, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if err := s.syncProvider(ctx, provider); err != nil {
		// 失败原因落库，前端能直接看到「验证码/2FA/密码错误」
		if markErr := s.repo.MarkSyncFailed(ctx, id, err.Error(), time.Now()); markErr != nil {
			slog.Warn("upstream_provider_mark_sync_failed", "provider_id", id, "error", markErr)
		}
		return nil, err
	}
	return s.repo.GetByID(ctx, id)
}

// SyncAll 供后台定时任务调用，逐个同步已启用的上游。
// 单个失败不影响其余，失败原因各自落库。
func (s *UpstreamProviderService) SyncAll(ctx context.Context) (succeeded, failed int) {
	providers, err := s.repo.ListSyncable(ctx)
	if err != nil {
		slog.Error("upstream_provider_list_syncable_failed", "error", err)
		return 0, 0
	}
	for i := range providers {
		provider := &providers[i]
		if err := s.syncProvider(ctx, provider); err != nil {
			failed++
			slog.Warn("upstream_provider_sync_failed",
				"provider_id", provider.ID, "provider", provider.Name, "error", err)
			if markErr := s.repo.MarkSyncFailed(ctx, provider.ID, err.Error(), time.Now()); markErr != nil {
				slog.Warn("upstream_provider_mark_sync_failed", "provider_id", provider.ID, "error", markErr)
			}
			continue
		}
		succeeded++
	}
	return succeeded, failed
}

func (s *UpstreamProviderService) syncProvider(ctx context.Context, provider *UpstreamProvider) error {
	token, err := s.ensureToken(ctx, provider)
	if err != nil {
		return err
	}

	profile, err := s.client.GetProfile(ctx, provider.BaseURL, token)
	if err != nil {
		return fmt.Errorf("fetch upstream profile: %w", err)
	}

	now := time.Now()
	snapshot := UpstreamSyncSnapshot{
		Balance:             &profile.Balance,
		FrozenBalance:       &profile.FrozenBalance,
		UpstreamConcurrency: &profile.Concurrency,
		SyncedAt:            now,
	}
	if profile.UserID > 0 {
		snapshot.UpstreamUserID = strconv.FormatInt(profile.UserID, 10)
	}

	groups, err := s.client.ListGroups(ctx, provider.BaseURL, token)
	if err != nil {
		return fmt.Errorf("fetch upstream groups: %w", err)
	}

	// 专属倍率是可选能力：老版本上游没有这个接口，拿不到就回退基础倍率。
	rates, err := s.client.GetGroupRates(ctx, provider.BaseURL, token)
	if err != nil {
		slog.Warn("upstream_provider_group_rates_failed",
			"provider_id", provider.ID, "error", err)
		rates = map[int64]float64{}
	}

	snapshotGroups := make([]UpstreamGroup, 0, len(groups))
	for i := range groups {
		group := &groups[i]
		mirrored := UpstreamGroup{
			UpstreamProviderID: provider.ID,
			RemoteGroupID:      group.ID,
			Name:               group.Name,
			Platform:           group.Platform,
			SubscriptionType:   group.SubscriptionType,
			RateMultiplier:     group.RateMultiplier,
			PeakRateEnabled:    group.PeakRateEnabled,
			PeakStart:          group.PeakStart,
			PeakEnd:            group.PeakEnd,
			DailyLimitUSD:      group.DailyLimitUSD,
			WeeklyLimitUSD:     group.WeeklyLimitUSD,
			MonthlyLimitUSD:    group.MonthlyLimitUSD,
			SyncedAt:           now,
		}
		if group.PeakRateEnabled && group.PeakRateMultiplier > 0 {
			peak := group.PeakRateMultiplier
			mirrored.PeakRateMultiplier = &peak
		}
		if rate, ok := rates[group.ID]; ok {
			mirrored.EffectiveRateMultiplier = &rate
		}
		snapshotGroups = append(snapshotGroups, mirrored)
	}

	if err := s.repo.ReplaceGroups(ctx, provider.ID, snapshotGroups); err != nil {
		return fmt.Errorf("store upstream groups: %w", err)
	}
	if err := s.repo.UpdateSyncSnapshot(ctx, provider.ID, snapshot); err != nil {
		return fmt.Errorf("store upstream snapshot: %w", err)
	}
	return nil
}

// ProvisionAccountInput 描述「在上游建 Key 并落地成本地账号」的一次操作。
type ProvisionAccountInput struct {
	ProviderID int64
	// RemoteGroupIDs 是管理员在前端勾选的上游分组
	RemoteGroupIDs []int64
	// LocalGroupIDs 可选：把新账号绑定到哪些本地分组
	LocalGroupIDs []int64
	// Concurrency/Priority 留空则用平台默认
	Concurrency int
	Priority    int
	// KeyNamePrefix 上游 Key 的名字前缀，便于在上游后台辨认
	KeyNamePrefix string
}

// ProvisionedAccount 是一次成功创建的结果。
type ProvisionedAccount struct {
	RemoteGroupID int64  `json:"remote_group_id"`
	GroupName     string `json:"group_name"`
	AccountID     int64  `json:"account_id"`
	AccountName   string `json:"account_name"`
	Error         string `json:"error,omitempty"`
}

// ProvisionAccounts 对每个勾选的上游分组：在上游建 API Key，再落地一个本地账号。
//
// 逐个分组独立处理：某个分组失败（比如上游 Key 数量到上限）不影响其他分组，
// 失败原因回传给前端。已经建好的 Key 不回滚——上游多一个闲置 Key 无害，
// 而回滚需要额外的删除权限且可能半途失败。
func (s *UpstreamProviderService) ProvisionAccounts(
	ctx context.Context, input ProvisionAccountInput,
) ([]ProvisionedAccount, error) {
	provider, err := s.repo.GetByID(ctx, input.ProviderID)
	if err != nil {
		return nil, err
	}
	if len(input.RemoteGroupIDs) == 0 {
		return nil, infraerrors.BadRequest(
			"UPSTREAM_PROVIDER_NO_GROUP_SELECTED", "select at least one upstream group",
		)
	}
	if s.adminService == nil {
		return nil, infraerrors.ServiceUnavailable(
			"UPSTREAM_PROVIDER_ADMIN_UNAVAILABLE", "account service is unavailable",
		)
	}

	token, err := s.ensureToken(ctx, provider)
	if err != nil {
		return nil, err
	}

	results := make([]ProvisionedAccount, 0, len(input.RemoteGroupIDs))
	for _, remoteGroupID := range input.RemoteGroupIDs {
		result := ProvisionedAccount{RemoteGroupID: remoteGroupID}

		group, err := s.repo.GetGroupByRemoteID(ctx, provider.ID, remoteGroupID)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}
		result.GroupName = group.Name

		accountName := s.buildAccountName(input.KeyNamePrefix, provider.Name, group.Name)
		created, err := s.client.CreateAPIKey(ctx, provider.BaseURL, token, accountName, remoteGroupID)
		if err != nil {
			result.Error = err.Error()
			results = append(results, result)
			continue
		}

		account, err := s.createLocalAccount(ctx, provider, group, created, accountName, input)
		if err != nil {
			result.Error = fmt.Sprintf("upstream key created but local account failed: %v", err)
			results = append(results, result)
			continue
		}
		result.AccountID = account.ID
		result.AccountName = account.Name
		results = append(results, result)
	}
	return results, nil
}

// createLocalAccount 用上游签发的 Key 建本地账号。
//
// platform 取上游分组的 platform：这决定了本地按哪套协议转发。
// 上游 sub2api 暴露的是 Anthropic 兼容接口，所以分组没声明平台时兜底 anthropic。
func (s *UpstreamProviderService) createLocalAccount(
	ctx context.Context,
	provider *UpstreamProvider,
	group *UpstreamGroup,
	created *UpstreamCreatedKey,
	accountName string,
	input ProvisionAccountInput,
) (*Account, error) {
	platform := strings.TrimSpace(group.Platform)
	if platform == "" {
		platform = PlatformAnthropic
	}

	concurrency := input.Concurrency
	if concurrency <= 0 {
		// 上游账户的并发上限是所有 Key 共享的，直接照搬会超卖；留给管理员显式设定，
		// 未设定时用平台默认值。
		concurrency = 0
	}

	accountInput := &CreateAccountInput{
		Name:     accountName,
		Supplier: provider.Name,
		Platform: platform,
		Type:     AccountTypeAPIKey,
		Credentials: map[string]any{
			"api_key":  created.Key,
			"base_url": provider.BaseURL,
		},
		Concurrency:           concurrency,
		Priority:              input.Priority,
		GroupIDs:              input.LocalGroupIDs,
		UpstreamProviderID:    &provider.ID,
		UpstreamRemoteGroupID: &group.RemoteGroupID,
		// 没勾本地分组时不要自动绑到平台默认分组：新账号的倍率还没核对过，
		// 悄悄进默认分组会立刻参与真实流量调度。
		SkipDefaultGroupBind: len(input.LocalGroupIDs) == 0,
	}
	return s.adminService.CreateAccount(ctx, accountInput)
}

func (s *UpstreamProviderService) buildAccountName(prefix, providerName, groupName string) string {
	base := strings.TrimSpace(prefix)
	if base == "" {
		base = providerName
	}
	return fmt.Sprintf("%s-%s", base, groupName)
}

// ensureToken 返回可用的上游 token：缓存有效就直接用，否则优先使用 refresh token
// 续期；refresh 失败时再用密码重新登录并回写完整会话。
func (s *UpstreamProviderService) ensureToken(ctx context.Context, provider *UpstreamProvider) (string, error) {
	if provider.HasValidToken(time.Now()) {
		return provider.Token, nil
	}

	if strings.TrimSpace(provider.RefreshToken) != "" {
		result, refreshErr := s.client.RefreshToken(ctx, provider.BaseURL, provider.RefreshToken)
		if refreshErr == nil {
			// 兼容未轮转 refresh token 的旧上游：响应不带新值时继续沿用旧值。
			if strings.TrimSpace(result.RefreshToken) == "" {
				result.RefreshToken = provider.RefreshToken
			}
			s.persistSession(ctx, provider, result)
			return result.AccessToken, nil
		}
		// refresh token 可能因上游重启、管理员主动登出等原因失效；
		// 有密码时允许无感回退登录，避免一次失效令牌就中断同步。
		if provider.Password == "" {
			return "", fmt.Errorf("refresh upstream token: %w", refreshErr)
		}
		slog.Warn("upstream_provider_refresh_failed_fallback_login",
			"provider_id", provider.ID, "error", refreshErr)
	}

	if provider.Password == "" {
		// 只有手填 access token 的上游没有 refresh token 或密码可用来续期，
		// 只能让管理员再贴一个，提示要说清楚是哪种情况。
		if provider.Token != "" {
			return "", ErrUpstreamProviderTokenExpired
		}
		return "", ErrUpstreamProviderMissingCredentials
	}

	result, err := s.client.Login(ctx, provider.BaseURL, provider.Username, provider.Password)
	if err != nil {
		return "", err
	}

	if result.Requires2FA {
		if provider.TotpSecret == "" {
			return "", ErrUpstreamProviderTotpRequired
		}
		code, genErr := totp.GenerateCode(provider.TotpSecret, time.Now())
		if genErr != nil {
			return "", fmt.Errorf("generate upstream totp code: %w", genErr)
		}
		result, err = s.client.Login2FA(ctx, provider.BaseURL, result.TempToken, code)
		if err != nil {
			return "", err
		}
	}

	s.persistSession(ctx, provider, result)
	return result.AccessToken, nil
}

// persistSession 同步更新内存和数据库中的 access/refresh token。
// 缓存写失败不致命：这次请求照常用新 token，下次再登录/续期。
func (s *UpstreamProviderService) persistSession(ctx context.Context, provider *UpstreamProvider, result *UpstreamLoginResult) {
	if result == nil {
		return
	}
	if err := s.repo.UpdateSession(ctx, provider.ID, result.AccessToken, result.RefreshToken, result.ExpiresAt); err != nil {
		slog.Warn("upstream_provider_session_cache_failed", "provider_id", provider.ID, "error", err)
	}
	provider.Token = result.AccessToken
	provider.RefreshToken = result.RefreshToken
	provider.TokenExpiresAt = &result.ExpiresAt
}

// validateBaseURL 复用账号测试那套 SSRF 防护口径：上游地址由管理员输入，
// 必须过 allowlist / 私网校验。
func (s *UpstreamProviderService) validateBaseURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", infraerrors.BadRequest("UPSTREAM_PROVIDER_URL_REQUIRED", "base_url is required")
	}
	if s.cfg == nil {
		return "", infraerrors.ServiceUnavailable(
			"UPSTREAM_PROVIDER_CONFIG_UNAVAILABLE", "configuration is not available",
		)
	}
	allowlist := s.cfg.Security.URLAllowlist
	var (
		normalized string
		err        error
	)
	if !allowlist.Enabled {
		normalized, err = urlvalidator.ValidateURLFormat(trimmed, allowlist.AllowInsecureHTTP)
	} else {
		normalized, err = urlvalidator.ValidateHTTPSURL(trimmed, urlvalidator.ValidationOptions{
			AllowedHosts:     allowlist.UpstreamHosts,
			RequireAllowlist: true,
			AllowPrivate:     allowlist.AllowPrivateHosts,
		})
	}
	if err != nil {
		var appErr *infraerrors.ApplicationError
		if errors.As(err, &appErr) {
			return "", err
		}
		return "", infraerrors.BadRequest("UPSTREAM_PROVIDER_URL_INVALID", err.Error())
	}
	return strings.TrimSuffix(normalized, "/"), nil
}
