-- Manual account probes share SVG storage without creating scheduled/public configs.
ALTER TABLE intelligence_test_results
    ALTER COLUMN question_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS account_id BIGINT REFERENCES accounts(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS account_name TEXT NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS prompt_text TEXT NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_intelligence_test_results_account_id
    ON intelligence_test_results (account_id, id DESC) WHERE question_id IS NULL;
