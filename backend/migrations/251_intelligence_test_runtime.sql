ALTER TABLE intelligence_test_questions
    ADD COLUMN IF NOT EXISTS api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS cron_expression VARCHAR(100) NOT NULL DEFAULT '0 * * * *',
    ADD COLUMN IF NOT EXISTS keep_count INTEGER NOT NULL DEFAULT 20,
    ADD COLUMN IF NOT EXISTS last_run_at TIMESTAMPTZ,
    ADD COLUMN IF NOT EXISTS next_run_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_intelligence_test_questions_due
    ON intelligence_test_questions (enabled, next_run_at);

CREATE TABLE IF NOT EXISTS intelligence_test_results (
    id BIGSERIAL PRIMARY KEY,
    question_id BIGINT NOT NULL REFERENCES intelligence_test_questions(id) ON DELETE CASCADE,
    api_key_id BIGINT REFERENCES api_keys(id) ON DELETE SET NULL,
    group_id BIGINT REFERENCES groups(id) ON DELETE SET NULL,
    group_name VARCHAR(200) NOT NULL DEFAULT '',
    model_id VARCHAR(200) NOT NULL DEFAULT '',
    status VARCHAR(32) NOT NULL DEFAULT 'failed',
    check_ok BOOLEAN NOT NULL DEFAULT FALSE,
    svg_content TEXT NOT NULL DEFAULT '',
    response_text TEXT NOT NULL DEFAULT '',
    error_message TEXT NOT NULL DEFAULT '',
    latency_ms BIGINT NOT NULL DEFAULT 0,
    started_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_intelligence_test_results_question_created
    ON intelligence_test_results (question_id, created_at DESC);
