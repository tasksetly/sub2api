<template>
  <div class="space-y-6">
    <!-- Date Range Filter -->
    <div class="card p-4">
      <div class="flex flex-wrap items-center gap-4">
        <div class="flex items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.timeRange') }}:</span>
          <DateRangePicker :start-date="startDate" :end-date="endDate" @update:startDate="$emit('update:startDate', $event)" @update:endDate="$emit('update:endDate', $event)" @change="$emit('dateRangeChange', $event)" />
        </div>
        <div class="w-full min-w-0 sm:w-56">
          <Select :model-value="apiKeyId" :options="keyOptions" :aria-label="t('dashboard.apiKeys')" @change="changeApiKey" />
          <p v-if="keyLoadFailed" role="status" class="mt-1 text-xs text-red-600 dark:text-red-400">{{ t('keys.failedToLoad') }}</p>
        </div>
        <button @click="$emit('refresh')" :disabled="loading" class="btn btn-secondary">
          {{ t('common.refresh') }}
        </button>
        <div class="ml-auto flex items-center gap-2">
          <span class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.granularity') }}:</span>
          <div class="w-28">
            <Select :model-value="granularity" :options="[{value:'day', label:t('dashboard.day')}, {value:'hour', label:t('dashboard.hour')}]" @update:model-value="$emit('update:granularity', $event)" @change="$emit('granularityChange')" />
          </div>
        </div>
      </div>
    </div>

    <section class="border-y border-gray-200 px-4 py-4 dark:border-dark-700" :aria-busy="loading">
      <dl class="grid gap-4 sm:grid-cols-3 sm:gap-6">
        <div>
          <dt class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('usage.cacheHitRate') }}</dt>
          <dd data-test="cache-hit-rate" class="mt-1 text-3xl font-semibold tabular-nums text-emerald-700 dark:text-emerald-400">
            {{ showCacheStats && hitRate !== null ? percentFormatter.format(hitRate) : '–' }}
          </dd>
          <progress v-if="showCacheStats && hitRate !== null" :value="hitRate" :max="1" :aria-label="t('usage.cacheHitRate')" class="mt-2 block h-1.5 w-full max-w-40 accent-emerald-600" />
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.cacheReadTokens') }}</dt>
          <dd class="mt-1 break-words text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ showCacheStats ? formatNumber(cacheTotals.read) : '–' }}</dd>
        </div>
        <div>
          <dt class="text-sm font-medium text-gray-700 dark:text-gray-300">{{ t('dashboard.promptTokens') }}</dt>
          <dd class="mt-1 break-words text-xl font-semibold tabular-nums text-gray-900 dark:text-white">{{ showCacheStats ? formatNumber(cacheTotals.prompt) : '–' }}</dd>
        </div>
      </dl>
      <p v-if="!loading && loadFailed" role="status" class="mt-2 text-sm text-red-600 dark:text-red-400">{{ t('dashboard.chartLoadFailed') }}</p>
      <p v-else-if="!loading && hitRate === null" class="mt-2 text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</p>
    </section>

    <!-- Charts Grid -->
    <div class="grid grid-cols-1 gap-6 lg:grid-cols-2">
      <!-- Model Distribution Chart -->
      <div class="card relative overflow-hidden p-4">
        <div v-if="loading" class="absolute inset-0 z-10 flex items-center justify-center bg-white/50 backdrop-blur-sm dark:bg-dark-800/50">
          <LoadingSpinner size="md" />
        </div>
        <h3 class="mb-4 text-sm font-semibold text-gray-900 dark:text-white">{{ t('dashboard.modelDistribution') }}</h3>
        <div class="flex flex-col items-center gap-4 sm:flex-row sm:gap-6">
          <div class="h-48 w-48 shrink-0">
            <Doughnut v-if="modelData" :data="modelData" :options="doughnutOptions" />
            <div v-else class="flex h-full items-center justify-center text-sm text-gray-500 dark:text-gray-400">{{ t('dashboard.noDataAvailable') }}</div>
          </div>
          <div class="max-h-48 w-full min-w-0 flex-1 overflow-auto">
            <table class="w-full text-xs">
              <thead>
                <tr class="text-gray-500 dark:text-gray-400">
                  <th class="pb-2 text-left">{{ t('dashboard.model') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.requests') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.tokens') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.actual') }}</th>
                  <th class="pb-2 text-right">{{ t('dashboard.standard') }}</th>
                </tr>
              </thead>
              <tbody>
                <tr v-for="model in models" :key="model.model" class="border-t border-gray-100 dark:border-dark-700">
                  <td class="max-w-[100px] truncate py-1.5 font-medium text-gray-900 dark:text-white" :title="model.model">{{ model.model }}</td>
                  <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatNumber(model.requests) }}</td>
                  <td class="py-1.5 text-right text-gray-600 dark:text-gray-400">{{ formatTokens(model.total_tokens) }}</td>
                  <td class="py-1.5 text-right text-green-600 dark:text-green-400">￥{{ formatCost(model.actual_cost) }}</td>
                  <td class="py-1.5 text-right text-gray-400 dark:text-gray-500">${{ formatCost(model.cost) }}</td>
                </tr>
              </tbody>
            </table>
          </div>
        </div>
      </div>

      <!-- Token Usage Trend Chart -->
      <TokenUsageTrend :trend-data="trend" :loading="loading" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import LoadingSpinner from '@/components/common/LoadingSpinner.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select from '@/components/common/Select.vue'
import { Doughnut } from 'vue-chartjs'
import TokenUsageTrend from '@/components/charts/TokenUsageTrend.vue'
import type { ApiKey, TrendDataPoint, ModelStat } from '@/types'
import { formatCostFixed as formatCost, formatNumberLocaleString as formatNumber, formatTokensK as formatTokens } from '@/utils/format'
import { Chart as ChartJS, CategoryScale, LinearScale, PointElement, LineElement, ArcElement, Title, Tooltip, Legend, Filler } from 'chart.js'
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, ArcElement, Title, Tooltip, Legend, Filler)

const props = defineProps<{
  loading: boolean
  loadFailed: boolean
  keyLoadFailed: boolean
  startDate: string
  endDate: string
  granularity: 'day' | 'hour'
  trend: TrendDataPoint[]
  models: ModelStat[]
  apiKeyId: number | null
  apiKeys: Pick<ApiKey, 'id' | 'name'>[]
}>()
const emit = defineEmits(['update:startDate', 'update:endDate', 'update:granularity', 'update:apiKeyId', 'dateRangeChange', 'granularityChange', 'apiKeyChange', 'refresh'])
const { t, locale } = useI18n()

const keyOptions = computed(() => [
  { value: null, label: t('usage.allApiKeys') },
  ...props.apiKeys.map(({ id, name }) => ({ value: id, label: `${name} (#${id})` }))
])

const changeApiKey = (value: string | number | boolean | null) => {
  if (value !== null && typeof value !== 'number') return
  emit('update:apiKeyId', value)
  emit('apiKeyChange')
}

const cacheTotals = computed(() => props.trend.reduce((totals, point) => {
  totals.read += point.cache_read_tokens
  totals.prompt += point.input_tokens + point.cache_read_tokens + point.cache_creation_tokens
  return totals
}, { read: 0, prompt: 0 }))
const hitRate = computed(() => cacheTotals.value.prompt > 0 ? cacheTotals.value.read / cacheTotals.value.prompt : null)
const showCacheStats = computed(() => !props.loading && !props.loadFailed && hitRate.value !== null)
const percentFormatter = computed(() => new Intl.NumberFormat(locale.value, { style: 'percent', maximumFractionDigits: 2 }))

const modelData = computed(() => !props.models?.length ? null : {
  labels: props.models.map((m: ModelStat) => m.model),
  datasets: [{
    data: props.models.map((m: ModelStat) => m.total_tokens),
    backgroundColor: ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#06b6d4', '#84cc16']
  }]
})

const doughnutOptions = {
  responsive: true,
  maintainAspectRatio: false,
  plugins: {
    legend: { display: false },
    tooltip: {
      callbacks: {
        label: (context: any) => `${context.label}: ${formatTokens(context.parsed)} tokens`
      }
    }
  }
}
</script>
