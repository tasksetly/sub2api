import { describe, expect, it } from 'vitest'
import { calculateAffiliateUSDT, affiliateTransactionURL } from '../affiliate'
import type { AffiliatePayout } from '@/types/affiliate'

describe('cashback USDT conversion', () => {
  it('divides CNY by the exchange rate and rounds payouts to 6 decimals', () => {
    expect(calculateAffiliateUSDT(68, 6.8)).toBe(10)
    expect(calculateAffiliateUSDT(10, 6.8)).toBe(1.470588)
    expect(calculateAffiliateUSDT(68.000000001, 6.8)).toBe(10)
    expect(calculateAffiliateUSDT(70, 7)).toBe(10)
    expect(calculateAffiliateUSDT(10, 0)).toBe(0)
    expect(calculateAffiliateUSDT(Number.NaN, 6.8)).toBe(0)
  })
  it('links only supported networks and valid transaction hashes', () => {
    const payout = { network: 'polygon', tx_hash: '0x' + 'a'.repeat(64) } as AffiliatePayout
    expect(affiliateTransactionURL(payout)).toBe('https://polygonscan.com/tx/' + payout.tx_hash)
    expect(affiliateTransactionURL({ ...payout, network: 'bsc' })).toBe('https://bscscan.com/tx/' + payout.tx_hash)
    expect(affiliateTransactionURL({ ...payout, tx_hash: 'javascript:alert(1)' })).toBe('')
  })
})
