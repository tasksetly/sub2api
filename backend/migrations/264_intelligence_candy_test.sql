-- Keep the result type with each run so historical SVG and candy rows stay distinct.
ALTER TABLE intelligence_test_results
    ADD COLUMN IF NOT EXISTS answer_mode VARCHAR(20) NOT NULL DEFAULT 'svg',
    ADD COLUMN IF NOT EXISTS response_text TEXT NOT NULL DEFAULT '';
