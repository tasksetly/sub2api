-- The question bank is independent from scheduled_test_plans. Runtime fields
-- and result storage are added by 251_intelligence_test_runtime.sql.
CREATE TABLE IF NOT EXISTS intelligence_test_questions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    model_id VARCHAR(200) NOT NULL,
    question_text TEXT NOT NULL,
    expected_answer TEXT NOT NULL DEFAULT '',
    scoring_criteria TEXT NOT NULL DEFAULT '',
    answer_mode VARCHAR(20) NOT NULL DEFAULT 'svg',
    strength VARCHAR(10) NOT NULL DEFAULT 'medium',
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_intelligence_test_questions_enabled_updated
    ON intelligence_test_questions (enabled, updated_at DESC);
