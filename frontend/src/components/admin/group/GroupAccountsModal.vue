<template>
  <BaseDialog
    :show="show"
    :title="t('admin.groups.groupAccountsTitle')"
    width="full"
    @close="handleClose"
  >
    <div v-if="group" class="space-y-4">
      <!-- 分组信息 -->
      <div class="flex flex-wrap items-center gap-3 rounded-lg bg-gray-50 px-4 py-2.5 text-sm dark:bg-dark-700">
        <span class="inline-flex items-center gap-1.5" :class="platformColorClass">
          <PlatformIcon :platform="group.platform" size="sm" />
          {{ t('admin.groups.platforms.' + group.platform) }}
        </span>
        <span class="text-gray-400">|</span>
        <span class="font-medium text-gray-900 dark:text-white">{{ group.name }}</span>
        <span class="text-gray-400">|</span>
        <span class="text-gray-600 dark:text-gray-400">
          {{ t('admin.groups.groupAccountsCount', { count: pagination.total }) }}
        </span>
      </div>

      <!-- 筛选 -->
      <div class="flex flex-wrap items-center gap-2">
        <input
          v-model="filters.search"
          type="text"
          autocomplete="off"
          class="input w-full sm:w-56"
          :placeholder="t('admin.accounts.searchAccounts')"
          @input="debouncedReload"
        />
        <select v-model="filters.status" class="input w-full sm:w-40" @change="reload">
          <option value="">{{ t('admin.accounts.allStatus') }}</option>
          <option value="active">{{ t('admin.accounts.status.active') }}</option>
          <option value="inactive">{{ t('admin.accounts.status.inactive') }}</option>
          <option value="error">{{ t('admin.accounts.status.error') }}</option>
        </select>
        <button type="button" class="btn btn-secondary" :disabled="loading" @click="reload">
          <Icon name="refresh" size="sm" :class="['mr-1.5', loading ? 'animate-spin' : '']" />
          {{ t('common.refresh') }}
        </button>
        <RouterLink
          :to="{ path: '/admin/accounts', query: { group: String(group.id) } }"
          class="btn btn-secondary ml-auto"
          @click="handleClose"
        >
          <Icon name="externalLink" size="sm" class="mr-1.5" />
          {{ t('admin.groups.groupAccountsOpenPage') }}
        </RouterLink>
      </div>

      <!-- 列表 -->
      <DataTable
        :columns="cols"
        :data="accounts"
        :loading="loading"
        row-key="id"
        :sticky-actions-column="false"
      >
        <template #cell-id="{ value }">
          <span class="font-mono text-xs text-gray-500 dark:text-gray-400">#{{ value }}</span>
        </template>
        <template #cell-name="{ row }">
          <div class="flex min-w-0 flex-col">
            <span class="truncate font-medium text-gray-900 dark:text-white">{{ row.name }}</span>
            <span v-if="row.supplier" class="truncate text-xs text-gray-400">{{ row.supplier }}</span>
          </div>
        </template>
        <template #cell-platform_type="{ row }">
          <PlatformTypeBadge :platform="row.platform" :type="row.type" />
        </template>
        <template #cell-capacity="{ row }">
          <AccountCapacityCell :account="row" />
        </template>
        <template #cell-status="{ row }">
          <AccountStatusIndicator :account="row" />
        </template>
        <template #cell-schedulable="{ row }">
          <span
            :class="[
              'inline-flex items-center rounded px-1.5 py-0.5 text-[11px] font-medium',
              row.schedulable
                ? 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400'
                : 'bg-gray-100 text-gray-500 dark:bg-dark-600 dark:text-gray-400'
            ]"
          >
            {{ row.schedulable ? t('admin.accounts.schedulableEnabled') : t('admin.accounts.schedulableDisabled') }}
          </span>
        </template>
        <template #cell-groups="{ row }">
          <AccountGroupsCell :groups="row.groups" :max-display="4" />
        </template>
        <template #cell-priority="{ value }">
          <span class="text-sm text-gray-700 dark:text-gray-300">{{ value }}</span>
        </template>
        <template #cell-rate_multiplier="{ row }">
          <span class="inline-flex items-center gap-1 font-mono text-sm text-gray-700 dark:text-gray-300">
            <span>{{ formatMultiplier(row.rate_multiplier ?? 1) }}x</span>
            <span
              v-if="row.extra?.upstream_billing_rate_sync_enabled === true"
              class="inline-flex cursor-help text-emerald-600 dark:text-emerald-400"
              :aria-label="t('admin.accounts.upstreamBilling.syncedRateTooltip')"
              :title="t('admin.accounts.upstreamBilling.syncedRateTooltip')"
            >
              <Icon name="sync" size="xs" />
            </span>
          </span>
        </template>
        <template #cell-upstream_billing_rate="{ row }">
          <UpstreamBillingRateCell
            :account="row"
            :global-probe-enabled="upstreamBillingProbeGloballyEnabled"
            :now="upstreamBillingNow"
            :probing="probingUpstreamBilling.has(row.id)"
            @probe="handleProbeUpstreamBilling(row)"
          />
        </template>
        <template #cell-last_used_at="{ value }">
          <span class="text-xs text-gray-500 dark:text-gray-400">
            {{ value ? formatDateTime(value) : '-' }}
          </span>
        </template>
      </DataTable>

      <div v-if="!loading && accounts.length === 0" class="py-6 text-center text-sm text-gray-400 dark:text-gray-500">
        {{ t('admin.groups.groupAccountsEmpty') }}
      </div>

      <Pagination
        v-if="pagination.total > 0"
        :total="pagination.total"
        :page="pagination.page"
        :page-size="pagination.pageSize"
        @update:page="handlePageChange"
        @update:pageSize="handlePageSizeChange"
      />

      <div class="flex items-center justify-end border-t border-gray-200 pt-4 dark:border-dark-600">
        <button type="button" class="btn btn-sm px-4 py-1.5" @click="handleClose">
          {{ t('common.close') }}
        </button>
      </div>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onUnmounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useIntervalFn } from '@vueuse/core'
import { RouterLink } from 'vue-router'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { Account, AdminGroup, UpstreamBillingProbeSnapshot } from '@/types'
import BaseDialog from '@/components/common/BaseDialog.vue'
import DataTable from '@/components/common/DataTable.vue'
import Pagination from '@/components/common/Pagination.vue'
import Icon from '@/components/icons/Icon.vue'
import PlatformIcon from '@/components/common/PlatformIcon.vue'
import PlatformTypeBadge from '@/components/common/PlatformTypeBadge.vue'
import AccountCapacityCell from '@/components/account/AccountCapacityCell.vue'
import AccountGroupsCell from '@/components/account/AccountGroupsCell.vue'
import AccountStatusIndicator from '@/components/account/AccountStatusIndicator.vue'
import UpstreamBillingRateCell from '@/components/account/UpstreamBillingRateCell.vue'
import { formatDateTime } from '@/utils/format'
import { formatMultiplier } from '@/utils/formatters'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const accounts = ref<Account[]>([])
const filters = reactive({ search: '', status: '' })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })

const upstreamBillingProbeGloballyEnabled = ref<boolean | undefined>(undefined)
const upstreamBillingNow = ref(Date.now())
const probingUpstreamBilling = reactive(new Set<number>())

let searchTimeout: ReturnType<typeof setTimeout> | undefined

// Keeps the "stale / next probe" countdowns in the cell fresh while the modal is open.
const { pause: pauseBillingClock, resume: resumeBillingClock } = useIntervalFn(
  () => { upstreamBillingNow.value = Date.now() },
  60_000,
  { immediate: false }
)

const cols = computed(() => [
  { key: 'id', label: t('admin.accounts.columns.id'), sortable: false },
  { key: 'name', label: t('admin.accounts.columns.name'), sortable: false },
  { key: 'platform_type', label: t('admin.accounts.columns.platformType'), sortable: false },
  { key: 'capacity', label: t('admin.accounts.columns.capacity'), sortable: false },
  { key: 'status', label: t('admin.accounts.columns.status'), sortable: false },
  { key: 'schedulable', label: t('admin.accounts.columns.schedulable'), sortable: false },
  { key: 'groups', label: t('admin.accounts.columns.groups'), sortable: false },
  { key: 'priority', label: t('admin.accounts.columns.priority'), sortable: false },
  { key: 'rate_multiplier', label: t('admin.accounts.columns.billingRateMultiplier'), sortable: false },
  { key: 'upstream_billing_rate', label: t('admin.accounts.columns.upstreamBillingRate'), sortable: false },
  { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: false }
])

const platformColorClass = computed(() => {
  switch (props.group?.platform) {
    case 'anthropic': return 'text-orange-700 dark:text-orange-400'
    case 'openai': return 'text-emerald-700 dark:text-emerald-400'
    case 'antigravity': return 'text-purple-700 dark:text-purple-400'
    default: return 'text-blue-700 dark:text-blue-400'
  }
})

const loadAccounts = async () => {
  if (!props.group) return
  loading.value = true
  try {
    const res = await adminAPI.accounts.list(pagination.page, pagination.pageSize, {
      group: String(props.group.id),
      status: filters.status || undefined,
      search: filters.search.trim() || undefined
    })
    accounts.value = res.items
    pagination.total = res.total
  } catch (error) {
    appStore.showError(t('admin.accounts.failedToLoad'))
    console.error('Error loading group accounts:', error)
  } finally {
    loading.value = false
  }
}

const loadUpstreamBillingProbeGlobalState = async () => {
  if (upstreamBillingProbeGloballyEnabled.value !== undefined) return
  try {
    const settings = await adminAPI.accounts.getUpstreamBillingProbeSettings()
    upstreamBillingProbeGloballyEnabled.value = settings.enabled
  } catch (error) {
    console.error('Failed to load upstream billing probe settings:', error)
  }
}

const patchUpstreamBillingSnapshot = (accountID: number, snapshot: UpstreamBillingProbeSnapshot) => {
  const index = accounts.value.findIndex(item => item.id === accountID)
  if (index === -1) return
  const account = accounts.value[index]
  upstreamBillingNow.value = Date.now()
  const syncedRate = snapshot.status === 'ok' ? snapshot.synced_rate_multiplier : undefined
  accounts.value.splice(index, 1, {
    ...account,
    rate_multiplier: typeof syncedRate === 'number' && Number.isFinite(syncedRate) && syncedRate > 0
      ? syncedRate
      : account.rate_multiplier,
    extra: { ...account.extra, upstream_billing_probe: snapshot }
  })
}

const handleProbeUpstreamBilling = async (account: Account) => {
  if (probingUpstreamBilling.has(account.id)) return
  probingUpstreamBilling.add(account.id)
  try {
    const result = await adminAPI.accounts.probeUpstreamBilling(account.id)
    if (result.snapshot) {
      patchUpstreamBillingSnapshot(account.id, result.snapshot)
    }
  } catch (error) {
    console.error('Failed to probe upstream billing:', error)
    appStore.showError(extractApiErrorMessage(error, t('admin.accounts.upstreamBilling.probeFailed')))
  } finally {
    probingUpstreamBilling.delete(account.id)
  }
}

const reload = () => {
  pagination.page = 1
  loadAccounts()
}

const debouncedReload = () => {
  clearTimeout(searchTimeout)
  searchTimeout = setTimeout(reload, 300)
}

const handlePageChange = (page: number) => {
  pagination.page = page
  loadAccounts()
}

const handlePageSizeChange = (size: number) => {
  pagination.pageSize = size
  pagination.page = 1
  loadAccounts()
}

const handleClose = () => {
  emit('close')
}

watch(() => props.show, (val) => {
  if (val && props.group) {
    filters.search = ''
    filters.status = ''
    pagination.page = 1
    pagination.total = 0
    accounts.value = []
    upstreamBillingNow.value = Date.now()
    resumeBillingClock()
    loadAccounts()
    loadUpstreamBillingProbeGlobalState()
  } else {
    pauseBillingClock()
  }
})

onUnmounted(() => {
  clearTimeout(searchTimeout)
  pauseBillingClock()
})
</script>
