CREATE TABLE IF NOT EXISTS cctest_runs (
    id UUID PRIMARY KEY,
    owner_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    account_id BIGINT,
    api_key_id BIGINT,
    target_name TEXT NOT NULL,
    model TEXT NOT NULL,
    token_audit BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','starting','running','done','error','cancelled')),
    task_id TEXT NOT NULL DEFAULT '',
    relay_hash TEXT NOT NULL UNIQUE,
    credentials TEXT NOT NULL,
    relay_url TEXT NOT NULL,
    request_count INTEGER NOT NULL DEFAULT 0,
    result JSONB NOT NULL DEFAULT '{}',
    error TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT NOW() + INTERVAL '30 minutes',
    next_poll_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    lease_id TEXT NOT NULL DEFAULT '',
    lease_until TIMESTAMPTZ,
    CHECK ((account_id IS NULL) <> (api_key_id IS NULL))
);
CREATE INDEX IF NOT EXISTS cctest_runs_owner_created ON cctest_runs(owner_id, created_at DESC);
CREATE INDEX IF NOT EXISTS cctest_runs_pending ON cctest_runs(next_poll_at) WHERE status IN ('queued','starting','running');
