CREATE TABLE IF NOT EXISTS scheduled_test_prompt_templates (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(200) NOT NULL,
    prompt_text TEXT NOT NULL,
    expected_answer TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_stpt_updated_at
    ON scheduled_test_prompt_templates(updated_at DESC, id DESC);
