import type { AffiliatePayoutInput } from '@/types/affiliate'

/** 一笔提现登记；重试保留首次提交的金额和结算信息。 */
export interface AffiliateWithdrawOperation {
  userId: number
  amount: number
  payout?: AffiliatePayoutInput
  key: string
  storageKey: string | null
  outcomeUncertain: boolean
}

type StoredOperation = Pick<AffiliateWithdrawOperation, 'key' | 'amount' | 'payout'>
const LEDGER_SCALE = 1e8
const pendingKeys = new Map<string, StoredOperation>()

function currentAdminId(): number | null {
  try {
    const user = JSON.parse(globalThis.localStorage?.getItem('auth_user') ?? 'null') as { id?: unknown } | null
    const id = user?.id
    return typeof id === 'number' && Number.isSafeInteger(id) && id > 0 ? id : null
  } catch {
    return null
  }
}

function readStoredOperation(storageKey: string, amount: number): StoredOperation | null {
  try {
    const raw = globalThis.sessionStorage?.getItem(storageKey)
    if (!raw) return null
    // 兼容之前只保存幂等键的普通线下提现。
    if (!raw.startsWith('{')) return { key: raw, amount }
    const stored = JSON.parse(raw) as StoredOperation
    return typeof stored.key === 'string' && Number.isFinite(stored.amount) ? stored : null
  } catch {
    return null
  }
}

function storeOperation(storageKey: string, operation: StoredOperation | null) {
  try {
    if (operation) globalThis.sessionStorage?.setItem(storageKey, JSON.stringify(operation))
    else globalThis.sessionStorage?.removeItem(storageKey)
  } catch {
    // 浏览器存储不可用时，同一页面内的重试仍由内存中的键保护。
  }
}

export function prepareAffiliateWithdrawOperation(userId: number, amount: number, payout?: AffiliatePayoutInput): AffiliateWithdrawOperation {
  const normalizedAmount = Math.round(amount * LEDGER_SCALE) / LEDGER_SCALE
  const normalizedPayout = payout ? {
    ...payout,
    exchange_rate: Math.round(payout.exchange_rate * LEDGER_SCALE) / LEDGER_SCALE,
    wallet_address: payout.wallet_address.trim(),
    tx_hash: payout.tx_hash.trim().toLowerCase(),
    note: payout.note.trim(),
  } : undefined
  const adminId = currentAdminId()
  const identity = normalizedPayout ? `${normalizedPayout.network}:${normalizedPayout.tx_hash}` : String(normalizedAmount)
  const storageKey = adminId ? `sub2api:admin:affiliate-withdraw:${adminId}:${userId}:${identity}` : null
  let stored = storageKey ? pendingKeys.get(storageKey) ?? readStoredOperation(storageKey, normalizedAmount) : null
  const outcomeUncertain = !!stored
  if (!stored) {
    const requestId = globalThis.crypto?.randomUUID?.() ?? `${Date.now()}-${Math.random().toString(36).slice(2)}`
    stored = { key: `affiliate-withdraw-${adminId ?? 'unknown'}-${requestId}`, amount: normalizedAmount, payout: normalizedPayout }
  }
  if (storageKey) {
    pendingKeys.set(storageKey, stored)
    storeOperation(storageKey, stored)
  }
  return { userId, ...stored, storageKey, outcomeUncertain }
}

export function completeAffiliateWithdrawOperation(operation: AffiliateWithdrawOperation) {
  if (!operation.storageKey) return
  pendingKeys.delete(operation.storageKey)
  storeOperation(operation.storageKey, null)
}
