-- Keep automation state separate from credentials and Ent-generated snapshots.
CREATE TABLE IF NOT EXISTS upstream_provider_automation (
    upstream_provider_id BIGINT PRIMARY KEY REFERENCES upstream_providers(id) ON DELETE CASCADE,
    sync_cron VARCHAR(128) NOT NULL DEFAULT '*/30 * * * *',
    next_sync_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    balance_alert_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    balance_alert_threshold NUMERIC(20,8) NOT NULL DEFAULT 0 CHECK (balance_alert_threshold >= 0),
    balance_alert_email VARCHAR(254) NOT NULL DEFAULT '',
    balance_checked_at TIMESTAMPTZ,
    balance_alert_claimed_at TIMESTAMPTZ,
    balance_alert_sent_at TIMESTAMPTZ,
    balance_alert_last_error TEXT NOT NULL DEFAULT ''
);

INSERT INTO upstream_provider_automation (upstream_provider_id)
SELECT id FROM upstream_providers WHERE deleted_at IS NULL
ON CONFLICT (upstream_provider_id) DO NOTHING;
