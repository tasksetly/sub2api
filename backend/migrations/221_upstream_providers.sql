-- 上游 sub2api 供应商管理。
--
-- upstream_providers 存能登录上游后台的账号，用于自动拉取分组倍率、余额、
-- 并发限制，并直接在上游创建 API Key。密码使用 AES-256-GCM 可逆加密
-- （见 internal/repository/aes_encryptor.go）：token 过期后需要用原密码重新
-- 登录，所以不能用 bcrypt。
CREATE TABLE IF NOT EXISTS upstream_providers (
    id                    BIGSERIAL PRIMARY KEY,
    name                  VARCHAR(100) NOT NULL,
    base_url              VARCHAR(255) NOT NULL,
    notes                 TEXT,

    -- 登录凭据
    username              VARCHAR(255) NOT NULL,
    password_encrypted    TEXT NOT NULL,
    totp_secret_encrypted TEXT,

    -- 会话缓存，避免每次同步都重新登录
    token_encrypted       TEXT,
    token_expires_at      TIMESTAMPTZ,

    -- 同步来的只读快照，仅用于展示比价，不自动写回本地 accounts
    balance               DECIMAL(20,8),
    frozen_balance        DECIMAL(20,8),
    upstream_concurrency  INTEGER,
    upstream_user_id      VARCHAR(64),

    -- 同步状态
    status                VARCHAR(20) NOT NULL DEFAULT 'active',
    last_sync_at          TIMESTAMPTZ,
    last_sync_error       TEXT,
    sync_enabled          BOOLEAN NOT NULL DEFAULT TRUE,

    created_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at            TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_upstream_providers_status
    ON upstream_providers (status);
CREATE INDEX IF NOT EXISTS idx_upstream_providers_deleted_at
    ON upstream_providers (deleted_at);
CREATE INDEX IF NOT EXISTS idx_upstream_providers_sync_enabled
    ON upstream_providers (sync_enabled);

-- 软删除后允许同名复用，与 groups/proxies 的部分唯一索引口径一致
-- （见 016_soft_delete_partial_unique_indexes.sql）
CREATE UNIQUE INDEX IF NOT EXISTS idx_upstream_providers_name_unique
    ON upstream_providers (name)
    WHERE deleted_at IS NULL;

-- upstream_groups 是上游分组的只读镜像，每次同步整体覆盖。
-- 不参与本地调度，也不自动写回本地 groups 表。
CREATE TABLE IF NOT EXISTS upstream_groups (
    id                        BIGSERIAL PRIMARY KEY,
    upstream_provider_id      BIGINT NOT NULL
        REFERENCES upstream_providers (id) ON DELETE CASCADE,
    -- 分组在上游的主键，创建 API Key 时要回传给上游
    remote_group_id           BIGINT NOT NULL,
    name                      VARCHAR(100) NOT NULL,
    platform                  VARCHAR(50) NOT NULL DEFAULT '',
    subscription_type         VARCHAR(20) NOT NULL DEFAULT '',

    -- 比价核心字段。effective_rate_multiplier 叠加了用户专属倍率与高峰倍率，
    -- 来自 GET /groups/rates，是真正该用来比价的值。
    rate_multiplier           DECIMAL(10,4) NOT NULL DEFAULT 1.0,
    effective_rate_multiplier DECIMAL(10,4),
    peak_rate_enabled         BOOLEAN NOT NULL DEFAULT FALSE,
    peak_rate_multiplier      DECIMAL(10,4),
    peak_start                VARCHAR(5) NOT NULL DEFAULT '',
    peak_end                  VARCHAR(5) NOT NULL DEFAULT '',

    -- 限额
    daily_limit_usd           DECIMAL(20,8),
    weekly_limit_usd          DECIMAL(20,8),
    monthly_limit_usd         DECIMAL(20,8),

    synced_at                 TIMESTAMPTZ NOT NULL,
    created_at                TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_upstream_groups_provider
    ON upstream_groups (upstream_provider_id);
-- 同步时按 (provider, remote_group_id) upsert
CREATE UNIQUE INDEX IF NOT EXISTS idx_upstream_groups_provider_remote_unique
    ON upstream_groups (upstream_provider_id, remote_group_id);

-- accounts 关联到签发其 API Key 的上游供应商，用于把本地 usage_log 的实际
-- 花费归集到具体上游做比价。比 supplier 字符串匹配可靠：上游改名不会断开关联。
-- ON DELETE SET NULL：删上游不应连带删掉还在用的账号。
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS upstream_provider_id BIGINT;

DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'fk_accounts_upstream_provider'
    ) THEN
        ALTER TABLE accounts
            ADD CONSTRAINT fk_accounts_upstream_provider
            FOREIGN KEY (upstream_provider_id)
            REFERENCES upstream_providers (id) ON DELETE SET NULL;
    END IF;
END $$;

CREATE INDEX IF NOT EXISTS idx_accounts_upstream_provider_id
    ON accounts (upstream_provider_id);
