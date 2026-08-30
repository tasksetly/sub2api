package service

import (
	"context"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

// UpstreamProvider 是一个上游 sub2api 供应商（能登录其后台的账号）。
//
// 密码/TOTP/token 在仓储层用 AES-256-GCM 加解密，这里的字段是明文，
// 但绝不能进入 DTO 响应——见 dto.UpstreamProviderFromService。
type UpstreamProvider struct {
	ID      int64
	Name    string
	BaseURL string
	Notes   *string

	Username   string
	Password   string
	TotpSecret string

	// RateCorrection 抹平各上游充值比例差异的修正系数。
	// 充值比例 10 倍就填 0.1；比价倍率 = 上游声明倍率 × RateCorrection。
	RateCorrection float64

	Token          string
	TokenExpiresAt *time.Time

	// 同步来的只读快照
	Balance             *float64
	FrozenBalance       *float64
	UpstreamConcurrency *int
	UpstreamUserID      string

	Status        string
	LastSyncAt    *time.Time
	LastSyncError string
	SyncEnabled   bool

	CreatedAt time.Time
	UpdatedAt time.Time

	// Groups 是同步下来的分组快照（按需加载）
	Groups []UpstreamGroup
}

func (p *UpstreamProvider) IsActive() bool {
	return p.Status == StatusActive
}

// HasValidToken 报告缓存的 token 是否还能用（留了提前过期的安全边界）。
func (p *UpstreamProvider) HasValidToken(now time.Time) bool {
	if p.Token == "" || p.TokenExpiresAt == nil {
		return false
	}
	return p.TokenExpiresAt.After(now.Add(upstreamProviderTokenSkew))
}

// UpstreamGroup 是上游分组的只读镜像，用于横向比价。
type UpstreamGroup struct {
	ID                 int64
	UpstreamProviderID int64
	RemoteGroupID      int64
	Name               string
	Platform           string
	SubscriptionType   string

	RateMultiplier float64
	// EffectiveRateMultiplier 叠加了专属倍率后的实际倍率，nil 表示无覆盖
	EffectiveRateMultiplier *float64
	PeakRateEnabled         bool
	PeakRateMultiplier      *float64
	PeakStart               string
	PeakEnd                 string

	DailyLimitUSD   *float64
	WeeklyLimitUSD  *float64
	MonthlyLimitUSD *float64

	SyncedAt  time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

// ComparableRate 返回该分组用于比价的倍率：优先专属倍率，回退基础倍率。
//
// 注意这是**上游声明的原始倍率**，未做充值比例修正。跨上游比价要用
// UpstreamGroupComparison.CorrectedRate()。
func (g *UpstreamGroup) ComparableRate() float64 {
	if g.EffectiveRateMultiplier != nil {
		return *g.EffectiveRateMultiplier
	}
	return g.RateMultiplier
}

// NormalizeRateCorrection 把非法的修正系数收敛到 1.0（不修正）。
//
// 0 和负数都不是有意义的充值比例：0 会让所有分组的比价倍率变成 0 而排在最前，
// 反而误导决策。缺失/非法一律按「不修正」处理。
func NormalizeRateCorrection(value float64) float64 {
	if value <= 0 {
		return 1.0
	}
	return value
}

// UpstreamProviderWithStats 是列表页用的聚合视图：上游快照 + 本地实际用量成本。
//
// LocalCostUSD/LocalRequests 来自本地 usage_log 按 accounts.upstream_provider_id
// 归集，是「在这个上游实际花了多少钱」的口径，与上游自己声明的倍率相互印证。
type UpstreamProviderWithStats struct {
	UpstreamProvider
	AccountCount  int64
	GroupCount    int64
	LocalCostUSD  float64
	LocalRequests int64
	// MinRate/MaxRate 是该上游各分组比价倍率的区间，便于列表页横向扫
	MinRate *float64
	MaxRate *float64
}

// UpstreamGroupComparison 是跨上游拉平的分组视图，用于横向比价。
//
// 与 UpstreamGroup 的区别：带上了所属上游的名称和余额，因此单独一行就能
// 判断「这个价格值不值得用」——倍率再低，上游没余额也用不了。
type UpstreamGroupComparison struct {
	UpstreamGroup
	ProviderName    string
	ProviderStatus  string
	ProviderBalance *float64
	// ProviderSyncEnabled 为 false 时该行数据可能已经过期
	ProviderSyncEnabled bool
	// ProviderRateCorrection 是该上游的充值比例修正系数
	ProviderRateCorrection float64
	// LocalAccountCount 是本地已经基于该上游分组建了几个账号；
	// >0 说明这个价位已经在用了
	LocalAccountCount int64
}

// CorrectedRate 返回跨上游可比的真实成本倍率：声明倍率 × 充值比例修正。
//
// 这才是排序和比价该用的值。只用 ComparableRate() 会把「倍率 1.0 但充值
// 比例 10 倍」的上游排在「倍率 0.2 且 1:1 充值」之后，结论正好相反。
func (c *UpstreamGroupComparison) CorrectedRate() float64 {
	return c.ComparableRate() * NormalizeRateCorrection(c.ProviderRateCorrection)
}

// UpstreamProviderRepository 是上游供应商的持久化接口。
// 实现负责 password/totp_secret/token 的加解密。
type UpstreamProviderRepository interface {
	Create(ctx context.Context, provider *UpstreamProvider) error
	GetByID(ctx context.Context, id int64) (*UpstreamProvider, error)
	Update(ctx context.Context, provider *UpstreamProvider) error
	Delete(ctx context.Context, id int64) error

	List(ctx context.Context, params pagination.PaginationParams, status, search string) ([]UpstreamProviderWithStats, *pagination.PaginationResult, error)
	ListSyncable(ctx context.Context) ([]UpstreamProvider, error)
	ExistsByName(ctx context.Context, name string, excludeID int64) (bool, error)
	// ListNamesByIDs 批量取 id → 名称，供账号列表回填「来自哪个上游」。
	// 只查名称，不解密任何凭据：这条路径每次列表请求都会走，没必要付解密开销。
	// 包含软删除的上游——账号还在用它签发的 Key，名称得照样显示得出来。
	ListNamesByIDs(ctx context.Context, ids []int64) (map[int64]string, error)

	// UpdateSession 只更新 token 缓存，避免整行覆盖把并发的同步结果写丢。
	UpdateSession(ctx context.Context, id int64, token string, expiresAt time.Time) error
	// UpdateSyncSnapshot 写入余额/并发/同步状态。
	UpdateSyncSnapshot(ctx context.Context, id int64, snapshot UpstreamSyncSnapshot) error
	// MarkSyncFailed 记录同步失败原因，供前端展示。
	MarkSyncFailed(ctx context.Context, id int64, reason string, at time.Time) error

	// ReplaceGroups 用最新一次同步结果整体覆盖该上游的分组快照。
	ReplaceGroups(ctx context.Context, providerID int64, groups []UpstreamGroup) error
	ListGroups(ctx context.Context, providerID int64) ([]UpstreamGroup, error)
	GetGroupByRemoteID(ctx context.Context, providerID, remoteGroupID int64) (*UpstreamGroup, error)

	// ListAllGroupsForComparison 跨上游拉平所有分组，按比价倍率升序。
	// platform 为空表示不过滤。
	ListAllGroupsForComparison(
		ctx context.Context, platform string, params pagination.PaginationParams,
	) ([]UpstreamGroupComparison, *pagination.PaginationResult, error)
}

// UpstreamSyncSnapshot 是一次成功同步后要落库的上游账户信息。
type UpstreamSyncSnapshot struct {
	Balance             *float64
	FrozenBalance       *float64
	UpstreamConcurrency *int
	UpstreamUserID      string
	SyncedAt            time.Time
}
