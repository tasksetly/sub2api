<template>
  <section class="card overflow-hidden">
    <header class="border-b border-gray-200 p-4 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('admin.upstreamProviders.financeTitle') }}</h2>
        <slot name="actions" />
      </div>
      <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreamProviders.financeHint') }}</p>
    </header>
    <div v-if="loading" class="p-6 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
    <div v-else-if="error" class="p-6 text-sm text-red-600 dark:text-red-400">{{ error }}</div>
    <div v-else-if="!providers.length" class="p-6 text-center text-sm text-gray-500">{{ t('admin.upstreamProviders.noProviders') }}</div>
    <div v-else class="overflow-x-auto">
      <table class="min-w-[1100px] w-full text-sm">
        <thead class="bg-gray-50 text-xs text-gray-500 dark:bg-dark-800">
          <tr>
            <th class="px-4 py-3 text-left">{{ t('admin.upstreamProviders.colName') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financeRecharge') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financePaid') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financeConsumed') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financeCost') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financeBilled') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financeProfit') }}</th>
            <th class="px-4 py-3 text-right">{{ t('admin.upstreamProviders.financeDifference') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-100 dark:divide-dark-700">
          <tr v-for="provider in providers" :key="provider.id" class="align-top text-gray-700 dark:text-gray-200">
            <td class="px-4 py-3">
              <div class="font-medium">{{ provider.name }}</div>
              <div class="text-xs text-gray-500">{{ provider.provider_type }}</div>
              <p v-if="provider.last_sync_error" class="mt-1 max-w-xs text-xs text-red-600 dark:text-red-400">{{ provider.last_sync_error }}</p>
            </td>
            <td class="px-4 py-3 text-right">
              <template v-if="provider.finance?.orders_synced_at">
                <span class="font-mono">{{ money(provider.finance.recharge_usd) }}</span>
                <div class="text-xs text-gray-500">{{ t('admin.upstreamProviders.financeOrders', { count: provider.finance.order_count }) }}</div>
                <div class="text-xs text-gray-500">{{ formatDateTime(provider.finance.orders_synced_at) }}</div>
                <div v-if="provider.finance.coverage === 'observed_history'" class="max-w-xs text-xs text-amber-600 dark:text-amber-400">{{ t('admin.upstreamProviders.financeObserved') }}</div>
                <div v-if="provider.finance.unknown_credit_orders" class="text-xs text-amber-600 dark:text-amber-400">{{ t('admin.upstreamProviders.financeUnknownUnits', { count: provider.finance.unknown_credit_orders }) }}</div>
              </template>
              <span v-else class="text-gray-400">—</span>
              <p v-if="provider.finance?.orders_error" class="mt-1 max-w-xs text-xs text-red-600 dark:text-red-400">{{ provider.finance.orders_error }}</p>
            </td>
            <td class="px-4 py-3 text-right font-mono">
              <template v-if="provider.finance?.orders_synced_at">
                <div v-for="(amount, currency) in provider.finance.paid_amounts" :key="currency" class="whitespace-nowrap">{{ currencyLabel(currency) }} {{ amount.toFixed(2) }}</div>
                <span v-if="!Object.keys(provider.finance.paid_amounts).length">—</span>
                <p v-if="provider.finance.unknown_paid_orders" class="max-w-xs font-sans text-xs text-amber-600 dark:text-amber-400">{{ t('admin.upstreamProviders.financeUnknownPaid', { count: provider.finance.unknown_paid_orders }) }}</p>
              </template>
              <span v-else class="text-gray-400">—</span>
            </td>
            <td class="px-4 py-3 text-right">
              <span class="font-mono">{{ money(provider.finance?.used_usd) }}</span>
              <div v-if="provider.finance?.usage_synced_at" class="text-xs text-gray-500">{{ formatDateTime(provider.finance.usage_synced_at) }}</div>
              <p v-if="provider.finance?.usage_error" class="mt-1 max-w-xs text-xs text-red-600 dark:text-red-400">{{ provider.finance.usage_error }}</p>
            </td>
            <td class="px-4 py-3 text-right font-mono">{{ money(provider.finance?.consumed_cost_usd) }}</td>
            <td class="px-4 py-3 text-right font-mono">{{ money(provider.local_user_billed_usd) }}</td>
            <td class="px-4 py-3 text-right font-mono" :class="profitClass(provider.finance?.gross_profit_usd)">{{ money(provider.finance?.gross_profit_usd) }}</td>
            <td class="px-4 py-3 text-right font-mono">{{ money(provider.finance?.cost_difference_usd) }}</td>
          </tr>
        </tbody>
      </table>
    </div>
  </section>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import type { UpstreamProviderWithStats } from '@/types/upstreamProvider'
import { formatDateTime } from '@/utils/format'
withDefaults(defineProps<{ providers: UpstreamProviderWithStats[]; loading?: boolean; error?: string }>(), { loading: false, error: '' })
const { t } = useI18n()
const currencyLabel = (currency: string) => currency.startsWith('UNSPECIFIED') ? t('admin.upstreamProviders.financeUnspecifiedCurrency') + (currency.includes('/') ? ` (${currency.split('/')[1]})` : '') : currency
const money = (value: number | null | undefined) => value == null ? '—' : `$${value.toFixed(4)}`
const profitClass = (value: number | null | undefined) => value == null ? '' : value >= 0 ? 'text-emerald-600 dark:text-emerald-400' : 'text-red-600 dark:text-red-400'
</script>
