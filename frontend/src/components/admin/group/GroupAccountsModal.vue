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
        <span class="text-gray-500 dark:text-gray-400">
          {{ t('admin.groups.groupAccountsGroupRate', { rate: formatMultiplier(group.rate_multiplier) }) }}
        </span>
      </div>

      <section class="border-b border-gray-200 pb-4 dark:border-dark-600">
        <div class="flex flex-wrap items-center gap-2">
          <button type="button" class="btn btn-primary" :disabled="adding" @click="toggleCandidates">
            <Icon name="plus" size="sm" class="mr-1.5" />
            {{ showCandidates ? t('common.cancel') : t('admin.groups.groupAccountsAdd') }}
          </button>
          <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.groupAccountsMarginHint') }}</span>
        </div>
        <div v-if="showCandidates" class="mt-3 space-y-3 rounded border border-gray-200 p-3 dark:border-dark-600">
          <div class="flex flex-wrap items-center gap-2">
            <input v-model="candidateSearch" class="input w-full sm:w-64" :placeholder="t('admin.accounts.searchAccounts')" @input="debouncedCandidateReload" />
            <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.groups.groupAccountsSelected', { count: selectedCandidateIDs.length }) }}</span>
            <button type="button" class="btn btn-secondary ml-auto" :disabled="adding || selectedCandidateIDs.length === 0" @click="addSelectedCandidates">
              {{ adding ? t('common.loading') : t('admin.groups.groupAccountsAddSelected') }}
            </button>
          </div>
          <div class="max-h-64 overflow-auto">
            <div v-if="candidateLoading" class="py-5 text-center text-sm text-gray-500">{{ t('common.loading') }}</div>
            <div v-else-if="candidateAccounts.length === 0" class="py-5 text-center text-sm text-gray-500">{{ t('admin.groups.groupAccountsNoCandidates') }}</div>
            <label v-for="candidate in candidateAccounts" :key="candidate.id" class="flex cursor-pointer items-center gap-3 border-b border-gray-100 px-2 py-2 text-sm last:border-0 dark:border-dark-700">
              <input type="checkbox" :checked="selectedCandidateIDs.includes(candidate.id)" @change="toggleCandidateSelection(candidate.id)" />
              <span class="font-mono text-xs text-gray-500">#{{ candidate.id }}</span>
              <span class="min-w-0 flex-1 truncate">{{ candidate.name }}</span>
              <span class="text-xs text-gray-500">{{ candidate.platform }} / {{ candidate.type }}</span>
              <span class="font-mono text-xs">{{ formatMultiplier(candidate.rate_multiplier ?? 1) }}x</span>
            </label>
          </div>
          <Pagination v-if="candidatePagination.total > 0" :total="candidatePagination.total" :page="candidatePagination.page" :page-size="candidatePagination.pageSize" @update:page="handleCandidatePageChange" @update:pageSize="handleCandidatePageSizeChange" />
          <div v-if="lastAddResults.length" class="space-y-1 border-t border-gray-200 pt-2 text-xs dark:border-dark-600">
            <div v-for="result in lastAddResults" :key="result.account_id" :class="result.added ? 'text-emerald-700 dark:text-emerald-400' : 'text-amber-700 dark:text-amber-400'">
              #{{ result.account_id }} {{ result.added ? t('admin.groups.groupAccountsAdded') : t(`admin.groups.groupAccountsSkipReasons.${result.reason || 'unknown'}`) }}
            </div>
          </div>
        </div>
      </section>

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
        <template #cell-group_rate_multiplier>
          <span class="font-mono text-sm text-gray-700 dark:text-gray-300">{{ formatMultiplier(group.rate_multiplier) }}x</span>
        </template>
        <template #cell-estimated_margin="{ row }">
          <span :class="marginClass(estimatedMargin(row))" class="font-mono text-sm">{{ formatMargin(estimatedMargin(row)) }}</span>
        </template>
        <template #cell-actions="{ row }">
          <button type="button" class="btn btn-sm btn-secondary" :disabled="removingAccountIDs.has(row.id)" :aria-label="t('admin.groups.groupAccountsRemove')" @click="removeAccount(row)">
            <Icon name="trash" size="sm" />
          </button>
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
import type { GroupAccountMutationResult } from '@/api/admin/groups'

const props = defineProps<{
  show: boolean
  group: AdminGroup | null
}>()

const emit = defineEmits<{
  close: []
  success: []
}>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const accounts = ref<Account[]>([])
const filters = reactive({ search: '', status: '' })
const pagination = reactive({ page: 1, pageSize: 20, total: 0 })
const showCandidates = ref(false)
const candidateLoading = ref(false)
const adding = ref(false)
const candidateAccounts = ref<Account[]>([])
const candidateSearch = ref('')
const selectedCandidateIDs = ref<number[]>([])
const lastAddResults = ref<GroupAccountMutationResult[]>([])
const candidatePagination = reactive({ page: 1, pageSize: 20, total: 0 })
const removingAccountIDs = reactive(new Set<number>())

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
  { key: 'group_rate_multiplier', label: t('admin.groups.groupAccountsGroupRateColumn'), sortable: false },
  { key: 'estimated_margin', label: t('admin.groups.groupAccountsMarginColumn'), sortable: false },
  { key: 'upstream_billing_rate', label: t('admin.accounts.columns.upstreamBillingRate'), sortable: false },
  { key: 'last_used_at', label: t('admin.accounts.columns.lastUsed'), sortable: false },
  { key: 'actions', label: t('admin.groups.groupAccountsActions'), sortable: false }
])

const estimatedMargin = (account: Account): number | null => {
  const groupRate = props.group?.rate_multiplier
  const accountRate = account.rate_multiplier ?? 1
  if (typeof groupRate !== 'number' || !Number.isFinite(groupRate) || groupRate <= 0 || !Number.isFinite(accountRate)) return null
  return ((groupRate - accountRate) / groupRate) * 100
}
const formatMargin = (value: number | null): string => value === null ? '-' : `${value.toFixed(2)}%`
const marginClass = (value: number | null): string => value === null ? 'text-gray-500 dark:text-gray-400' : value < 0 ? 'text-rose-700 dark:text-rose-400' : 'text-emerald-700 dark:text-emerald-400'

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

const loadCandidates = async () => {
  if (!props.group) return
  candidateLoading.value = true
  try {
    const result = await adminAPI.groups.getGroupAccountCandidates(props.group.id, candidatePagination.page, candidatePagination.pageSize, candidateSearch.value)
    candidateAccounts.value = result.items
    candidatePagination.total = result.total
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.groupAccountsCandidatesFailed')))
  } finally {
    candidateLoading.value = false
  }
}

const toggleCandidates = () => {
  showCandidates.value = !showCandidates.value
  if (showCandidates.value) loadCandidates()
}
const toggleCandidateSelection = (id: number) => {
  if (selectedCandidateIDs.value.includes(id)) {
    selectedCandidateIDs.value = selectedCandidateIDs.value.filter(candidateID => candidateID !== id)
    return
  }
  if (selectedCandidateIDs.value.length >= 100) {
    appStore.showError(t('admin.groups.groupAccountsSelectionLimit'))
    return
  }
  selectedCandidateIDs.value = [...selectedCandidateIDs.value, id]
}
const addSelectedCandidates = async () => {
  if (!props.group || adding.value || selectedCandidateIDs.value.length === 0) return
  adding.value = true
  try {
    lastAddResults.value = await adminAPI.groups.addGroupAccounts(props.group.id, selectedCandidateIDs.value)
    selectedCandidateIDs.value = []
    await Promise.all([loadAccounts(), loadCandidates()])
    emit('success')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.groupAccountsAddFailed')))
  } finally {
    adding.value = false
  }
}
const removeAccount = async (account: Account) => {
  if (!props.group || removingAccountIDs.has(account.id)) return
  if (!window.confirm(t('admin.groups.groupAccountsRemoveConfirm', { name: account.name }))) return
  removingAccountIDs.add(account.id)
  try {
    await adminAPI.groups.removeGroupAccount(props.group.id, account.id)
    await loadAccounts()
    emit('success')
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.groups.groupAccountsRemoveFailed')))
  } finally {
    removingAccountIDs.delete(account.id)
  }
}
const handleCandidatePageChange = (page: number) => { candidatePagination.page = page; loadCandidates() }
const handleCandidatePageSizeChange = (size: number) => { candidatePagination.pageSize = size; candidatePagination.page = 1; loadCandidates() }
let candidateSearchTimeout: ReturnType<typeof setTimeout> | undefined
const debouncedCandidateReload = () => {
  clearTimeout(candidateSearchTimeout)
  candidateSearchTimeout = setTimeout(() => { candidatePagination.page = 1; loadCandidates() }, 300)
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
    showCandidates.value = false
    candidateAccounts.value = []
    selectedCandidateIDs.value = []
    lastAddResults.value = []
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
  clearTimeout(candidateSearchTimeout)
  pauseBillingClock()
})
</script>
