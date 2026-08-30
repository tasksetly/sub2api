-- accounts 记下 Key 绑定在上游的哪个分组。
--
-- 存的是上游侧的分组 id（不是本地 groups.id），所以不加外键——上游分组快照
-- 会被同步整体覆盖，硬外键会在上游删分组时挡住写入。
--
-- 有了这一列，比价表的「已建号」才能按分组粒度统计：只有
-- upstream_provider_id 时，同一上游的所有分组行都会显示相同的账号总数。
ALTER TABLE accounts ADD COLUMN IF NOT EXISTS upstream_remote_group_id BIGINT;

-- 复合索引服务比价表按 (上游, 上游分组) 的分组统计
CREATE INDEX IF NOT EXISTS idx_accounts_upstream_provider_remote_group
    ON accounts (upstream_provider_id, upstream_remote_group_id);
