export type AffiliatePayoutNetwork = 'polygon' | 'bsc'

export interface AffiliatePayoutInput {
  network: AffiliatePayoutNetwork
  wallet_address: string
  tx_hash: string
  exchange_rate: number
  note: string
}

export interface AffiliatePayout extends AffiliatePayoutInput {
  usdt_amount: number
  ticket_id: number
}

/** transfer = user moved quota into balance; withdraw = admin-recorded offline withdrawal. */
export type AffiliateOutflowAction = 'transfer' | 'withdraw'

/** One affiliate quota outflow: a transfer into balance or an offline withdrawal. */
export interface AffiliateTransferRecord {
  payout?: AffiliatePayout | null
  ledger_id: number
  action: AffiliateOutflowAction
  user_id: number
  user_email: string
  username: string
  amount: number
  balance_after?: number | null
  available_quota_after?: number | null
  frozen_quota_after?: number | null
  history_quota_after?: number | null
  snapshot_available: boolean
  created_at: string
}
