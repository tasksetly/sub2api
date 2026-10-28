-- Preserve the selected reasoning level and purge raw model replies saved by
-- earlier versions. New results persist only the extracted SVG fragment.
ALTER TABLE intelligence_test_questions
    ADD COLUMN IF NOT EXISTS reasoning_effort VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_name VARCHAR(200) NOT NULL DEFAULT '';

ALTER TABLE intelligence_test_results
    ADD COLUMN IF NOT EXISTS reasoning_effort VARCHAR(10) NOT NULL DEFAULT '',
    ADD COLUMN IF NOT EXISTS model_name VARCHAR(200) NOT NULL DEFAULT '';

UPDATE intelligence_test_questions SET model_name = model_id WHERE model_name = '';
UPDATE intelligence_test_results SET model_name = model_id WHERE model_name = '';

ALTER TABLE intelligence_test_results
    DROP COLUMN IF EXISTS response_text;

-- Earlier upstream errors may contain response bodies. Do not retain them.
UPDATE intelligence_test_results
SET error_message = 'upstream test failed'
WHERE error_message <> '';

-- Apply the SVG retention rule to rows written before this migration too.
WITH ranked AS (
    SELECT r.id, (r.check_ok AND r.svg_content <> '') AS is_svg,
           ROW_NUMBER() OVER (
               PARTITION BY r.question_id, (r.check_ok AND r.svg_content <> '')
               ORDER BY r.created_at DESC, r.id DESC
           ) AS rn,
           LEAST(GREATEST(q.keep_count, 1), 100) AS keep_count
    FROM intelligence_test_results r
    JOIN intelligence_test_questions q ON q.id = r.question_id
)
DELETE FROM intelligence_test_results AS result USING ranked
WHERE result.id = ranked.id
  AND ((ranked.is_svg AND ranked.rn > ranked.keep_count)
       OR (NOT ranked.is_svg AND ranked.rn > 20));
