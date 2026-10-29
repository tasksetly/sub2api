<template>
  <div class="space-y-3">
    <UpstreamFinanceTable :providers="providers" :loading="loading" :error="error">
      <template #actions>
        <button class="btn btn-secondary" :disabled="loading || syncing" @click="reload">{{ t('common.refresh') }}</button>
        <button class="btn btn-secondary" :disabled="loading || syncing" @click="syncAll">{{ t(syncing ? 'admin.upstreamProviders.syncAllRunning' : 'admin.upstreamProviders.syncAll') }}</button>
      </template>
    </UpstreamFinanceTable>
    <Pagination v-if="total > 0" :page="page" :page-size="20" :total="total" @update:page="changePage" />
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { upstreamProvidersAPI } from '@/api/admin/upstreamProviders'
import type { UpstreamProviderWithStats } from '@/types/upstreamProvider'
import Pagination from '@/components/common/Pagination.vue'
import UpstreamFinanceTable from './UpstreamFinanceTable.vue'
import { useAppStore } from '@/stores/app'
const { t } = useI18n()
const appStore = useAppStore()
const providers = ref<UpstreamProviderWithStats[]>([])
const loading = ref(false)
const syncing = ref(false)
const error = ref('')
const page = ref(1)
const total = ref(0)
let requestSeq = 0
async function reload() {
  const seq = ++requestSeq
  loading.value = true
  error.value = ''
  try {
    const result = await upstreamProvidersAPI.list(page.value, 20)
    if (seq !== requestSeq) return
    providers.value = result.items || []
    total.value = result.total
  } catch {
    if (seq !== requestSeq) return
    providers.value = []
    error.value = t('admin.upstreamProviders.financeLoadFailed')
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}
async function syncAll() {
  syncing.value = true
  try {
    const result = await upstreamProvidersAPI.syncAll()
    appStore.showSuccess(t('admin.upstreamProviders.syncAllResult', result))
  } catch {
    error.value = t('admin.upstreamProviders.financeSyncFailed')
    syncing.value = false
    return
  }
  syncing.value = false
  await reload()
}
function changePage(next: number) { page.value = next; void reload() }
defineExpose({ reload })
onMounted(() => { void reload() })
</script>
