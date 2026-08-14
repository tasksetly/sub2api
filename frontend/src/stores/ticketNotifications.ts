import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { ticketsAPI } from '@/api/admin'

const CACHE_TTL_MS = 30_000
const POLLING_INTERVAL_MS = 60_000

export const useTicketNotificationStore = defineStore('ticketNotifications', () => {
  const pendingTicketCount = ref(0)
  const loading = ref(false)
  const loaded = ref(false)
  const lastFetchedAt = ref<number | null>(null)

  let requestGeneration = 0
  let activePromise: Promise<number> | null = null
  let poller: ReturnType<typeof setInterval> | null = null

  const hasPendingTickets = computed(() => pendingTicketCount.value > 0)

  function fetchPendingTicketCount(force = false): Promise<number> {
    const now = Date.now()
    if (!force && loaded.value && lastFetchedAt.value != null && now - lastFetchedAt.value < CACHE_TTL_MS) {
      return Promise.resolve(pendingTicketCount.value)
    }
    if (activePromise) return activePromise

    const currentGeneration = ++requestGeneration
    loading.value = true
    const request = ticketsAPI.getPendingCount()
      .then((count) => {
        const normalizedCount = Number.isFinite(count) && count > 0 ? Math.floor(count) : 0
        if (currentGeneration === requestGeneration) {
          pendingTicketCount.value = normalizedCount
          loaded.value = true
          lastFetchedAt.value = Date.now()
        }
        return normalizedCount
      })
      .catch((error) => {
        console.error('Failed to fetch pending admin ticket count:', error)
        return pendingTicketCount.value
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
      void fetchPendingTicketCount(true)
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
    pendingTicketCount.value = 0
    loading.value = false
    loaded.value = false
    lastFetchedAt.value = null
    stopPolling()
  }

  return {
    pendingTicketCount,
    loading,
    hasPendingTickets,
    fetchPendingTicketCount,
    startPolling,
    stopPolling,
    reset
  }
})
