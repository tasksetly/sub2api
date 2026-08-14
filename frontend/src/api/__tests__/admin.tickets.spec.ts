import { beforeEach, describe, expect, it, vi } from 'vitest'

const { get } = vi.hoisted(() => ({
  get: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: { get }
}))

import { ticketsAPI } from '@/api/admin/tickets'

describe('admin ticket notification API', () => {
  beforeEach(() => {
    get.mockReset()
  })

  it('uses the open-ticket total as the pending admin count', async () => {
    get.mockResolvedValue({
      data: {
        items: [],
        total: 3,
        page: 1,
        page_size: 1,
        pages: 3
      }
    })

    await expect(ticketsAPI.getPendingCount()).resolves.toBe(3)
    expect(get).toHaveBeenCalledWith('/admin/tickets', {
      params: { page: 1, page_size: 1, status: 'open' }
    })
  })
})
