ALTER TABLE scheduled_test_plans
    ADD COLUMN IF NOT EXISTS success_add_group_ids BIGINT[] NOT NULL DEFAULT '{}';

-- Keep existing single-group actions when upgrading to multiple selections.
UPDATE scheduled_test_plans
SET success_add_group_ids = ARRAY[success_add_group_id]
WHERE success_add_group_id IS NOT NULL
  AND cardinality(success_add_group_ids) = 0;
