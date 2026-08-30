<template>
  <section class="mt-8">
    <header class="mb-3 flex flex-wrap items-center justify-between gap-3">
      <div>
        <h3 class="text-base font-medium text-gray-900 dark:text-white">
          {{ t('admin.upstreamProviders.compareTitle') }}
        </h3>
        <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.upstreamProviders.compareHint') }}
        </p>
      </div>

      <div class="flex items-center gap-2">
        <div class="w-40">
          <Select
            v-model="platform"
            :options="platformOptions"
            :placeholder="t('admin.upstreamProviders.comparePlatformAll')"
            @change="handlePlatformChange"
          />
        </div>
        <button
          class="btn btn-secondary"
          :disabled="loading"
          :title="t('common.refresh')"
          @click="load"
        >
          <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
        </button>
      </div>
    </header>

    <div v-if="loading" class="py-6 text-center text-sm text-gray-500 dark:text-gray-400">
      {{ t('common.loading') }}
    </div>
    <div
      v-else-if="rows.length === 0"
      class="rounded border border-dashed border-gray-300 py-8 text-center text-sm text-gray-500 dark:border-gray-600 dark:text-gray-400"
    >
      {{ t('admin.upstreamProviders.compareEmpty') }}
    </div>

    <div v-else class="overflow-x-auto rounded border border-gray-200 dark:border-gray-700">
      <table class="min-w-full divide-y divide-gray-200 dark:divide-gray-700">
        <thead class="bg-gray-50 dark:bg-gray-800/50">
          <tr>
            <th class="th-cell w-10">#</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.compareCorrectedRate') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.compareProvider') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.colGroupName') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.colPlatform') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.colBalance') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.colDailyLimit') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.colMonthlyLimit') }}</th>
            <th class="th-cell">{{ t('admin.upstreamProviders.compareInUse') }}</th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200 bg-white dark:divide-gray-700 dark:bg-gray-900">
          <tr
            v-for="(row, index) in rows"
            :key="`${row.upstream_provider_id}-${row.remote_group_id}`"
            class="hover:bg-gray-50 dark:hover:bg-gray-800/50"
          >
            <td class="px-3 py-2 text-xs text-gray-400">{{ rowRank(index) }}</td>

            <!-- 修正后倍率：比价的主列，最便宜的排最前。
                 排序口径与后端一致（声明倍率 × 充值比例修正）。 -->
            <td class="px-3 py-2">
              <span
                :class="[
                  'font-semibold',
                  rowRank(index) === 1
                    ? 'text-emerald-600 dark:text-emerald-400'
                    : 'text-gray-900 dark:text-white'
                ]"
              >
                ×{{ formatRate(row.corrected_rate) }}
              </span>
              <span
                v-if="row.effective_rate_multiplier !== null"
                class="ml-1 text-xs text-blue-600 dark:text-blue-400"
              >
                {{ t('admin.upstreamProviders.rateExclusive') }}
              </span>
              <!-- 修正过的行把原始声明倍率一并显示，否则跟上游后台对不上账 -->
              <div
                v-if="row.provider_rate_correction !== 1"
                class="text-xs text-gray-500 dark:text-gray-400"
              >
                {{
                  t('admin.upstreamProviders.compareRateCorrectedFrom', {
                    raw: formatRate(row.comparable_rate),
                    correction: formatRate(row.provider_rate_correction)
                  })
                }}
              </div>
              <div
                v-if="row.peak_rate_enabled && row.peak_rate_multiplier"
                class="text-xs text-amber-600 dark:text-amber-400"
              >
                {{
                  t('admin.upstreamProviders.peakHint', {
                    start: row.peak_start || '—',
                    end: row.peak_end || '—',
                    multiplier: formatRate(row.peak_rate_multiplier)
                  })
                }}
              </div>
            </td>

            <td class="px-3 py-2 text-sm">
              <span class="text-gray-900 dark:text-white">{{ row.provider_name }}</span>
              <div class="flex flex-wrap items-center gap-1">
                <span v-if="row.provider_status !== 'active'" class="badge badge-gray">
                  {{ t('common.inactive') }}
                </span>
                <!-- 关了定时同步的上游，这行数据可能已经过期 -->
                <span v-if="!row.provider_sync_enabled" class="badge badge-warning">
                  {{ t('admin.upstreamProviders.compareStale') }}
                </span>
              </div>
            </td>

            <td class="px-3 py-2 text-sm text-gray-900 dark:text-white">{{ row.name }}</td>
            <td class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300">
              {{ row.platform || '—' }}
            </td>

            <!-- 余额：倍率再低，没余额也用不了 -->
            <td class="px-3 py-2 text-sm">
              <span
                v-if="row.provider_balance !== null"
                :class="
                  row.provider_balance <= 0
                    ? 'text-red-600 dark:text-red-400'
                    : 'text-gray-900 dark:text-white'
                "
              >
                ${{ row.provider_balance.toFixed(2) }}
              </span>
              <span v-else class="text-gray-400">—</span>
            </td>

            <td class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300">
              {{ row.daily_limit_usd !== null ? `$${row.daily_limit_usd.toFixed(2)}` : '—' }}
            </td>
            <td class="px-3 py-2 text-sm text-gray-600 dark:text-gray-300">
              {{ row.monthly_limit_usd !== null ? `$${row.monthly_limit_usd.toFixed(2)}` : '—' }}
            </td>

            <td class="px-3 py-2 text-sm">
              <span v-if="row.local_account_count > 0" class="badge badge-success">
                {{ row.local_account_count }}
              </span>
              <span v-else class="text-gray-400">—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- 比价表是分页的：不翻页只能看到最便宜的前 N 条 -->
    <Pagination
      v-if="total > 0"
      class="mt-3"
      :page="page"
      :page-size="pageSize"
      :total="total"
      @update:page="handlePageChange"
      @update:page-size="handlePageSizeChange"
    />
  </section>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { UpstreamGroupComparison } from '@/types/upstreamProvider'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import Pagination from '@/components/common/Pagination.vue'

const { t } = useI18n()
const appStore = useAppStore()

const rows = ref<UpstreamGroupComparison[]>([])
const loading = ref(false)
const platform = ref('')
const page = ref(1)
const pageSize = ref(50)
const total = ref(0)

// 跨平台比倍率没意义（anthropic 的 0.1 和 gemini 的 0.1 不是一回事），
// 所以默认全平台展示但提供筛选
const platformOptions = computed(() => [
  { value: '', label: t('admin.upstreamProviders.comparePlatformAll') },
  { value: 'anthropic', label: 'Anthropic' },
  { value: 'openai', label: 'OpenAI' },
  { value: 'gemini', label: 'Gemini' },
  { value: 'antigravity', label: 'Antigravity' },
  { value: 'grok', label: 'Grok' }
])

async function load() {
  loading.value = true
  try {
    const result = await adminAPI.upstreamProviders.compareGroups(
      platform.value || undefined,
      page.value,
      pageSize.value
    )
    rows.value = result.items ?? []
    total.value = result.total ?? 0
  } catch (error) {
    appStore.showError(resolveError(error))
    rows.value = []
    total.value = 0
  } finally {
    loading.value = false
  }
}

// 换平台要回到第一页，否则筛完可能停在一个已经不存在的页码上
function handlePlatformChange() {
  page.value = 1
  void load()
}

function handlePageChange(next: number) {
  page.value = next
  void load()
}

function handlePageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  void load()
}

function formatRate(value: number): string {
  return Number(value.toFixed(4)).toString()
}

// 全局序号：翻页后仍能看出这行在整体比价里排第几
function rowRank(index: number): number {
  return (page.value - 1) * pageSize.value + index + 1
}

function resolveError(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    return String((error as { message: unknown }).message)
  }
  return t('common.unknownError')
}

// 供父组件在同步后刷新
defineExpose({ reload: load })

onMounted(load)
</script>

<style scoped>
.th-cell {
  @apply px-3 py-2.5 text-left text-xs font-medium uppercase tracking-wider text-gray-500 dark:text-gray-400;
}
</style>
