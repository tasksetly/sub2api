package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/schema/mixins"
	"github.com/Wei-Shaw/sub2api/ent/upstreamgroup"
	"github.com/Wei-Shaw/sub2api/ent/upstreamprovider"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/lib/pq"
)

// upstreamProviderRepository 持久化上游供应商。
//
// 加解密边界就在这一层：service.UpstreamProvider 里的 Password/TotpSecret/Token
// 始终是明文，落库前用 AES-256-GCM 加密，读出后解密。
type upstreamProviderRepository struct {
	client    *dbent.Client
	sql       sqlExecutor
	encryptor service.SecretEncryptor
}

func NewUpstreamProviderRepository(
	client *dbent.Client, sqlDB *sql.DB, encryptor service.SecretEncryptor,
) service.UpstreamProviderRepository {
	return &upstreamProviderRepository{client: client, sql: sqlDB, encryptor: encryptor}
}

func (r *upstreamProviderRepository) Create(ctx context.Context, provider *service.UpstreamProvider) error {
	encryptedPassword, err := r.encryptor.Encrypt(provider.Password)
	if err != nil {
		return fmt.Errorf("encrypt upstream password: %w", err)
	}

	builder := r.client.UpstreamProvider.Create().
		SetName(provider.Name).
		SetBaseURL(provider.BaseURL).
		SetUsername(provider.Username).
		SetPasswordEncrypted(encryptedPassword).
		SetStatus(provider.Status).
		SetSyncEnabled(provider.SyncEnabled).
		SetRateCorrection(service.NormalizeRateCorrection(provider.RateCorrection))
	if provider.Notes != nil {
		builder.SetNotes(*provider.Notes)
	}
	if provider.TotpSecret != "" {
		encryptedSecret, encErr := r.encryptor.Encrypt(provider.TotpSecret)
		if encErr != nil {
			return fmt.Errorf("encrypt upstream totp secret: %w", encErr)
		}
		builder.SetTotpSecretEncrypted(encryptedSecret)
	}
	// 管理员手填的 token：上游做了 CF 校验时这是唯一能用的凭据
	if provider.Token != "" {
		encryptedToken, encErr := r.encryptor.Encrypt(provider.Token)
		if encErr != nil {
			return fmt.Errorf("encrypt upstream token: %w", encErr)
		}
		builder.SetTokenEncrypted(encryptedToken)
		if provider.TokenExpiresAt != nil {
			builder.SetTokenExpiresAt(*provider.TokenExpiresAt)
		}
	}
	if provider.RefreshToken != "" {
		encryptedRefreshToken, encErr := r.encryptor.Encrypt(provider.RefreshToken)
		if encErr != nil {
			return fmt.Errorf("encrypt upstream refresh token: %w", encErr)
		}
		builder.SetRefreshTokenEncrypted(encryptedRefreshToken)
	}

	created, err := builder.Save(ctx)
	if err != nil {
		return err
	}
	provider.ID = created.ID
	provider.CreatedAt = created.CreatedAt
	provider.UpdatedAt = created.UpdatedAt
	return nil
}

func (r *upstreamProviderRepository) GetByID(ctx context.Context, id int64) (*service.UpstreamProvider, error) {
	entity, err := r.client.UpstreamProvider.Get(ctx, id)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrUpstreamProviderNotFound
		}
		return nil, err
	}
	return r.entityToService(entity), nil
}

func (r *upstreamProviderRepository) Update(ctx context.Context, provider *service.UpstreamProvider) error {
	builder := r.client.UpstreamProvider.UpdateOneID(provider.ID).
		SetName(provider.Name).
		SetBaseURL(provider.BaseURL).
		SetUsername(provider.Username).
		SetStatus(provider.Status).
		SetSyncEnabled(provider.SyncEnabled).
		SetRateCorrection(service.NormalizeRateCorrection(provider.RateCorrection))

	if provider.Notes != nil {
		builder.SetNotes(*provider.Notes)
	} else {
		builder.ClearNotes()
	}

	passwordChanged := provider.Password != ""
	tokenProvided := provider.Token != ""
	refreshTokenProvided := provider.RefreshToken != ""

	// 密码为空表示「不修改」，避免编辑其他字段时把密码清掉。
	if passwordChanged {
		encrypted, err := r.encryptor.Encrypt(provider.Password)
		if err != nil {
			return fmt.Errorf("encrypt upstream password: %w", err)
		}
		builder.SetPasswordEncrypted(encrypted)
	}
	if provider.TotpSecret != "" {
		encrypted, err := r.encryptor.Encrypt(provider.TotpSecret)
		if err != nil {
			return fmt.Errorf("encrypt upstream totp secret: %w", err)
		}
		builder.SetTotpSecretEncrypted(encrypted)
	}
	// token 为空表示「不改会话」。密码和 token 同时填写时以手填 token 为准。
	if tokenProvided {
		encrypted, err := r.encryptor.Encrypt(provider.Token)
		if err != nil {
			return fmt.Errorf("encrypt upstream token: %w", err)
		}
		builder.SetTokenEncrypted(encrypted)
		if provider.TokenExpiresAt != nil {
			builder.SetTokenExpiresAt(*provider.TokenExpiresAt)
		} else {
			builder.ClearTokenExpiresAt()
		}
	}

	// 每个会话列只生成一次更新操作，避免 PostgreSQL 报 multiple assignments。
	// 改密码会作废旧 access token；手填 token 时它优先于密码。refresh token 可单独补填。
	if tokenProvided {
		if refreshTokenProvided {
			encrypted, err := r.encryptor.Encrypt(provider.RefreshToken)
			if err != nil {
				return fmt.Errorf("encrypt upstream refresh token: %w", err)
			}
			builder.SetRefreshTokenEncrypted(encrypted)
		} else {
			builder.ClearRefreshTokenEncrypted()
		}
	} else if passwordChanged {
		builder.ClearTokenEncrypted().ClearTokenExpiresAt()
		if refreshTokenProvided {
			encrypted, err := r.encryptor.Encrypt(provider.RefreshToken)
			if err != nil {
				return fmt.Errorf("encrypt upstream refresh token: %w", err)
			}
			builder.SetRefreshTokenEncrypted(encrypted)
		} else {
			builder.ClearRefreshTokenEncrypted()
		}
	} else if refreshTokenProvided {
		encrypted, err := r.encryptor.Encrypt(provider.RefreshToken)
		if err != nil {
			return fmt.Errorf("encrypt upstream refresh token: %w", err)
		}
		builder.SetRefreshTokenEncrypted(encrypted)
	}

	updated, err := builder.Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamProviderNotFound
		}
		return err
	}
	provider.UpdatedAt = updated.UpdatedAt
	return nil
}

func (r *upstreamProviderRepository) Delete(ctx context.Context, id int64) error {
	err := r.client.UpstreamProvider.DeleteOneID(id).Exec(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return service.ErrUpstreamProviderNotFound
		}
		return err
	}
	return nil
}

func (r *upstreamProviderRepository) List(
	ctx context.Context, params pagination.PaginationParams, status, search string,
) ([]service.UpstreamProviderWithStats, *pagination.PaginationResult, error) {
	q := r.client.UpstreamProvider.Query()
	if status != "" {
		q = q.Where(upstreamprovider.StatusEQ(status))
	}
	if search != "" {
		q = q.Where(upstreamprovider.Or(
			upstreamprovider.NameContainsFold(search),
			upstreamprovider.BaseURLContainsFold(search),
		))
	}

	total, err := q.Count(ctx)
	if err != nil {
		return nil, nil, err
	}

	entities, err := q.
		Order(dbent.Desc(upstreamprovider.FieldID)).
		Offset(params.Offset()).
		Limit(params.Limit()).
		All(ctx)
	if err != nil {
		return nil, nil, err
	}

	providers := make([]service.UpstreamProviderWithStats, 0, len(entities))
	ids := make([]int64, 0, len(entities))
	for _, entity := range entities {
		providers = append(providers, service.UpstreamProviderWithStats{
			UpstreamProvider: *r.entityToService(entity),
		})
		ids = append(ids, entity.ID)
	}

	if len(ids) > 0 {
		if err := r.attachStats(ctx, providers, ids); err != nil {
			// 统计失败不该让整个列表打不开，降级为无统计数据
			slog.Warn("upstream_provider_stats_failed", "error", err)
		}
	}

	return providers, paginationResultFromTotal(int64(total), params), nil
}

// attachStats 补齐账号数、分组倍率区间，以及本地实际用量成本。
//
// 本地成本按 accounts.upstream_provider_id 归集 usage_logs.total_cost，
// 这是「在这个上游实际花了多少钱」的口径。
func (r *upstreamProviderRepository) attachStats(
	ctx context.Context, providers []service.UpstreamProviderWithStats, ids []int64,
) error {
	index := make(map[int64]*service.UpstreamProviderWithStats, len(providers))
	for i := range providers {
		index[providers[i].ID] = &providers[i]
	}

	const statsQuery = `
		SELECT a.upstream_provider_id,
		       COUNT(DISTINCT a.id)                     AS account_count,
		       COALESCE(SUM(u.total_cost), 0)           AS local_cost,
		       COUNT(u.id)                              AS local_requests
		FROM accounts a
		LEFT JOIN usage_logs u ON u.account_id = a.id
		WHERE a.upstream_provider_id = ANY($1) AND a.deleted_at IS NULL
		GROUP BY a.upstream_provider_id`
	rows, err := r.sql.QueryContext(ctx, statsQuery, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("query upstream provider stats: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var providerID, accountCount, localRequests int64
		var localCost float64
		if err := rows.Scan(&providerID, &accountCount, &localCost, &localRequests); err != nil {
			return fmt.Errorf("scan upstream provider stats: %w", err)
		}
		if target, ok := index[providerID]; ok {
			target.AccountCount = accountCount
			target.LocalCostUSD = localCost
			target.LocalRequests = localRequests
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate upstream provider stats: %w", err)
	}

	return r.attachGroupRateRange(ctx, index, ids)
}

// attachGroupRateRange 填充各上游分组比价倍率的区间，便于列表页横向扫。
// COALESCE 与 UpstreamGroup.ComparableRate() 口径一致：优先专属倍率。
func (r *upstreamProviderRepository) attachGroupRateRange(
	ctx context.Context, index map[int64]*service.UpstreamProviderWithStats, ids []int64,
) error {
	// 倍率区间也要乘上充值比例修正，否则列表页的区间与比价表的排序口径不一致，
	// 两处对着看会得出相反结论。CASE 防 0/负数，与 NormalizeRateCorrection 一致。
	const rateQuery = `
		SELECT g.upstream_provider_id,
		       COUNT(*) AS group_count,
		       MIN(COALESCE(g.effective_rate_multiplier, g.rate_multiplier)
		           * CASE WHEN p.rate_correction > 0 THEN p.rate_correction ELSE 1 END) AS min_rate,
		       MAX(COALESCE(g.effective_rate_multiplier, g.rate_multiplier)
		           * CASE WHEN p.rate_correction > 0 THEN p.rate_correction ELSE 1 END) AS max_rate
		FROM upstream_groups g
		JOIN upstream_providers p ON p.id = g.upstream_provider_id
		WHERE g.upstream_provider_id = ANY($1)
		GROUP BY g.upstream_provider_id`
	rows, err := r.sql.QueryContext(ctx, rateQuery, pq.Array(ids))
	if err != nil {
		return fmt.Errorf("query upstream group rates: %w", err)
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var providerID, groupCount int64
		var minRate, maxRate sql.NullFloat64
		if err := rows.Scan(&providerID, &groupCount, &minRate, &maxRate); err != nil {
			return fmt.Errorf("scan upstream group rates: %w", err)
		}
		target, ok := index[providerID]
		if !ok {
			continue
		}
		target.GroupCount = groupCount
		if minRate.Valid {
			value := minRate.Float64
			target.MinRate = &value
		}
		if maxRate.Valid {
			value := maxRate.Float64
			target.MaxRate = &value
		}
	}
	return rows.Err()
}

func (r *upstreamProviderRepository) ListSyncable(ctx context.Context) ([]service.UpstreamProvider, error) {
	entities, err := r.client.UpstreamProvider.Query().
		Where(
			upstreamprovider.SyncEnabledEQ(true),
			upstreamprovider.StatusEQ(service.StatusActive),
		).
		Order(dbent.Asc(upstreamprovider.FieldID)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	providers := make([]service.UpstreamProvider, 0, len(entities))
	for _, entity := range entities {
		providers = append(providers, *r.entityToService(entity))
	}
	return providers, nil
}

// ListNamesByIDs 批量取 id → 名称，供账号列表回填「来自哪个上游」。
//
// 只 Select id/name 两列，不走 entityToService：那条路径会解密密码/TOTP/token，
// 而账号列表每次请求都要调这里，白付解密开销没有意义。
//
// SkipSoftDelete：上游被删后 accounts.upstream_provider_id 是 ON DELETE SET NULL，
// 但软删除不会触发它，外键还指着那一行。不跳过过滤的话这些账号的来源列会空掉。
func (r *upstreamProviderRepository) ListNamesByIDs(
	ctx context.Context, ids []int64,
) (map[int64]string, error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	var rows []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	err := r.client.UpstreamProvider.Query().
		Where(upstreamprovider.IDIn(ids...)).
		Select(upstreamprovider.FieldID, upstreamprovider.FieldName).
		Scan(mixins.SkipSoftDelete(ctx), &rows)
	if err != nil {
		return nil, fmt.Errorf("list upstream provider names: %w", err)
	}
	names := make(map[int64]string, len(rows))
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

func (r *upstreamProviderRepository) ExistsByName(ctx context.Context, name string, excludeID int64) (bool, error) {
	q := r.client.UpstreamProvider.Query().Where(upstreamprovider.NameEQ(name))
	if excludeID > 0 {
		q = q.Where(upstreamprovider.IDNEQ(excludeID))
	}
	return q.Exist(ctx)
}

func (r *upstreamProviderRepository) UpdateSession(
	ctx context.Context, id int64, token, refreshToken string, expiresAt time.Time,
) error {
	encrypted, err := r.encryptor.Encrypt(token)
	if err != nil {
		return fmt.Errorf("encrypt upstream token: %w", err)
	}
	builder := r.client.UpstreamProvider.UpdateOneID(id).
		SetTokenEncrypted(encrypted).
		SetTokenExpiresAt(expiresAt)
	if refreshToken != "" {
		encryptedRefreshToken, err := r.encryptor.Encrypt(refreshToken)
		if err != nil {
			return fmt.Errorf("encrypt upstream refresh token: %w", err)
		}
		builder.SetRefreshTokenEncrypted(encryptedRefreshToken)
	} else {
		builder.ClearRefreshTokenEncrypted()
	}
	return builder.Exec(ctx)
}

func (r *upstreamProviderRepository) UpdateSyncSnapshot(
	ctx context.Context, id int64, snapshot service.UpstreamSyncSnapshot,
) error {
	builder := r.client.UpstreamProvider.UpdateOneID(id).
		SetLastSyncAt(snapshot.SyncedAt).
		ClearLastSyncError()

	if snapshot.Balance != nil {
		builder.SetBalance(*snapshot.Balance)
	}
	if snapshot.FrozenBalance != nil {
		builder.SetFrozenBalance(*snapshot.FrozenBalance)
	}
	if snapshot.UpstreamConcurrency != nil {
		builder.SetUpstreamConcurrency(*snapshot.UpstreamConcurrency)
	}
	if snapshot.UpstreamUserID != "" {
		builder.SetUpstreamUserID(snapshot.UpstreamUserID)
	}
	return builder.Exec(ctx)
}

func (r *upstreamProviderRepository) MarkSyncFailed(
	ctx context.Context, id int64, reason string, at time.Time,
) error {
	return r.client.UpstreamProvider.UpdateOneID(id).
		SetLastSyncAt(at).
		SetLastSyncError(reason).
		Exec(ctx)
}

// ReplaceGroups 用最新一次同步结果整体覆盖分组快照。
//
// 按 (provider, remote_group_id) upsert 而非先删后插：直接 delete-then-insert
// 会让 id 每轮同步都变，前端勾选状态和任何引用都会失效。上游已删除的分组
// 在这里被清掉。
func (r *upstreamProviderRepository) ReplaceGroups(
	ctx context.Context, providerID int64, groups []service.UpstreamGroup,
) error {
	tx, err := r.client.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	keep := make([]int64, 0, len(groups))
	for i := range groups {
		group := &groups[i]
		keep = append(keep, group.RemoteGroupID)

		existing, findErr := tx.UpstreamGroup.Query().
			Where(
				upstreamgroup.UpstreamProviderIDEQ(providerID),
				upstreamgroup.RemoteGroupIDEQ(group.RemoteGroupID),
			).
			Only(ctx)
		switch {
		case findErr == nil:
			update := tx.UpstreamGroup.UpdateOneID(existing.ID).
				SetName(group.Name).
				SetPlatform(group.Platform).
				SetSubscriptionType(group.SubscriptionType).
				SetRateMultiplier(group.RateMultiplier).
				SetPeakRateEnabled(group.PeakRateEnabled).
				SetPeakStart(group.PeakStart).
				SetPeakEnd(group.PeakEnd).
				SetSyncedAt(group.SyncedAt)
			applyOptionalGroupRates(update, group)
			if err := update.Exec(ctx); err != nil {
				return fmt.Errorf("update upstream group: %w", err)
			}
		case dbent.IsNotFound(findErr):
			create := tx.UpstreamGroup.Create().
				SetUpstreamProviderID(providerID).
				SetRemoteGroupID(group.RemoteGroupID).
				SetName(group.Name).
				SetPlatform(group.Platform).
				SetSubscriptionType(group.SubscriptionType).
				SetRateMultiplier(group.RateMultiplier).
				SetPeakRateEnabled(group.PeakRateEnabled).
				SetPeakStart(group.PeakStart).
				SetPeakEnd(group.PeakEnd).
				SetSyncedAt(group.SyncedAt)
			if group.EffectiveRateMultiplier != nil {
				create.SetEffectiveRateMultiplier(*group.EffectiveRateMultiplier)
			}
			if group.PeakRateMultiplier != nil {
				create.SetPeakRateMultiplier(*group.PeakRateMultiplier)
			}
			if group.DailyLimitUSD != nil {
				create.SetDailyLimitUsd(*group.DailyLimitUSD)
			}
			if group.WeeklyLimitUSD != nil {
				create.SetWeeklyLimitUsd(*group.WeeklyLimitUSD)
			}
			if group.MonthlyLimitUSD != nil {
				create.SetMonthlyLimitUsd(*group.MonthlyLimitUSD)
			}
			if err := create.Exec(ctx); err != nil {
				return fmt.Errorf("create upstream group: %w", err)
			}
		default:
			return fmt.Errorf("query upstream group: %w", findErr)
		}
	}

	// 清掉上游已经不存在的分组
	stale := tx.UpstreamGroup.Delete().
		Where(upstreamgroup.UpstreamProviderIDEQ(providerID))
	if len(keep) > 0 {
		stale = stale.Where(upstreamgroup.RemoteGroupIDNotIn(keep...))
	}
	if _, err := stale.Exec(ctx); err != nil {
		return fmt.Errorf("prune upstream groups: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit upstream groups: %w", err)
	}
	committed = true
	return nil
}

func applyOptionalGroupRates(update *dbent.UpstreamGroupUpdateOne, group *service.UpstreamGroup) {
	if group.EffectiveRateMultiplier != nil {
		update.SetEffectiveRateMultiplier(*group.EffectiveRateMultiplier)
	} else {
		update.ClearEffectiveRateMultiplier()
	}
	if group.PeakRateMultiplier != nil {
		update.SetPeakRateMultiplier(*group.PeakRateMultiplier)
	} else {
		update.ClearPeakRateMultiplier()
	}
	if group.DailyLimitUSD != nil {
		update.SetDailyLimitUsd(*group.DailyLimitUSD)
	} else {
		update.ClearDailyLimitUsd()
	}
	if group.WeeklyLimitUSD != nil {
		update.SetWeeklyLimitUsd(*group.WeeklyLimitUSD)
	} else {
		update.ClearWeeklyLimitUsd()
	}
	if group.MonthlyLimitUSD != nil {
		update.SetMonthlyLimitUsd(*group.MonthlyLimitUSD)
	} else {
		update.ClearMonthlyLimitUsd()
	}
}

func (r *upstreamProviderRepository) ListGroups(
	ctx context.Context, providerID int64,
) ([]service.UpstreamGroup, error) {
	entities, err := r.client.UpstreamGroup.Query().
		Where(upstreamgroup.UpstreamProviderIDEQ(providerID)).
		Order(dbent.Asc(upstreamgroup.FieldName)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]service.UpstreamGroup, 0, len(entities))
	for _, entity := range entities {
		groups = append(groups, *upstreamGroupEntityToService(entity))
	}
	return groups, nil
}

// ListAllGroupsForComparison 跨上游拉平所有分组，按比价倍率升序。
//
// 用原生 SQL 而非 Ent：需要 join 上游拿名称和余额，还要左连 accounts 统计
// 本地已建号数量。COALESCE(effective, base) 与 UpstreamGroup.ComparableRate()
// 口径一致——专属倍率优先。
//
// 只列出未软删除且状态为 active 的上游：失效的上游其分组不应出现在比价列表中。
func (r *upstreamProviderRepository) ListAllGroupsForComparison(
	ctx context.Context, platform string, params pagination.PaginationParams,
) ([]service.UpstreamGroupComparison, *pagination.PaginationResult, error) {
	limit := params.Limit()
	offset := params.Offset()

	const countQuery = `
		SELECT COUNT(*)
		FROM upstream_groups g
		JOIN upstream_providers p ON p.id = g.upstream_provider_id 
			AND p.deleted_at IS NULL 
			AND p.status = 'active'
		WHERE ($1 = '' OR g.platform = $1)`
	// 用 QueryContext 而非 QueryRowContext：sqlExecutor 接口只暴露
	// ExecContext/QueryContext，给接口加方法会牵动所有实现与测试替身。
	var total int64
	countRows, err := r.sql.QueryContext(ctx, countQuery, platform)
	if err != nil {
		return nil, nil, fmt.Errorf("count upstream group comparison: %w", err)
	}
	if countRows.Next() {
		if scanErr := countRows.Scan(&total); scanErr != nil {
			_ = countRows.Close()
			return nil, nil, fmt.Errorf("scan comparison count: %w", scanErr)
		}
	}
	if closeErr := countRows.Close(); closeErr != nil {
		return nil, nil, fmt.Errorf("close comparison count: %w", closeErr)
	}
	// 排序用「声明倍率 × 充值比例修正」——这才是跨上游可比的真实成本。
	// GREATEST(p.rate_correction, 0) 之外还要防 0：0 会让所有行倍率变成 0
	// 排在最前，与 service.NormalizeRateCorrection 口径保持一致。
	const query = `
		SELECT g.id, g.upstream_provider_id, g.remote_group_id, g.name, g.platform,
		       g.subscription_type, g.rate_multiplier, g.effective_rate_multiplier,
		       g.peak_rate_enabled, g.peak_rate_multiplier, g.peak_start, g.peak_end,
		       g.daily_limit_usd, g.weekly_limit_usd, g.monthly_limit_usd, g.synced_at,
		       p.name, p.status, p.balance, p.sync_enabled, p.rate_correction,
		       COALESCE(a.cnt, 0) AS local_account_count
		FROM upstream_groups g
		JOIN upstream_providers p ON p.id = g.upstream_provider_id 
			AND p.deleted_at IS NULL 
			AND p.status = 'active'
		LEFT JOIN (
		    SELECT upstream_provider_id, upstream_remote_group_id, COUNT(*) AS cnt
		    FROM accounts
		    WHERE deleted_at IS NULL
		      AND upstream_provider_id IS NOT NULL
		      AND upstream_remote_group_id IS NOT NULL
		    GROUP BY upstream_provider_id, upstream_remote_group_id
		) a ON a.upstream_provider_id = g.upstream_provider_id
		   AND a.upstream_remote_group_id = g.remote_group_id
		WHERE ($1 = '' OR g.platform = $1)
		ORDER BY COALESCE(g.effective_rate_multiplier, g.rate_multiplier)
		         * CASE WHEN p.rate_correction > 0 THEN p.rate_correction ELSE 1 END ASC,
		         p.name ASC, g.name ASC
		LIMIT $2 OFFSET $3`

	rows, err := r.sql.QueryContext(ctx, query, platform, limit, offset)
	if err != nil {
		return nil, nil, fmt.Errorf("query upstream group comparison: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]service.UpstreamGroupComparison, 0)
	for rows.Next() {
		var (
			item          service.UpstreamGroupComparison
			effectiveRate sql.NullFloat64
			peakRate      sql.NullFloat64
			dailyLimit    sql.NullFloat64
			weeklyLimit   sql.NullFloat64
			monthlyLimit  sql.NullFloat64
			balance       sql.NullFloat64
		)
		if err := rows.Scan(
			&item.ID, &item.UpstreamProviderID, &item.RemoteGroupID, &item.Name, &item.Platform,
			&item.SubscriptionType, &item.RateMultiplier, &effectiveRate,
			&item.PeakRateEnabled, &peakRate, &item.PeakStart, &item.PeakEnd,
			&dailyLimit, &weeklyLimit, &monthlyLimit, &item.SyncedAt,
			&item.ProviderName, &item.ProviderStatus, &balance, &item.ProviderSyncEnabled,
			&item.ProviderRateCorrection,
			&item.LocalAccountCount,
		); err != nil {
			return nil, nil, fmt.Errorf("scan upstream group comparison: %w", err)
		}
		if effectiveRate.Valid {
			item.EffectiveRateMultiplier = &effectiveRate.Float64
		}
		if peakRate.Valid {
			item.PeakRateMultiplier = &peakRate.Float64
		}
		if dailyLimit.Valid {
			item.DailyLimitUSD = &dailyLimit.Float64
		}
		if weeklyLimit.Valid {
			item.WeeklyLimitUSD = &weeklyLimit.Float64
		}
		if monthlyLimit.Valid {
			item.MonthlyLimitUSD = &monthlyLimit.Float64
		}
		if balance.Valid {
			item.ProviderBalance = &balance.Float64
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate upstream group comparison: %w", err)
	}
	return result, paginationResultFromTotal(total, params), nil
}

func (r *upstreamProviderRepository) GetGroupByRemoteID(
	ctx context.Context, providerID, remoteGroupID int64,
) (*service.UpstreamGroup, error) {
	entity, err := r.client.UpstreamGroup.Query().
		Where(
			upstreamgroup.UpstreamProviderIDEQ(providerID),
			upstreamgroup.RemoteGroupIDEQ(remoteGroupID),
		).
		Only(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, service.ErrUpstreamGroupNotFound
		}
		return nil, err
	}
	return upstreamGroupEntityToService(entity), nil
}

// entityToService 把实体转成 service 模型，顺带解密敏感字段。
//
// 解密失败只记日志、留空值：一个坏密文不该让整张列表打不开，
// 后续用到时会因为凭据为空而走重新登录/提示重填。
func (r *upstreamProviderRepository) entityToService(entity *dbent.UpstreamProvider) *service.UpstreamProvider {
	if entity == nil {
		return nil
	}
	provider := &service.UpstreamProvider{
		ID:                  entity.ID,
		Name:                entity.Name,
		BaseURL:             entity.BaseURL,
		Notes:               entity.Notes,
		Username:            entity.Username,
		RateCorrection:      service.NormalizeRateCorrection(entity.RateCorrection),
		Balance:             entity.Balance,
		FrozenBalance:       entity.FrozenBalance,
		UpstreamConcurrency: entity.UpstreamConcurrency,
		Status:              entity.Status,
		LastSyncAt:          entity.LastSyncAt,
		SyncEnabled:         entity.SyncEnabled,
		TokenExpiresAt:      entity.TokenExpiresAt,
		CreatedAt:           entity.CreatedAt,
		UpdatedAt:           entity.UpdatedAt,
	}
	if entity.UpstreamUserID != nil {
		provider.UpstreamUserID = *entity.UpstreamUserID
	}
	if entity.LastSyncError != nil {
		provider.LastSyncError = *entity.LastSyncError
	}

	if entity.PasswordEncrypted != "" {
		if plain, err := r.encryptor.Decrypt(entity.PasswordEncrypted); err == nil {
			provider.Password = plain
		} else {
			slog.Error("upstream_provider_password_decrypt_failed", "provider_id", entity.ID, "error", err)
		}
	}
	if entity.TotpSecretEncrypted != nil && *entity.TotpSecretEncrypted != "" {
		if plain, err := r.encryptor.Decrypt(*entity.TotpSecretEncrypted); err == nil {
			provider.TotpSecret = plain
		} else {
			slog.Error("upstream_provider_totp_decrypt_failed", "provider_id", entity.ID, "error", err)
		}
	}
	if entity.TokenEncrypted != nil && *entity.TokenEncrypted != "" {
		if plain, err := r.encryptor.Decrypt(*entity.TokenEncrypted); err == nil {
			provider.Token = plain
		} else {
			slog.Warn("upstream_provider_token_decrypt_failed", "provider_id", entity.ID, "error", err)
		}
	}
	if entity.RefreshTokenEncrypted != nil && *entity.RefreshTokenEncrypted != "" {
		if plain, err := r.encryptor.Decrypt(*entity.RefreshTokenEncrypted); err == nil {
			provider.RefreshToken = plain
		} else {
			slog.Warn("upstream_provider_refresh_token_decrypt_failed", "provider_id", entity.ID, "error", err)
		}
	}
	return provider
}

func upstreamGroupEntityToService(entity *dbent.UpstreamGroup) *service.UpstreamGroup {
	if entity == nil {
		return nil
	}
	return &service.UpstreamGroup{
		ID:                      entity.ID,
		UpstreamProviderID:      entity.UpstreamProviderID,
		RemoteGroupID:           entity.RemoteGroupID,
		Name:                    entity.Name,
		Platform:                entity.Platform,
		SubscriptionType:        entity.SubscriptionType,
		RateMultiplier:          entity.RateMultiplier,
		EffectiveRateMultiplier: entity.EffectiveRateMultiplier,
		PeakRateEnabled:         entity.PeakRateEnabled,
		PeakRateMultiplier:      entity.PeakRateMultiplier,
		PeakStart:               entity.PeakStart,
		PeakEnd:                 entity.PeakEnd,
		DailyLimitUSD:           entity.DailyLimitUsd,
		WeeklyLimitUSD:          entity.WeeklyLimitUsd,
		MonthlyLimitUSD:         entity.MonthlyLimitUsd,
		SyncedAt:                entity.SyncedAt,
		CreatedAt:               entity.CreatedAt,
		UpdatedAt:               entity.UpdatedAt,
	}
}
