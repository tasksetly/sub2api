-- Durable cost ledger for upstream/model/day reporting.
-- usage_logs can be removed by retention or an operator, so cost reporting must
-- not depend on that table after the request has been billed.

CREATE TABLE IF NOT EXISTS upstream_cost_entries (
    usage_log_id       BIGINT PRIMARY KEY,
    created_at         TIMESTAMPTZ NOT NULL,
    provider_id        BIGINT NOT NULL DEFAULT 0,
    provider_name      VARCHAR(200) NOT NULL DEFAULT '',
    provider_type      VARCHAR(20) NOT NULL DEFAULT '',
    supplier           VARCHAR(100) NOT NULL DEFAULT '',
    model              VARCHAR(200) NOT NULL DEFAULT '',
    user_id            BIGINT NOT NULL DEFAULT 0,
    api_key_id         BIGINT NOT NULL DEFAULT 0,
    account_id         BIGINT NOT NULL DEFAULT 0,
    group_id           BIGINT NOT NULL DEFAULT 0,
    request_type       SMALLINT NOT NULL DEFAULT 0,
    stream             BOOLEAN NOT NULL DEFAULT FALSE,
    openai_ws_mode     BOOLEAN NOT NULL DEFAULT FALSE,
    billing_type       SMALLINT NOT NULL DEFAULT 0,
    billing_mode       VARCHAR(20) NOT NULL DEFAULT '',
    image_count        INTEGER NOT NULL DEFAULT 0,
    input_tokens       BIGINT NOT NULL DEFAULT 0,
    output_tokens      BIGINT NOT NULL DEFAULT 0,
    cache_tokens       BIGINT NOT NULL DEFAULT 0,
    standard_cost      DECIMAL(20,10) NOT NULL DEFAULT 0,
    account_cost       DECIMAL(20,10) NOT NULL DEFAULT 0,
    user_billed        DECIMAL(20,10) NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_upstream_cost_entries_created_at
    ON upstream_cost_entries (created_at);
CREATE INDEX IF NOT EXISTS idx_upstream_cost_entries_provider_model
    ON upstream_cost_entries (provider_id, model, created_at);

CREATE TABLE IF NOT EXISTS upstream_cost_daily_rollups (
    bucket_date        DATE NOT NULL,
    dimension_hash     CHAR(32) NOT NULL,
    provider_id        BIGINT NOT NULL DEFAULT 0,
    provider_name      VARCHAR(200) NOT NULL DEFAULT '',
    provider_type      VARCHAR(20) NOT NULL DEFAULT '',
    supplier           VARCHAR(100) NOT NULL DEFAULT '',
    model              VARCHAR(200) NOT NULL DEFAULT '',
    user_id            BIGINT NOT NULL DEFAULT 0,
    api_key_id         BIGINT NOT NULL DEFAULT 0,
    account_id         BIGINT NOT NULL DEFAULT 0,
    group_id           BIGINT NOT NULL DEFAULT 0,
    request_type       SMALLINT NOT NULL DEFAULT 0,
    stream             BOOLEAN NOT NULL DEFAULT FALSE,
    openai_ws_mode     BOOLEAN NOT NULL DEFAULT FALSE,
    billing_type       SMALLINT NOT NULL DEFAULT 0,
    billing_mode       VARCHAR(20) NOT NULL DEFAULT '',
    image_count        INTEGER NOT NULL DEFAULT 0,
    requests           BIGINT NOT NULL DEFAULT 0,
    input_tokens       BIGINT NOT NULL DEFAULT 0,
    output_tokens      BIGINT NOT NULL DEFAULT 0,
    cache_tokens       BIGINT NOT NULL DEFAULT 0,
    standard_cost      DECIMAL(20,10) NOT NULL DEFAULT 0,
    account_cost       DECIMAL(20,10) NOT NULL DEFAULT 0,
    user_billed        DECIMAL(20,10) NOT NULL DEFAULT 0,
    computed_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (bucket_date, dimension_hash)
);

CREATE INDEX IF NOT EXISTS idx_upstream_cost_daily_provider_date
    ON upstream_cost_daily_rollups (provider_id, bucket_date);
CREATE INDEX IF NOT EXISTS idx_upstream_cost_daily_supplier_date
    ON upstream_cost_daily_rollups (supplier, bucket_date);

CREATE TABLE IF NOT EXISTS upstream_cost_ledger_state (
    id                 SMALLINT PRIMARY KEY CHECK (id = 1),
    watermark          TIMESTAMPTZ NOT NULL DEFAULT TIMESTAMPTZ 'epoch',
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
INSERT INTO upstream_cost_ledger_state (id)
VALUES (1)
ON CONFLICT (id) DO NOTHING;

-- Capture the account/provider/model and pricing snapshots at billing time.
-- The trigger is intentionally part of the usage-log transaction: if the
-- durable snapshot cannot be written, the usage log cannot be committed either.
CREATE OR REPLACE FUNCTION capture_upstream_cost_entry()
RETURNS TRIGGER
LANGUAGE plpgsql
AS $$
BEGIN
    INSERT INTO upstream_cost_entries (
        usage_log_id, created_at, provider_id, provider_name, provider_type,
        supplier, model, user_id, api_key_id, account_id, group_id,
        request_type, stream, openai_ws_mode, billing_type, billing_mode, image_count,
        input_tokens, output_tokens, cache_tokens,
        standard_cost, account_cost, user_billed
    )
    SELECT
        NEW.id,
        NEW.created_at,
        COALESCE(a.upstream_provider_id, 0),
        COALESCE(NULLIF(BTRIM(p.name), ''), NULLIF(BTRIM(a.supplier), ''), ''),
        COALESCE(p.provider_type, ''),
        COALESCE(NULLIF(BTRIM(a.supplier), ''), ''),
        COALESCE(NULLIF(BTRIM(NEW.upstream_model), ''), NULLIF(BTRIM(NEW.model), ''), ''),
        NEW.user_id,
        NEW.api_key_id,
        NEW.account_id,
        COALESCE(NEW.group_id, 0),
        NEW.request_type,
        NEW.stream,
        COALESCE(NEW.openai_ws_mode, FALSE),
        NEW.billing_type,
        COALESCE(NEW.billing_mode, ''),
        COALESCE(NEW.image_count, 0),
        NEW.input_tokens,
        NEW.output_tokens,
        NEW.cache_creation_tokens + NEW.cache_read_tokens,
        NEW.total_cost,
        COALESCE(NEW.account_stats_cost, NEW.total_cost) * COALESCE(NEW.account_rate_multiplier, 1),
        NEW.actual_cost
    FROM accounts a
    LEFT JOIN upstream_providers p ON p.id = a.upstream_provider_id
    WHERE a.id = NEW.account_id
    ON CONFLICT (usage_log_id) DO NOTHING;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS trg_capture_upstream_cost_entry ON usage_logs;
CREATE TRIGGER trg_capture_upstream_cost_entry
AFTER INSERT ON usage_logs
FOR EACH ROW EXECUTE FUNCTION capture_upstream_cost_entry();

-- Backfill before the trigger is needed for existing installations. The
-- conflict guard keeps this migration idempotent and never double-counts logs.
INSERT INTO upstream_cost_entries (
    usage_log_id, created_at, provider_id, provider_name, provider_type,
    supplier, model, user_id, api_key_id, account_id, group_id,
    request_type, stream, openai_ws_mode, billing_type, billing_mode, image_count,
    input_tokens, output_tokens, cache_tokens,
    standard_cost, account_cost, user_billed
)
SELECT
    ul.id,
    ul.created_at,
    COALESCE(a.upstream_provider_id, 0),
    COALESCE(NULLIF(BTRIM(p.name), ''), NULLIF(BTRIM(a.supplier), ''), ''),
    COALESCE(p.provider_type, ''),
    COALESCE(NULLIF(BTRIM(a.supplier), ''), ''),
    COALESCE(NULLIF(BTRIM(ul.upstream_model), ''), NULLIF(BTRIM(ul.model), ''), ''),
    ul.user_id,
    ul.api_key_id,
    ul.account_id,
    COALESCE(ul.group_id, 0),
    ul.request_type,
    ul.stream,
    COALESCE(ul.openai_ws_mode, FALSE),
    ul.billing_type,
    COALESCE(ul.billing_mode, ''),
    COALESCE(ul.image_count, 0),
    ul.input_tokens,
    ul.output_tokens,
    ul.cache_creation_tokens + ul.cache_read_tokens,
    ul.total_cost,
    COALESCE(ul.account_stats_cost, ul.total_cost) * COALESCE(ul.account_rate_multiplier, 1),
    ul.actual_cost
FROM usage_logs ul
JOIN accounts a ON a.id = ul.account_id
LEFT JOIN upstream_providers p ON p.id = a.upstream_provider_id
ON CONFLICT (usage_log_id) DO NOTHING;
