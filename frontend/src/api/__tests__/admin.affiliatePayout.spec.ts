import { beforeEach, expect, it, vi } from 'vitest'
const { post } = vi.hoisted(() => ({ post: vi.fn() }))
vi.mock('@/api/client', () => ({ apiClient: { post } }))
import { withdrawUserQuota } from '@/api/admin/affiliates'

beforeEach(() => post.mockReset())

it('sends payout data and screenshots as multipart with the same idempotency key', async () => {
  post.mockResolvedValue({ data: { ledger_id: 9 }, headers: { 'x-idempotency-replayed': 'true' } })
  const payout = { network: 'bsc' as const, wallet_address: '0x' + '1'.repeat(40), tx_hash: '0x' + 'a'.repeat(64), exchange_rate: 6.8, note: 'paid' }
  const image = new File(['png'], 'proof.png', { type: 'image/png' })
  const result = await withdrawUserQuota(42, { amount: 68, payout }, 'payout-key', [image])
  const [url, form, config] = post.mock.calls[0]
  expect(url).toBe('/admin/affiliates/users/42/withdraw')
  expect(JSON.parse(form.get('payload'))).toEqual({ amount: 68, payout })
  expect(form.getAll('images')).toEqual([image])
  expect(config.headers).toMatchObject({ 'Idempotency-Key': 'payout-key', 'Content-Type': 'multipart/form-data' })
  expect(result.replayed).toBe(true)
})
