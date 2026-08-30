-- 上游价格倍率修正系数，用于抹平各上游充值比例的差异。
--
-- 上游声明的倍率是按它自己的站内币计的，充值比例不同就没法直接比：
-- 充值比例 10 倍、倍率 1.0 的上游，真实成本等于 1:1 充值、倍率 0.1 的上游。
-- 填 1/充值比例（10 倍 → 0.1）。
--
-- 比价倍率 = 声明倍率 × rate_correction。默认 1.0 表示 1:1 充值，不做修正，
-- 因此这次迁移对既有数据的比价结果没有影响。
ALTER TABLE upstream_providers
    ADD COLUMN IF NOT EXISTS rate_correction DECIMAL(10,6) NOT NULL DEFAULT 1.0;
