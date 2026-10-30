<template>
  <section class="card overflow-hidden" data-testid="upstream-cost-daily-table">
    <header class="border-b border-gray-200 px-4 py-4 dark:border-dark-700">
      <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('usage.supplierCost.dailyTitle') }}</h2>
      <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('usage.supplierCost.dailyDescription') }}</p>
    </header>
    <div v-if="loading" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('common.loading') }}</div>
    <div v-else-if="!rows.length" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">{{ t('usage.supplierCost.dailyEmpty') }}</div>
    <div v-else class="overflow-x-auto">
      <table class="min-w-[1220px] w-full text-sm">
        <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800/60 dark:text-gray-400">
          <tr>
            <th class="px-4 py-3 text-left">{{ t('usage.supplierCost.date') }}</th>
            <th class="px-4 py-3 text-left">{{ t('usage.supplierCost.upstream') }}</th>
            <th class="px-4 py-3 text-left">{{ t('usage.supplierCost.providerType') }}</th>
            <th class="px-4 py-3 text-left">{{ t('usage.supplierCost.model') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.requests') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.inputTokens') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.outputTokens') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.cacheTokens') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.dailyAccountCost') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.dailyUserBilled') }}</th>
            <th class="px-3 py-3 text-right">{{ t('usage.supplierCost.dailyGrossProfit') }}</th>
            <th class="px-4 py-3 text-right">{{ t('usage.supplierCost.dailyGrossMargin') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="row in rows" :key="`${row.date}-${row.provider_id}-${row.provider_name}-${row.provider_type}-${row.model}`" class="text-gray-700 dark:text-gray-200">
            <td class="whitespace-nowrap px-4 py-3 tabular-nums">{{ row.date }}</td>
            <td class="px-4 py-3 font-medium">{{ row.provider_name || t('usage.supplierCost.unset') }}</td>
            <td class="px-4 py-3 text-xs uppercase text-gray-500">{{ row.provider_type || '—' }}</td>
            <td class="max-w-[240px] truncate px-4 py-3" :title="row.model">{{ row.model || '—' }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ row.requests.toLocaleString() }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ formatTokens(row.input_tokens) }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ formatTokens(row.output_tokens) }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ formatTokens(row.cache_tokens) }}</td>
            <td class="px-3 py-3 text-right font-mono">${{ money(row.account_cost) }}</td>
            <td class="px-3 py-3 text-right font-mono">${{ money(row.user_billed) }}</td>
            <td class="px-3 py-3 text-right font-mono" :class="row.gross_profit >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">${{ money(row.gross_profit) }}</td>
            <td class="px-4 py-3 text-right font-mono" :class="row.gross_margin >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">{{ percent(row.gross_margin) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { UpstreamCostDailyStat } from '@/types'

withDefaults(defineProps<{ rows: UpstreamCostDailyStat[]; loading?: boolean }>(), { loading: false })
const { t } = useI18n()
const money = (value: number) => Number(value || 0).toFixed(4)
const percent = (value: number) => `${(Number(value || 0) * 100).toFixed(2)}%`
const formatTokens = (value: number) => {
  const n = Number(value || 0)
  if (n >= 1e9) return `${(n / 1e9).toFixed(2)}B`
  if (n >= 1e6) return `${(n / 1e6).toFixed(2)}M`
  if (n >= 1e3) return `${(n / 1e3).toFixed(2)}K`
  return n.toLocaleString()
}
</script>
