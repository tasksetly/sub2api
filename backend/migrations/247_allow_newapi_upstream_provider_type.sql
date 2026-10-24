-- Allow NewAPI providers in databases that already had a legacy provider_type check.
-- Some installations created this constraint outside the current migration set and
-- allowed the historical "codego" value, so keep it valid for existing rows.
ALTER TABLE upstream_providers
    DROP CONSTRAINT IF EXISTS upstream_providers_provider_type_check;

ALTER TABLE upstream_providers
    ADD CONSTRAINT upstream_providers_provider_type_check
    CHECK (provider_type IN ('sub2api', 'newapi', 'codego'));
