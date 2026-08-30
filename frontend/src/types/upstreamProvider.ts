/**
 * 上游 sub2api 供应商管理类型。
 *
 * 上游本身也是 sub2api 实例，所以这里的字段与本仓库用户侧接口对齐。
 * 安全约定：后端绝不回传明文密码/token，只给 has_password / has_totp_secret 布尔位。
 */

export interface UpstreamProvider {
  id: number
  name: string
  base_url: string
  notes: string | null

  username: string
  has_password: boolean
  has_totp_secret: boolean

  /** 上游账户余额（同步来的只读快照） */
  balance: number | null
  frozen_balance: number | null
  /** 上游账户的并发限制 */
  upstream_concurrency: number | null
  upstream_user_id?: string

  status: 'active' | 'inactive'
  last_sync_at: string | null
  /** 上次同步失败原因；空表示成功。验证码/2FA/密码错误都会落在这里 */
  last_sync_error?: string
  sync_enabled: boolean

  created_at: string
  updated_at: string
}

/** 列表页的聚合视图：上游快照 + 本地实际用量成本 */
export interface UpstreamProviderWithStats extends UpstreamProvider {
  account_count: number
  group_count: number
  /** 本地按该上游归集的实际花费（usage_logs.total_cost） */
  local_cost_usd: number
  local_requests: number
  /** 该上游各分组比价倍率的区间 */
  min_rate: number | null
  max_rate: number | null
}

/** 上游分组快照，用于横向比价 */
export interface UpstreamGroup {
  id: number
  upstream_provider_id: number
  /** 分组在上游的主键，创建 API Key 时要回传给上游 */
  remote_group_id: number
  name: string
  platform: string
  subscription_type: string

  rate_multiplier: number
  /** 叠加了专属倍率后的实际倍率；null 表示无覆盖 */
  effective_rate_multiplier: number | null
  /** 该用来排序比价的值：有专属倍率用它，否则用基础倍率 */
  comparable_rate: number
  peak_rate_enabled: boolean
  peak_rate_multiplier: number | null
  peak_start?: string
  peak_end?: string

  daily_limit_usd: number | null
  weekly_limit_usd: number | null
  monthly_limit_usd: number | null

  synced_at: string
}

/**
 * 跨上游拉平的分组比价视图。
 *
 * 带上游名称和余额：倍率再低，上游没余额也用不了，两者要一起看。
 */
export interface UpstreamGroupComparison extends UpstreamGroup {
  provider_name: string
  provider_status: 'active' | 'inactive'
  provider_balance: number | null
  provider_sync_enabled: boolean
  /** >0 说明这个价位本地已经在用了 */
  local_account_count: number
}

/** 测试连接返回的上游账户信息 */
export interface UpstreamProfile {
  email: string
  username: string
  balance: number
  frozen_balance: number
  concurrency: number
  status: string
}

export interface CreateUpstreamProviderRequest {
  name: string
  base_url: string
  username: string
  password: string
  totp_secret?: string
  notes?: string | null
  sync_enabled?: boolean
}

/** password/totp_secret 留空表示不修改 */
export interface UpdateUpstreamProviderRequest {
  name: string
  base_url: string
  username: string
  password?: string
  totp_secret?: string
  notes?: string | null
  status?: 'active' | 'inactive'
  sync_enabled?: boolean
}

export interface ProvisionAccountsRequest {
  /** 勾选的上游分组（remote_group_id） */
  remote_group_ids: number[]
  /** 可选：把新账号绑定到哪些本地分组 */
  local_group_ids?: number[]
  concurrency?: number
  priority?: number
  key_name_prefix?: string
}

/** 单个分组的建号结果；逐个独立，失败不影响其他 */
export interface ProvisionedAccount {
  remote_group_id: number
  group_name: string
  account_id: number
  account_name: string
  error?: string
}

export interface ProvisionAccountsResponse {
  results: ProvisionedAccount[]
}

/** 批量刷新结果 */
export interface SyncAllResult {
  succeeded: number
  failed: number
}
