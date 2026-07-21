<template>
  <section class="card overflow-hidden" data-testid="supplier-cost-table">
    <header class="flex flex-wrap items-start justify-between gap-3 border-b border-gray-200 px-4 py-4 dark:border-dark-700">
      <div>
        <h2 class="text-base font-semibold text-gray-900 dark:text-white">{{ t('usage.supplierCost.title') }}</h2>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('usage.supplierCost.description') }}</p>
      </div>
      <span v-if="rows.length" class="text-xs text-gray-500 dark:text-gray-400">
        {{ t('usage.supplierCost.suppliers') }}: {{ rows.length.toLocaleString() }}
      </span>
    </header>

    <div v-if="loading" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
      <span class="inline-flex items-center gap-2">
        <span class="h-4 w-4 animate-spin rounded-full border-2 border-gray-300 border-t-primary-500" aria-hidden="true" />
        {{ t('common.loading') }}
      </span>
    </div>
    <div v-else-if="rows.length === 0" class="px-4 py-10 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('usage.supplierCost.empty') }}
    </div>
    <div v-else class="overflow-x-auto">
      <table class="min-w-[980px] w-full text-sm">
        <thead class="bg-gray-50 text-xs uppercase tracking-wide text-gray-500 dark:bg-dark-800/60 dark:text-gray-400">
          <tr>
            <th class="px-4 py-3 text-left font-medium">{{ t('usage.supplierCost.supplier') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.accounts') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.requests') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.tokens') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.standardCost') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.accountCost') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.userBilled') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.grossProfit') }}</th>
            <th class="px-3 py-3 text-right font-medium">{{ t('usage.supplierCost.grossMargin') }}</th>
            <th class="px-4 py-3 text-right font-medium">{{ t('usage.supplierCost.share') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="row in rows" :key="row.supplier || '__unset__'" class="text-gray-700 dark:text-gray-200">
            <td class="whitespace-nowrap px-4 py-3 font-medium">
              {{ row.supplier || t('usage.supplierCost.unset') }}
            </td>
            <td class="px-3 py-3 text-right tabular-nums">{{ row.account_count.toLocaleString() }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ row.requests.toLocaleString() }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ formatTokens(row.total_tokens) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums">${{ formatCost(row.standard_cost) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums">${{ formatCost(row.account_cost) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums">${{ formatCost(row.user_billed) }}</td>
            <td
              class="px-3 py-3 text-right font-mono tabular-nums"
              :class="row.gross_profit >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'"
            >
              ${{ formatCost(row.gross_profit) }}
            </td>
            <td
              class="px-3 py-3 text-right font-mono tabular-nums"
              :class="row.gross_margin >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'"
            >
              {{ formatPercent(row.gross_margin) }}
            </td>
            <td class="px-4 py-3 text-right font-mono tabular-nums">{{ formatPercent(row.cost_percentage) }}</td>
          </tr>
        </tbody>
        <tfoot class="border-t border-gray-200 bg-gray-50 font-semibold text-gray-900 dark:border-dark-700 dark:bg-dark-800/60 dark:text-white">
          <tr>
            <td class="px-4 py-3">{{ t('usage.supplierCost.total') }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ totals.accounts.toLocaleString() }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ totals.requests.toLocaleString() }}</td>
            <td class="px-3 py-3 text-right tabular-nums">{{ formatTokens(totals.tokens) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums">${{ formatCost(totals.standardCost) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums">${{ formatCost(totals.accountCost) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums">${{ formatCost(totals.userBilled) }}</td>
            <td class="px-3 py-3 text-right font-mono tabular-nums" :class="totals.grossProfit >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">
              ${{ formatCost(totals.grossProfit) }}
            </td>
            <td class="px-3 py-3 text-right font-mono tabular-nums" :class="totals.grossMargin >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'">
              {{ formatPercent(totals.grossMargin) }}
            </td>
            <td class="px-4 py-3 text-right font-mono tabular-nums">{{ formatPercent(totals.costShare) }}</td>
          </tr>
        </tfoot>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { SupplierCostStat } from '@/types'

const props = withDefaults(defineProps<{
  rows: SupplierCostStat[]
  loading?: boolean
}>(), {
  loading: false,
})

const { t } = useI18n()

const formatCost = (value: number) => Number(value || 0).toFixed(4)
const formatPercent = (value: number) => `${(Number(value || 0) * 100).toFixed(2)}%`
const formatTokens = (value: number) => {
  const safeValue = Number(value || 0)
  if (safeValue >= 1e9) return `${(safeValue / 1e9).toFixed(2)}B`
  if (safeValue >= 1e6) return `${(safeValue / 1e6).toFixed(2)}M`
  if (safeValue >= 1e3) return `${(safeValue / 1e3).toFixed(2)}K`
  return safeValue.toLocaleString()
}

const totals = computed(() => {
  const result = props.rows.reduce((total, row) => ({
    accounts: total.accounts + row.account_count,
    requests: total.requests + row.requests,
    tokens: total.tokens + row.total_tokens,
    standardCost: total.standardCost + row.standard_cost,
    accountCost: total.accountCost + row.account_cost,
    userBilled: total.userBilled + row.user_billed,
  }), {
    accounts: 0,
    requests: 0,
    tokens: 0,
    standardCost: 0,
    accountCost: 0,
    userBilled: 0,
  })
  const grossProfit = result.userBilled - result.accountCost
  return {
    ...result,
    grossProfit,
    grossMargin: result.userBilled > 0 ? grossProfit / result.userBilled : 0,
    costShare: result.accountCost > 0 ? 1 : 0,
  }
})
</script>
