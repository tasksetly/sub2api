ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS temp_unschedulable_enabled BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN IF NOT EXISTS temp_unschedulable_error_codes INTEGER[] NOT NULL DEFAULT '{}',
    ADD COLUMN IF NOT EXISTS temp_unschedulable_minutes INTEGER NOT NULL DEFAULT 5;

ALTER TABLE scheduled_test_results
    ADD COLUMN IF NOT EXISTS error_code INTEGER NOT NULL DEFAULT 0;
