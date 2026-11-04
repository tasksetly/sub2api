/**
 * 账号列表上的指纹标记。
 *
 * 标记按最近一次检测结果自动更新（后端写在账号 extra 里），不需要管理员手动确认——
 * 证据不会丢，历史记录在指纹面板里永久留存。
 *
 * 这一层不依赖运维监控总开关：即便 ops 告警整个关掉，打开账号列表仍看得见。
 */

export type FingerprintBadgeStatus = 'match' | 'mismatch' | 'inconclusive' | 'error' | 'unknown'

const KNOWN: FingerprintBadgeStatus[] = ['match', 'mismatch', 'inconclusive', 'error']

const CLASSES: Record<FingerprintBadgeStatus, string> = {
  mismatch: 'bg-red-50 text-red-700 dark:bg-red-900/20 dark:text-red-300',
  match: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-300',
  inconclusive: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
  error: 'bg-amber-50 text-amber-700 dark:bg-amber-900/20 dark:text-amber-300',
  unknown: ''
}

/** 账号 extra 里与指纹相关的字段。 */
export interface FingerprintExtra {
  fingerprint_status?: unknown
  fingerprint_checked_at?: unknown
  fingerprint_predicted_model?: unknown
}

/** 从账号 extra 读出标记状态；没测过或值不认识都归为 unknown。 */
export function fingerprintBadgeStatus(extra: FingerprintExtra | null | undefined): FingerprintBadgeStatus {
  const raw = extra?.fingerprint_status
  return KNOWN.includes(raw as FingerprintBadgeStatus) ? (raw as FingerprintBadgeStatus) : 'unknown'
}

/** 标记的配色。unknown 不上色，由调用方按普通文本渲染。 */
export function fingerprintBadgeClass(status: FingerprintBadgeStatus): string {
  return CLASSES[status] ?? ''
}

/**
 * 悬停提示：判成了什么模型、什么时候判的。
 * 无法判定与检测失败同样要出提示，不能只有不符才有——否则那两态在列表上是沉默的。
 */
export function fingerprintBadgeHint(
  extra: FingerprintExtra | null | undefined,
  predictedLabel: string
): string {
  const parts: string[] = []
  const predicted = extra?.fingerprint_predicted_model
  if (typeof predicted === 'string' && predicted !== '') {
    parts.push(`${predictedLabel}: ${predicted}`)
  }
  const checkedAt = extra?.fingerprint_checked_at
  if (typeof checkedAt === 'string' && checkedAt !== '') {
    const date = new Date(checkedAt)
    parts.push(Number.isNaN(date.getTime()) ? checkedAt : date.toLocaleString())
  }
  return parts.join(' · ')
}
