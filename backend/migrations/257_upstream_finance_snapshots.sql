-- Financial snapshots are separate from credentials and request billing.
-- Keep observed NewAPI orders locally: its user endpoint may only expose 30 days.
CREATE TABLE IF NOT EXISTS upstream_provider_finance (
    upstream_provider_id BIGINT PRIMARY KEY REFERENCES upstream_providers(id) ON DELETE CASCADE,
    snapshot JSONB NOT NULL,
    synced_at TIMESTAMPTZ NOT NULL
);
