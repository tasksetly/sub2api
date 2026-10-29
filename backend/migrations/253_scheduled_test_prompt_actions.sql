-- Configurable prompt/answer validation and account actions for scheduled tests.

ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS prompt_text TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS expected_answer TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS failure_remove_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS failure_disable BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS failure_temp_unschedulable_minutes INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN IF NOT EXISTS success_add_group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS success_enable BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS success_recover BOOLEAN NOT NULL DEFAULT false;

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS check_ok BOOLEAN NOT NULL DEFAULT true;
