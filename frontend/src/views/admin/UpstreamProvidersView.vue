<template>
  <AppLayout>
    <TablePageLayout>
      <template #filters>
        <div class="flex flex-wrap items-center gap-3">
          <div class="relative w-full sm:w-64">
            <Icon
              name="search"
              size="md"
              class="absolute left-3 top-1/2 -translate-y-1/2 text-gray-400 dark:text-gray-500"
            />
            <input
              v-model="searchQuery"
              type="text"
              :placeholder="t('admin.upstreamProviders.searchProviders')"
              class="input pl-10"
              @input="handleSearch"
            />
          </div>

          <div class="w-full sm:w-36">
            <Select
              v-model="filters.status"
              :options="statusOptions"
              :placeholder="t('admin.upstreamProviders.allStatus')"
              @change="loadProviders"
            />
          </div>

          <div class="flex flex-1 flex-wrap items-center justify-end gap-2">
            <button
              @click="loadProviders"
              :disabled="loading"
              class="btn btn-secondary"
              :title="t('common.refresh')"
            >
              <Icon name="refresh" size="md" :class="loading ? 'animate-spin' : ''" />
            </button>
            <button
              @click="handleSyncAll"
              :disabled="syncingAll || loading"
              class="btn btn-secondary"
              :title="t('admin.upstreamProviders.syncAll')"
            >
              <Icon name="refresh" size="md" :class="['mr-2', syncingAll ? 'animate-spin' : '']" />
              {{ syncingAll ? t('admin.upstreamProviders.syncAllRunning') : t('admin.upstreamProviders.syncAll') }}
            </button>
            <button @click="openCreateModal" class="btn btn-primary">
              <Icon name="plus" size="md" class="mr-2" />
              {{ t('admin.upstreamProviders.createProvider') }}
            </button>
          </div>
        </div>

        <p class="mt-2 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.upstreamProviders.readOnlyHint') }}
        </p>
      </template>

      <template #table>
        <EmptyState
          v-if="!loading && providers.length === 0"
          :description="t('admin.upstreamProviders.noProviders')"
          :action-text="t('admin.upstreamProviders.createProvider')"
          @action="openCreateModal"
        />
        <UpstreamProviderTable
          v-else
          :providers="providers"
          :expanded-id="expandedId"
          :groups="expandedGroups"
          :groups-loading="groupsLoading"
          :syncing-id="syncingId"
          :testing-id="testingId"
          @toggle-groups="toggleGroups"
          @sync="handleSync"
          @test="handleTest"
          @provision="openProvisionModal"
          @edit="openEditModal"
          @delete="confirmDelete"
        />
      </template>

      <template #pagination>
        <Pagination
          v-if="total > 0"
          :page="page"
          :page-size="pageSize"
          :total="total"
          @update:page="handlePageChange"
          @update:page-size="handlePageSizeChange"
        />
      </template>
    </TablePageLayout>

    <!-- 跨上游拉平的分组比价表：横向比倍率的主视图 -->
    <div class="px-4 pb-8 sm:px-6">
      <UpstreamGroupComparisonTable
        ref="comparisonTable"
        @provision="openProvisionFromComparison"
      />
    </div>

    <UpstreamProviderFormDialog
      :show="showFormModal"
      :editing="editingProvider"
      :submitting="submitting"
      @close="closeFormModal"
      @submit="handleFormSubmit"
    />

    <UpstreamProvisionDialog
      :show="showProvisionModal"
      :provider="provisionProvider"
      :groups="provisionGroups"
      :groups-loading="provisionGroupsLoading"
      :local-groups="localGroups"
      :submitting="provisioning"
      :results="provisionResults"
      :preset-remote-group-ids="presetRemoteGroupIDs"
      @close="closeProvisionModal"
      @submit="handleProvision"
    />

    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('admin.upstreamProviders.deleteProvider')"
      :message="deleteMessage"
      :confirm-text="t('common.delete')"
      danger
      @confirm="handleDelete"
      @cancel="showDeleteConfirm = false"
    />
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminAPI } from '@/api/admin'
import type { AdminGroup } from '@/types'
import type {
  UpstreamProvider,
  UpstreamProviderWithStats,
  UpstreamGroup,
  UpstreamGroupComparison,
  ProvisionedAccount,
  CreateUpstreamProviderRequest,
  UpdateUpstreamProviderRequest,
  ProvisionAccountsRequest
} from '@/types/upstreamProvider'
import AppLayout from '@/components/layout/AppLayout.vue'
import TablePageLayout from '@/components/layout/TablePageLayout.vue'
import Pagination from '@/components/common/Pagination.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import EmptyState from '@/components/common/EmptyState.vue'
import Select from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import UpstreamProviderTable from '@/components/admin/upstream/UpstreamProviderTable.vue'
import UpstreamProviderFormDialog from '@/components/admin/upstream/UpstreamProviderFormDialog.vue'
import UpstreamProvisionDialog from '@/components/admin/upstream/UpstreamProvisionDialog.vue'
import UpstreamGroupComparisonTable from '@/components/admin/upstream/UpstreamGroupComparisonTable.vue'
import { getPersistedPageSize } from '@/composables/usePersistedPageSize'

const { t } = useI18n()
const appStore = useAppStore()

const providers = ref<UpstreamProviderWithStats[]>([])
const loading = ref(false)
const page = ref(1)
const pageSize = ref(getPersistedPageSize(20))
const total = ref(0)
const searchQuery = ref('')
const filters = reactive<{ status?: 'active' | 'inactive' }>({})

// 展开行显示分组明细
const expandedId = ref<number | null>(null)
const expandedGroups = ref<UpstreamGroup[]>([])
const groupsLoading = ref(false)

const syncingId = ref<number | null>(null)
const testingId = ref<number | null>(null)
const syncingAll = ref(false)

const showFormModal = ref(false)
const editingProvider = ref<UpstreamProviderWithStats | null>(null)
const submitting = ref(false)

const showProvisionModal = ref(false)
const provisionProvider = ref<UpstreamProviderWithStats | UpstreamProvider | null>(null)
const provisioning = ref(false)
const provisionResults = ref<ProvisionedAccount[]>([])
const localGroups = ref<AdminGroup[]>([])

// 建号弹窗用独立的分组状态，不复用展开行的 expandedGroups：
// 从比价表快捷建号时目标上游未必是当前展开的那个，共用会互相顶掉。
const provisionGroups = ref<UpstreamGroup[]>([])
const provisionGroupsLoading = ref(false)
// 快捷建号预勾的上游分组；从上游列表进来时为空
const presetRemoteGroupIDs = ref<number[]>([])

const showDeleteConfirm = ref(false)
const deletingProvider = ref<UpstreamProviderWithStats | null>(null)

// 同步后比价表要一起刷新，否则看到的还是旧倍率
const comparisonTable = ref<{ reload: () => Promise<void> } | null>(null)

const statusOptions = computed(() => [
  { value: '', label: t('admin.upstreamProviders.allStatus') },
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

const deleteMessage = computed(() =>
  deletingProvider.value
    ? t('admin.upstreamProviders.deleteConfirmMessage', { name: deletingProvider.value.name })
    : ''
)

let searchTimer: ReturnType<typeof setTimeout> | undefined
function handleSearch() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    page.value = 1
    loadProviders()
  }, 300)
}

async function loadProviders() {
  loading.value = true
  try {
    const response = await adminAPI.upstreamProviders.list(page.value, pageSize.value, {
      status: filters.status || undefined,
      search: searchQuery.value.trim() || undefined
    })
    providers.value = response.items ?? []
    total.value = response.total ?? 0
  } catch (error) {
    appStore.showError(resolveError(error))
  } finally {
    loading.value = false
  }
}

function handlePageChange(next: number) {
  page.value = next
  loadProviders()
}

function handlePageSizeChange(next: number) {
  pageSize.value = next
  page.value = 1
  loadProviders()
}

async function toggleGroups(provider: UpstreamProviderWithStats) {
  if (expandedId.value === provider.id) {
    expandedId.value = null
    expandedGroups.value = []
    return
  }
  expandedId.value = provider.id
  await loadGroups(provider.id)
}

async function loadGroups(providerID: number) {
  groupsLoading.value = true
  try {
    expandedGroups.value = await adminAPI.upstreamProviders.listGroups(providerID)
  } catch (error) {
    appStore.showError(resolveError(error))
    expandedGroups.value = []
  } finally {
    groupsLoading.value = false
  }
}

// 手动刷新单个上游的余额、并发与分组倍率
async function handleSync(provider: UpstreamProviderWithStats) {
  syncingId.value = provider.id
  try {
    await adminAPI.upstreamProviders.sync(provider.id)
    appStore.showSuccess(t('admin.upstreamProviders.syncSuccess'))
    await loadProviders()
    // 展开中的分组一并刷新，否则看到的还是旧倍率
    if (expandedId.value === provider.id) {
      await loadGroups(provider.id)
    }
    await comparisonTable.value?.reload()
  } catch (error) {
    appStore.showError(resolveError(error))
    // 同步失败时也要刷列表：失败原因已落 last_sync_error，要展示出来
    await loadProviders()
  } finally {
    syncingId.value = null
  }
}

async function handleSyncAll() {
  syncingAll.value = true
  try {
    const result = await adminAPI.upstreamProviders.syncAll()
    appStore.showSuccess(
      t('admin.upstreamProviders.syncAllResult', {
        succeeded: result.succeeded,
        failed: result.failed
      })
    )
    await loadProviders()
    if (expandedId.value !== null) {
      await loadGroups(expandedId.value)
    }
    await comparisonTable.value?.reload()
  } catch (error) {
    appStore.showError(resolveError(error))
  } finally {
    syncingAll.value = false
  }
}

async function handleTest(provider: UpstreamProviderWithStats) {
  testingId.value = provider.id
  try {
    const profile = await adminAPI.upstreamProviders.testConnection(provider.id)
    appStore.showSuccess(
      t('admin.upstreamProviders.testSuccess', {
        balance: profile.balance.toFixed(2),
        concurrency: profile.concurrency
      })
    )
  } catch (error) {
    appStore.showError(resolveError(error))
  } finally {
    testingId.value = null
  }
}

function openCreateModal() {
  editingProvider.value = null
  showFormModal.value = true
}

function openEditModal(provider: UpstreamProviderWithStats) {
  editingProvider.value = provider
  showFormModal.value = true
}

function closeFormModal() {
  showFormModal.value = false
  editingProvider.value = null
}

async function handleFormSubmit(
  payload: CreateUpstreamProviderRequest | UpdateUpstreamProviderRequest
) {
  submitting.value = true
  try {
    if (editingProvider.value) {
      await adminAPI.upstreamProviders.update(
        editingProvider.value.id,
        payload as UpdateUpstreamProviderRequest
      )
      appStore.showSuccess(t('admin.upstreamProviders.updateSuccess'))
    } else {
      await adminAPI.upstreamProviders.create(payload as CreateUpstreamProviderRequest)
      appStore.showSuccess(t('admin.upstreamProviders.createSuccess'))
    }
    closeFormModal()
    await loadProviders()
  } catch (error) {
    appStore.showError(resolveError(error))
  } finally {
    submitting.value = false
  }
}

async function openProvisionModal(provider: UpstreamProviderWithStats) {
  presetRemoteGroupIDs.value = []
  await openProvisionFor(provider)
}

// 比价表的快捷建号：直接拿这一行的上游 + 分组开建，不用回列表再勾一遍。
//
// 比价表是跨上游拉平的，行里的上游可能不在当前页的 providers 里，
// 所以先在已加载列表里找，找不到再按 id 拉一次。
async function openProvisionFromComparison(row: UpstreamGroupComparison) {
  const loaded = providers.value.find((item) => item.id === row.upstream_provider_id)
  presetRemoteGroupIDs.value = [row.remote_group_id]

  if (loaded) {
    await openProvisionFor(loaded)
    return
  }
  try {
    const provider = await adminAPI.upstreamProviders.getByID(row.upstream_provider_id)
    await openProvisionFor(provider)
  } catch (error) {
    presetRemoteGroupIDs.value = []
    appStore.showError(resolveError(error))
  }
}

async function openProvisionFor(provider: UpstreamProviderWithStats | UpstreamProvider) {
  provisionProvider.value = provider
  provisionResults.value = []
  provisionGroups.value = []
  showProvisionModal.value = true
  await Promise.all([loadProvisionGroups(provider.id), loadLocalGroups()])
}

async function loadProvisionGroups(providerID: number) {
  provisionGroupsLoading.value = true
  try {
    provisionGroups.value = await adminAPI.upstreamProviders.listGroups(providerID)
  } catch (error) {
    appStore.showError(resolveError(error))
    provisionGroups.value = []
  } finally {
    provisionGroupsLoading.value = false
  }
}

async function loadLocalGroups() {
  if (localGroups.value.length > 0) return
  try {
    localGroups.value = await adminAPI.groups.getAll()
  } catch {
    // 本地分组拉不到不影响建号：不绑分组也是合法选择
    localGroups.value = []
  }
}

function closeProvisionModal() {
  showProvisionModal.value = false
  provisionProvider.value = null
  provisionResults.value = []
  provisionGroups.value = []
  presetRemoteGroupIDs.value = []
}

async function handleProvision(payload: ProvisionAccountsRequest) {
  if (!provisionProvider.value) return
  provisioning.value = true
  try {
    const response = await adminAPI.upstreamProviders.provisionAccounts(
      provisionProvider.value.id,
      payload
    )
    provisionResults.value = response.results ?? []
    const failed = provisionResults.value.filter((item) => item.error).length
    const succeeded = provisionResults.value.length - failed
    if (failed === 0) {
      appStore.showSuccess(t('admin.upstreamProviders.provisionSummary', { succeeded, failed }))
    } else {
      appStore.showError(t('admin.upstreamProviders.provisionSummary', { succeeded, failed }))
    }
    await loadProviders()
    // 建号会改变比价表的「已建号」列，不刷的话还是旧计数。
    // 只在真的建成过才刷，全失败时没有任何计数变化。
    if (succeeded > 0) {
      await comparisonTable.value?.reload()
    }
  } catch (error) {
    appStore.showError(resolveError(error))
  } finally {
    provisioning.value = false
  }
}

function confirmDelete(provider: UpstreamProviderWithStats) {
  deletingProvider.value = provider
  showDeleteConfirm.value = true
}

async function handleDelete() {
  if (!deletingProvider.value) return
  try {
    await adminAPI.upstreamProviders.remove(deletingProvider.value.id)
    appStore.showSuccess(t('admin.upstreamProviders.deleteSuccess'))
    if (expandedId.value === deletingProvider.value.id) {
      expandedId.value = null
      expandedGroups.value = []
    }
    await loadProviders()
  } catch (error) {
    appStore.showError(resolveError(error))
  } finally {
    showDeleteConfirm.value = false
    deletingProvider.value = null
  }
}

function resolveError(error: unknown): string {
  if (error && typeof error === 'object' && 'message' in error) {
    return String((error as { message: unknown }).message)
  }
  return t('common.unknownError')
}

onMounted(loadProviders)
</script>
