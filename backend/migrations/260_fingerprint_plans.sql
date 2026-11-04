-- 260_fingerprint_plans.sql
-- 账号指纹校对：计划与检测结果
-- 设计文档：docs/superpowers/specs/2026-11-04-account-fingerprint-verification-design.md

CREATE TABLE IF NOT EXISTS fingerprint_plans (
    id                                 BIGSERIAL PRIMARY KEY,
    account_id                         BIGINT NOT NULL REFERENCES accounts(id) ON DELETE CASCADE,
    -- 发给上游的最终模型名（经本地 model_mapping 映射之后的值）
    test_model                         VARCHAR(200) NOT NULL DEFAULT '',
    -- 参考库里的规范化模型 ID，用于和 analysis.prediction 比对
    expected_model                     VARCHAR(100) NOT NULL DEFAULT '',
    -- responses | chat_completions | messages
    api                                VARCHAR(20) NOT NULL DEFAULT 'chat_completions',
    -- 单次检测的挑战数，1-3；越大越准也越贵
    challenge_count                    INT NOT NULL DEFAULT 3,
    cron_expression                    VARCHAR(100) NOT NULL DEFAULT '0 */6 * * *',
    enabled                            BOOLEAN NOT NULL DEFAULT true,
    max_results                        INT NOT NULL DEFAULT 50,
    -- 连续判定不符的次数；相符时清零，无法判定/检测失败时不动
    consecutive_mismatches             INT NOT NULL DEFAULT 0,
    -- 连续不符达到该阈值才执行动账号的动作
    mismatch_action_threshold          INT NOT NULL DEFAULT 2,
    failure_disable                    BOOLEAN NOT NULL DEFAULT false,
    failure_temp_unschedulable_minutes INT NOT NULL DEFAULT 0,
    failure_remove_group_ids           BIGINT[] NOT NULL DEFAULT '{}',
    success_add_group_ids              BIGINT[] NOT NULL DEFAULT '{}',
    last_run_at                        TIMESTAMPTZ,
    next_run_at                        TIMESTAMPTZ,
    created_at                         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at                         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fp_plans_account_id ON fingerprint_plans(account_id);
CREATE INDEX IF NOT EXISTS idx_fp_plans_enabled_next_run ON fingerprint_plans(enabled, next_run_at) WHERE enabled = true;

CREATE TABLE IF NOT EXISTS fingerprint_results (
    id                      BIGSERIAL PRIMARY KEY,
    plan_id                 BIGINT NOT NULL REFERENCES fingerprint_plans(id) ON DELETE CASCADE,
    -- match | mismatch | inconclusive | error
    verdict                 VARCHAR(20) NOT NULL DEFAULT 'error',
    -- SDK 的 decision：scored | partial | unscorable | not_model
    decision                VARCHAR(20) NOT NULL DEFAULT '',
    predicted_model         VARCHAR(100) NOT NULL DEFAULT '',
    predicted_model_name    VARCHAR(200) NOT NULL DEFAULT '',
    predicted_family        VARCHAR(100) NOT NULL DEFAULT '',
    probability             DOUBLE PRECISION,
    -- 完整的 lmfpd.Analysis，供人工复核候选排名与置信度
    analysis_json           JSONB,
    error_message           TEXT NOT NULL DEFAULT '',
    -- 被并发闸或每日配额拦下时的原因；不能静默跳过
    skip_reason             VARCHAR(100) NOT NULL DEFAULT '',
    estimated_output_tokens INT NOT NULL DEFAULT 0,
    latency_ms              BIGINT NOT NULL DEFAULT 0,
    started_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at             TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fp_results_plan_created ON fingerprint_results(plan_id, created_at DESC);
