<template>
  <div class="overflow-x-auto">
    <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
      <thead class="bg-gray-50 dark:bg-gray-800/50">
        <tr>
          <th class="w-8 px-3 py-3"></th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colName') }}</th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colBalance') }}</th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colConcurrency') }}</th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colRateRange') }}</th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colLocalCost') }}</th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colAccounts') }}</th>
          <th class="th-cell">{{ t('admin.upstreamProviders.colLastSync') }}</th>
          <th class="th-cell text-right">{{ t('admin.upstreamProviders.colActions') }}</th>
        </tr>
      </thead>
      <tbody class="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
        <template v-for="provider in providers" :key="provider.id">
          <tr class="hover:bg-gray-50 dark:hover:bg-gray-800/50">
            <!-- 展开箭头：看该上游各分组的倍率明细 -->
            <td class="px-3 py-3">
              <button
                class="text-gray-400 transition hover:text-gray-600 dark:hover:text-gray-200"
                :title="t('admin.upstreamProviders.viewGroups')"
                @click="emit('toggle-groups', provider)"
              >
                <Icon
                  name="chevronRight"
                  size="sm"
                  :class="expandedId === provider.id ? 'rotate-90 transform' : ''"
                />
              </button>
            </td>

            <td class="px-3 py-3">
              <div class="font-medium text-gray-900 dark:text-white">{{ provider.name }}</div>
              <div class="max-w-[220px] truncate text-xs text-gray-500 dark:text-gray-400">
                {{ provider.base_url }}
              </div>
              <div class="mt-1 flex flex-wrap items-center gap-1">
                <span v-if="provider.status !== 'active'" class="badge badge-gray">
                  {{ t('common.inactive') }}
                </span>
                <span v-if="provider.has_totp_secret" class="badge badge-primary">
                  {{ t('admin.upstreamProviders.hasTotp') }}
                </span>
                <span v-if="!provider.has_password" class="badge badge-warning">
                  {{ t('admin.upstreamProviders.noPassword') }}
                </span>
              </div>
            </td>

            <td class="px-3 py-3 text-sm">
              <span v-if="provider.balance !== null" class="font-medium text-gray-900 dark:text-white">
                ${{ formatMoney(provider.balance) }}
              </span>
              <span v-else class="text-gray-400">—</span>
              <div
                v-if="provider.frozen_balance !== null && provider.frozen_balance > 0"
                class="text-xs text-gray-500 dark:text-gray-400"
              >
                {{ t('admin.upstreamProviders.frozenLabel') }}: ${{ formatMoney(provider.frozen_balance) }}
              </div>
            </td>

            <td class="px-3 py-3 text-sm text-gray-900 dark:text-white">
              {{ provider.upstream_concurrency ?? '—' }}
            </td>

            <!-- 倍率区间：横向比价的核心列 -->
            <td class="px-3 py-3 text-sm">
              <span v-if="provider.min_rate !== null" class="text-gray-900 dark:text-white">
                {{ formatRateRange(provider.min_rate, provider.max_rate) }}
              </span>
              <span v-else class="text-gray-400">—</span>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ provider.group_count }} {{ t('admin.upstreamProviders.colGroups') }}
              </div>
            </td>

            <td class="px-3 py-3 text-sm">
              <span class="text-gray-900 dark:text-white">${{ formatMoney(provider.local_cost_usd) }}</span>
              <div class="text-xs text-gray-500 dark:text-gray-400">
                {{ provider.local_requests }} req
              </div>
            </td>

            <td class="px-3 py-3 text-sm text-gray-900 dark:text-white">
              {{ provider.account_count }}
            </td>

            <td class="px-3 py-3 text-sm">
              <div v-if="provider.last_sync_error" class="text-red-600 dark:text-red-400">
                <div class="font-medium">{{ t('admin.upstreamProviders.syncFailed') }}</div>
                <div class="max-w-[200px] truncate text-xs" :title="provider.last_sync_error">
                  {{ provider.last_sync_error }}
                </div>
              </div>
              <span v-else-if="provider.last_sync_at" class="text-gray-600 dark:text-gray-300">
                {{ formatDateTime(provider.last_sync_at) }}
              </span>
              <span v-else class="text-gray-400">
                {{ t('admin.upstreamProviders.neverSynced') }}
              </span>
            </td>

            <td class="px-3 py-3">
              <div class="flex items-center justify-end gap-1">
                <button
                  class="icon-btn"
                  :disabled="syncingId === provider.id"
                  :title="t('admin.upstreamProviders.syncNow')"
                  @click="emit('sync', provider)"
                >
                  <Icon
                    name="refresh"
                    size="sm"
                    :class="syncingId === provider.id ? 'animate-spin' : ''"
                  />
                </button>
                <button
                  class="icon-btn"
                  :disabled="testingId === provider.id"
                  :title="t('admin.upstreamProviders.testConnection')"
                  @click="emit('test', provider)"
                >
                  <Icon name="play" size="sm" />
                </button>
                <button
                  class="icon-btn"
                  :title="t('admin.upstreamProviders.provision')"
                  @click="emit('provision', provider)"
                >
                  <Icon name="plus" size="sm" />
                </button>
                <button
                  class="icon-btn"
                  :title="t('common.edit')"
                  @click="emit('edit', provider)"
                >
                  <Icon name="edit" size="sm" />
                </button>
                <button
                  class="icon-btn text-red-600 hover:text-red-700 dark:text-red-400"
                  :title="t('common.delete')"
                  @click="emit('delete', provider)"
                >
                  <Icon name="trash" size="sm" />
                </button>
              </div>
            </td>
          </tr>

          <!-- 展开行：分组倍率与限额明细 -->
          <tr v-if="expandedId === provider.id" class="bg-gray-50 dark:bg-gray-800/30">
            <td colspan="9" class="px-6 py-4">
              <div v-if="groupsLoading" class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('common.loading') }}
              </div>
              <div v-else-if="groups.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
                {{ t('admin.upstreamProviders.groupsEmpty') }}
              </div>
              <table v-else class="min-w-full text-sm">
                <thead>
                  <tr class="text-left text-xs uppercase text-gray-500 dark:text-gray-400">
                    <th class="py-2 pr-4">{{ t('admin.upstreamProviders.colGroupName') }}</th>
                    <th class="py-2 pr-4">{{ t('admin.upstreamProviders.colPlatform') }}</th>
                    <th class="py-2 pr-4">{{ t('admin.upstreamProviders.colRate') }}</th>
                    <th class="py-2 pr-4">{{ t('admin.upstreamProviders.colDailyLimit') }}</th>
                    <th class="py-2 pr-4">{{ t('admin.upstreamProviders.colMonthlyLimit') }}</th>
                  </tr>
                </thead>
                <tbody>
                  <tr
                    v-for="group in sortedGroups"
                    :key="group.id"
                    class="border-t border-gray-200 dark:border-gray-700"
                  >
                    <td class="py-2 pr-4 text-gray-900 dark:text-white">{{ group.name }}</td>
                    <td class="py-2 pr-4 text-gray-600 dark:text-gray-300">
                      {{ group.platform || '—' }}
                    </td>
                    <td class="py-2 pr-4">
                      <span class="font-medium text-gray-900 dark:text-white">
                        ×{{ formatRate(group.comparable_rate) }}
                      </span>
                      <span
                        v-if="group.effective_rate_multiplier !== null"
                        class="ml-1 text-xs text-blue-600 dark:text-blue-400"
                      >
                        {{ t('admin.upstreamProviders.rateExclusive') }}
                      </span>
                      <span v-else class="ml-1 text-xs text-gray-400">
                        {{ t('admin.upstreamProviders.rateBase') }}
                      </span>
                      <div
                        v-if="group.peak_rate_enabled && group.peak_rate_multiplier"
                        class="text-xs text-amber-600 dark:text-amber-400"
                      >
                        {{
                          t('admin.upstreamProviders.peakHint', {
                            start: group.peak_start || '—',
                            end: group.peak_end || '—',
                            multiplier: formatRate(group.peak_rate_multiplier)
                          })
                        }}
                      </div>
                    </td>
                    <td class="py-2 pr-4 text-gray-600 dark:text-gray-300">
                      {{ group.daily_limit_usd !== null ? `$${formatMoney(group.daily_limit_usd)}` : '—' }}
                    </td>
                    <td class="py-2 pr-4 text-gray-600 dark:text-gray-300">
                      {{ group.monthly_limit_usd !== null ? `$${formatMoney(group.monthly_limit_usd)}` : '—' }}
                    </td>
                  </tr>
                </tbody>
              </table>
            </td>
          </tr>
        </template>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { UpstreamProviderWithStats, UpstreamGroup } from '@/types/upstreamProvider'
import Icon from '@/components/icons/Icon.vue'
import { formatDateTime } from '@/utils/format'

interface Props {
  providers: UpstreamProviderWithStats[]
  expandedId: number | null
  groups: UpstreamGroup[]
  groupsLoading: boolean
  syncingId: number | null
  testingId: number | null
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'toggle-groups', provider: UpstreamProviderWithStats): void
  (e: 'sync', provider: UpstreamProviderWithStats): void
  (e: 'test', provider: UpstreamProviderWithStats): void
  (e: 'provision', provider: UpstreamProviderWithStats): void
  (e: 'edit', provider: UpstreamProviderWithStats): void
  (e: 'delete', provider: UpstreamProviderWithStats): void
}>()

const { t } = useI18n()

// 按比价倍率升序：最便宜的分组排最前，这是比价时最想先看到的
const sortedGroups = computed(() =>
  [...props.groups].sort((a, b) => a.comparable_rate - b.comparable_rate)
)

function formatMoney(value: number): string {
  return value.toFixed(2)
}

function formatRate(value: number): string {
  // 倍率常见是 0.1 / 1.5 这类值，保留两位但去掉无意义的尾零
  return Number(value.toFixed(4)).toString()
}

function formatRateRange(min: number, max: number | null): string {
  if (max === null || min === max) {
    return `×${formatRate(min)}`
  }
  return `×${formatRate(min)} ~ ×${formatRate(max)}`
}
</script>

<style scoped>
.th-cell {
  @apply px-3 py-3 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400;
}

.icon-btn {
  @apply rounded p-1.5 text-gray-500 transition hover:bg-gray-100 hover:text-gray-700 disabled:cursor-not-allowed disabled:opacity-50 dark:text-gray-400 dark:hover:bg-gray-700 dark:hover:text-gray-200;
}
</style>
