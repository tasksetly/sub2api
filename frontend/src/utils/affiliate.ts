import type { AffiliatePayout } from '@/types/affiliate'

export function calculateAffiliateUSDT(amount: number, exchangeRate: number): number {
  const cny = Math.round(amount * 1e8) / 1e8
  const rate = Math.round(exchangeRate * 1e8) / 1e8
  if (!Number.isFinite(cny) || !Number.isFinite(rate) || cny <= 0 || rate <= 0) return 0
  return Math.round((cny / rate) * 1e6) / 1e6
}

export function affiliateTransactionURL(payout: AffiliatePayout): string {
  if (!/^0x[\da-f]{64}$/i.test(payout.tx_hash)) return ''
  const explorer = { polygon: 'https://polygonscan.com/tx/', bsc: 'https://bscscan.com/tx/' }[payout.network]
  return explorer ? explorer + payout.tx_hash : ''
}
