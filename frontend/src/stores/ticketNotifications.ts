import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ticketsAPI } from '@/api'

const CACHE_TTL_MS = 30_000
const POLLING_INTERVAL_MS = 60_000

export const useTicketNotificationStore = defineStore('ticketNotifications', () => {
  const waitingUserCount = ref(0)
  const loading = ref(false)
  const loaded = ref(false)
  const lastFetchedAt = ref<number | null>(null)

  let requestGeneration = 0
  let activePromise: Promise<number> | null = null
  let poller: ReturnType<typeof setInterval> | null = null

  const hasWaitingUserTickets = computed(() => waitingUserCount.value > 0)

  function fetchWaitingUserCount(force = false): Promise<number> {
    const now = Date.now()
    if (!force && loaded.value && lastFetchedAt.value != null && now - lastFetchedAt.value < CACHE_TTL_MS) {
      return Promise.resolve(waitingUserCount.value)
    }
    if (activePromise) return activePromise

    const currentGeneration = ++requestGeneration
    loading.value = true
    const request = ticketsAPI.getWaitingUserCount()
      .then((count) => {
        const normalizedCount = Number.isFinite(count) && count > 0 ? Math.floor(count) : 0
        if (currentGeneration === requestGeneration) {
          waitingUserCount.value = normalizedCount
          loaded.value = true
          lastFetchedAt.value = Date.now()
        }
        return normalizedCount
      })
      .catch((error) => {
        console.error('Failed to fetch waiting ticket count:', error)
        return waitingUserCount.value
      })
      .finally(() => {
        if (activePromise === request) {
          activePromise = null
          loading.value = false
        }
      })

    activePromise = request
    return request
  }

  function startPolling() {
    if (poller) return
    poller = setInterval(() => {
      void fetchWaitingUserCount(true)
    }, POLLING_INTERVAL_MS)
  }

  function stopPolling() {
    if (!poller) return
    clearInterval(poller)
    poller = null
  }

  function reset() {
    requestGeneration++
    activePromise = null
    waitingUserCount.value = 0
    loading.value = false
    loaded.value = false
    lastFetchedAt.value = null
    stopPolling()
  }

  return {
    waitingUserCount,
    loading,
    hasWaitingUserTickets,
    fetchWaitingUserCount,
    startPolling,
    stopPolling,
    reset
  }
})
