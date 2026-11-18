<template>
  <AppLayout>
    <div class="space-y-6">
      <div v-if="loading" class="flex items-center justify-center py-12"><LoadingSpinner /></div>
      <template v-else-if="stats">
        <UserDashboardStats :stats="stats" :balance="user?.balance || 0" :is-simple="authStore.isSimpleMode" :platform-quotas="platformQuotas" />
        <UserDashboardCharts
          v-model:startDate="startDate"
          v-model:endDate="endDate"
          v-model:granularity="granularity"
          v-model:apiKeyId="apiKeyId"
          :api-keys="apiKeys"
          :key-load-failed="keyLoadFailed"
          :loading="loadingCharts"
          :load-failed="chartLoadFailed"
          :trend="trendData"
          :models="modelStats"
          @dateRangeChange="loadCharts"
          @granularityChange="loadCharts"
          @apiKeyChange="loadCharts"
          @refresh="refreshAll"
        />
        <div class="grid grid-cols-1 gap-6 lg:grid-cols-3">
          <div class="lg:col-span-2"><UserDashboardRecentUsage :data="recentUsage" :loading="loadingUsage" /></div>
          <div class="lg:col-span-1"><UserDashboardQuickActions /></div>
        </div>
      </template>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'; import { useAuthStore } from '@/stores/auth'; import { usageAPI, type UserDashboardStats as UserStatsType } from '@/api/usage'
import AppLayout from '@/components/layout/AppLayout.vue'; import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import UserDashboardStats from '@/components/user/dashboard/UserDashboardStats.vue'; import UserDashboardCharts from '@/components/user/dashboard/UserDashboardCharts.vue'
import UserDashboardRecentUsage from '@/components/user/dashboard/UserDashboardRecentUsage.vue'; import UserDashboardQuickActions from '@/components/user/dashboard/UserDashboardQuickActions.vue'
import type { ApiKey, UsageLog, TrendDataPoint, ModelStat, PlatformQuotaItem } from '@/types'
import { getMyPlatformQuotas } from '@/api/user'
import { formatDateLocalInput } from '@/utils/format'
import { keysAPI } from '@/api/keys'

const authStore = useAuthStore(); const user = computed(() => authStore.user)
const stats = ref<UserStatsType | null>(null); const loading = ref(false); const loadingUsage = ref(false); const loadingCharts = ref(false)
const trendData = ref<TrendDataPoint[]>([]); const modelStats = ref<ModelStat[]>([]); const recentUsage = ref<UsageLog[]>([])
const platformQuotas = ref<PlatformQuotaItem[] | null>(null)

const startDate = ref(formatDateLocalInput(new Date(Date.now() - 6 * 86400000))); const endDate = ref(formatDateLocalInput(new Date())); const granularity = ref<'day' | 'hour'>('day')
const apiKeyId = ref<number | null>(null)
const apiKeys = ref<Pick<ApiKey, 'id' | 'name'>[]>([])
const chartLoadFailed = ref(false)
const keyLoadFailed = ref(false)
let chartRequest = 0

const loadStats = async () => { loading.value = true; try { await authStore.refreshUser(); stats.value = await usageAPI.getDashboardStats() } catch (error) { console.error('Failed to load dashboard stats:', error) } finally { loading.value = false } }
const loadCharts = async () => {
  const request = ++chartRequest
  loadingCharts.value = true
  chartLoadFailed.value = false
  try {
    const data = await usageAPI.getDashboardSnapshotV2({
      start_date: startDate.value,
      end_date: endDate.value,
      granularity: granularity.value,
      api_key_id: apiKeyId.value ?? undefined
    })
    if (request !== chartRequest) return
    trendData.value = data.trend ?? []
    modelStats.value = data.models ?? []
  } catch (error) {
    if (request !== chartRequest) return
    trendData.value = []
    modelStats.value = []
    chartLoadFailed.value = true
    console.error('Failed to load charts:', error)
  } finally {
    if (request === chartRequest) loadingCharts.value = false
  }
}

const loadApiKeys = async () => {
  keyLoadFailed.value = false
  try {
    apiKeys.value = await keysAPI.listOptions()
    if (apiKeyId.value !== null && !apiKeys.value.some(({ id }) => id === apiKeyId.value)) {
      apiKeyId.value = null
      await loadCharts()
    }
  } catch (error) {
    keyLoadFailed.value = true
    console.error('Failed to load API keys:', error)
  }
}
const loadRecent = async () => { loadingUsage.value = true; try { const res = await usageAPI.getByDateRange(startDate.value, endDate.value); recentUsage.value = res.items.slice(0, 5) } catch (error) { console.error('Failed to load recent usage:', error) } finally { loadingUsage.value = false } }
const loadPlatformQuotas = async () => { try { const data = await getMyPlatformQuotas(); platformQuotas.value = data.platform_quotas ?? [] } catch (error) { console.warn('Failed to load platform quotas:', error); platformQuotas.value = [] } }
const refreshAll = () => { loadStats(); loadCharts(); loadRecent(); loadPlatformQuotas(); loadApiKeys() }

onMounted(() => { refreshAll() })
</script>
