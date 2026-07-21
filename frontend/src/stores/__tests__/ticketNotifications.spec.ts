import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useTicketNotificationStore } from '@/stores/ticketNotifications'

const { getWaitingUserCount } = vi.hoisted(() => ({
  getWaitingUserCount: vi.fn()
}))

vi.mock('@/api', () => ({
  ticketsAPI: {
    getWaitingUserCount
  }
}))

describe('useTicketNotificationStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    getWaitingUserCount.mockReset()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('loads the waiting-user count and exposes the attention state', async () => {
    getWaitingUserCount.mockResolvedValue(2)
    const store = useTicketNotificationStore()

    await expect(store.fetchWaitingUserCount()).resolves.toBe(2)

    expect(store.waitingUserCount).toBe(2)
    expect(store.hasWaitingUserTickets).toBe(true)
    expect(store.loading).toBe(false)
  })

  it('uses the cache, deduplicates active requests, and allows forced refresh', async () => {
    let resolveRequest!: (count: number) => void
    getWaitingUserCount.mockReturnValue(new Promise<number>((resolve) => {
      resolveRequest = resolve
    }))
    const store = useTicketNotificationStore()

    const first = store.fetchWaitingUserCount()
    const duplicate = store.fetchWaitingUserCount(true)
    expect(getWaitingUserCount).toHaveBeenCalledTimes(1)
    resolveRequest(1)
    await expect(Promise.all([first, duplicate])).resolves.toEqual([1, 1])

    await expect(store.fetchWaitingUserCount()).resolves.toBe(1)
    expect(getWaitingUserCount).toHaveBeenCalledTimes(1)

    getWaitingUserCount.mockResolvedValue(0)
    await expect(store.fetchWaitingUserCount(true)).resolves.toBe(0)
    expect(getWaitingUserCount).toHaveBeenCalledTimes(2)
    expect(store.hasWaitingUserTickets).toBe(false)
  })

  it('polls once per minute and reset stops polling and clears state', async () => {
    getWaitingUserCount.mockResolvedValue(3)
    const store = useTicketNotificationStore()
    await store.fetchWaitingUserCount()

    store.startPolling()
    store.startPolling()
    await vi.advanceTimersByTimeAsync(60_000)
    expect(getWaitingUserCount).toHaveBeenCalledTimes(2)

    store.reset()
    expect(store.waitingUserCount).toBe(0)
    expect(store.hasWaitingUserTickets).toBe(false)
    await vi.advanceTimersByTimeAsync(60_000)
    expect(getWaitingUserCount).toHaveBeenCalledTimes(2)
  })

  it('keeps the last known count when refresh fails', async () => {
    const consoleError = vi.spyOn(console, 'error').mockImplementation(() => undefined)
    getWaitingUserCount.mockResolvedValueOnce(4).mockRejectedValueOnce(new Error('network'))
    const store = useTicketNotificationStore()
    await store.fetchWaitingUserCount()

    await expect(store.fetchWaitingUserCount(true)).resolves.toBe(4)
    expect(store.waitingUserCount).toBe(4)
    expect(consoleError).toHaveBeenCalled()
    consoleError.mockRestore()
  })
})
