-- Group-level public model aliases.
ALTER TABLE groups
    ADD COLUMN IF NOT EXISTS model_mapping JSONB NOT NULL DEFAULT '{}'::jsonb;
