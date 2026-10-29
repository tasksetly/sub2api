ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS failure_remove_group_ids BIGINT[] NOT NULL DEFAULT '{}';
