-- Add NewAPI as a separately selectable upstream management protocol.
-- Existing rows remain sub2api-compatible through the default.
ALTER TABLE upstream_providers
    ADD COLUMN IF NOT EXISTS provider_type VARCHAR(20) NOT NULL DEFAULT 'sub2api';

-- Token-only NewAPI providers do not have a login username.
ALTER TABLE upstream_providers
    ALTER COLUMN username DROP NOT NULL;

-- NewAPI groups are named strings rather than numeric sub2api group IDs. Keep the
-- existing numeric column for API compatibility and persist the original key here.
ALTER TABLE upstream_groups
    ADD COLUMN IF NOT EXISTS remote_group_name VARCHAR(150) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_upstream_providers_provider_type
    ON upstream_providers (provider_type);
