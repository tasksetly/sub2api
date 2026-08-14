import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTicketNotificationStore } from '@/stores/ticketNotifications'

const { getPendingCount } = vi.hoisted(() => ({
  getPendingCount: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  ticketsAPI: {
    getPendingCount
  }
}))

describe('useTicketNotificationStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    getPendingCount.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads the pending admin count and exposes the attention state', async () => {
    getPendingCount.mockResolvedValue(2)
    const store = useTicketNotificationStore()

    await expect(store.fetchPendingTicketCount()).resolves.toBe(2)

    expect(store.pendingTicketCount).toBe(2)
    expect(store.hasPendingTickets).toBe(true)
    expect(store.loading).toBe(false)
  })

  it('uses the cache, deduplicates active requests, and allows forced refresh', async () => {
    let resolveRequest!: (count: number) => void
    getPendingCount.mockReturnValue(new Promise<number>((resolve) => {
      resolveRequest = resolve
    }))
    const store = useTicketNotificationStore()

    const first = store.fetchPendingTicketCount()
    const duplicate = store.fetchPendingTicketCount(true)
    expect(getPendingCount).toHaveBeenCalledTimes(1)
    resolveRequest(1)
    await expect(Promise.all([first, duplicate])).resolves.toEqual([1, 1])

    await expect(store.fetchPendingTicketCount()).resolves.toBe(1)
    expect(getPendingCount).toHaveBeenCalledTimes(1)

    getPendingCount.mockResolvedValue(0)
    await expect(store.fetchPendingTicketCount(true)).resolves.toBe(0)
    expect(getPendingCount).toHaveBeenCalledTimes(2)
    expect(store.hasPendingTickets).toBe(false)
  })

  it('polls once per minute and reset stops polling and clears state', async () => {
    getPendingCount.mockResolvedValue(3)
    const store = useTicketNotificationStore()
    await store.fetchPendingTicketCount()

    store.startPolling()
    store.startPolling()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(getPendingCount).toHaveBeenCalledTimes(2)

    store.reset()
    expect(store.pendingTicketCount).toBe(0)
    expect(store.hasPendingTickets).toBe(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(getPendingCount).toHaveBeenCalledTimes(2)
  })

  it('keeps the last known count when refresh fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    getPendingCount.mockResolvedValueOnce(4).mockRejectedValueOnce(new Error('network'))
    const store = useTicketNotificationStore()
    await store.fetchPendingTicketCount()

    await expect(store.fetchPendingTicketCount(true)).resolves.toBe(4)
    expect(store.pendingTicketCount).toBe(4)
    expect(consoleError).toHaveBeenCalled()
    consoleError.mockRestore()
  })
})
