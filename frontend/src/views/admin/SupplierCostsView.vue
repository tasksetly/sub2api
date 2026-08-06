<template>
  <AppLayout>
    <div class="space-y-6">
      <header>
        <h1 class="text-2xl font-semibold text-gray-900 dark:text-white">{{ t('usage.supplierCost.title') }}</h1>
        <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('usage.supplierCost.pageDescription') }}</p>
      </header>

      <section class="card p-4">
        <div class="flex flex-wrap items-end gap-4">
          <div class="w-full sm:w-auto">
            <label class="input-label">{{ t('usage.timeRange') }}</label>
            <DateRangePicker
              v-model:start-date="startDate"
              v-model:end-date="endDate"
              @change="onDateRangeChange"
            />
          </div>
          <div class="w-full sm:min-w-[240px] sm:max-w-[320px]">
            <label class="input-label">{{ t('usage.supplierCost.supplierFilter') }}</label>
            <Select
              v-model="supplierSelection"
              :options="supplierOptions"
              searchable
              @change="onSupplierChange"
            />
          </div>
          <button
            type="button"
            class="btn btn-secondary"
            :disabled="loading"
            :title="t('common.refresh')"
            @click="loadSupplierCosts"
          >
            <Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" />
            <span>{{ t('common.refresh') }}</span>
          </button>
        </div>
      </section>

      <SupplierCostTable :rows="supplierCosts" :loading="loading" />
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'
import { adminUsageAPI } from '@/api/admin/usage'
import type { SupplierCostStat } from '@/types'
import AppLayout from '@/components/layout/AppLayout.vue'
import DateRangePicker from '@/components/common/DateRangePicker.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import SupplierCostTable from '@/components/admin/usage/SupplierCostTable.vue'

const { t } = useI18n()
const appStore = useAppStore()

const formatLocalDate = (date: Date): string => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}

const getLast24HoursRangeDates = (): { start: string; end: string } => {
  const end = new Date()
  const start = new Date(end.getTime() - 24 * 60 * 60 * 1000)
  return {
    start: formatLocalDate(start),
    end: formatLocalDate(end),
  }
}

const defaultRange = getLast24HoursRangeDates()
const startDate = ref(defaultRange.start)
const endDate = ref(defaultRange.end)
const supplierSelection = ref<string | null>(null)
const supplierCosts = ref<SupplierCostStat[]>([])
const knownSuppliers = ref<Set<string>>(new Set())
const loading = ref(false)
let requestSeq = 0

const supplierOptions = computed<SelectOption[]>(() => [
  { value: null, label: t('usage.supplierCost.allSuppliers') },
  { value: '', label: t('usage.supplierCost.unset') },
  ...Array.from(knownSuppliers.value)
    .sort((a, b) => a.localeCompare(b))
    .map((supplier) => ({ value: supplier, label: supplier })),
])

const rememberSuppliers = (rows: SupplierCostStat[]) => {
  const next = new Set(knownSuppliers.value)
  rows.forEach((row) => {
    const supplier = row.supplier.trim()
    if (supplier) next.add(supplier)
  })
  knownSuppliers.value = next
}

const loadSupplierCosts = async () => {
  const seq = ++requestSeq
  loading.value = true
  try {
    const selection = supplierSelection.value
    const response = await adminUsageAPI.getSupplierCosts({
      supplier: selection && selection.length > 0 ? selection : undefined,
      supplier_unset: selection === '' ? true : undefined,
      start_date: startDate.value,
      end_date: endDate.value,
    })
    if (seq !== requestSeq) return
    supplierCosts.value = response.suppliers || []
    rememberSuppliers(supplierCosts.value)
  } catch (error) {
    if (seq !== requestSeq) return
    console.error('Failed to load supplier cost stats:', error)
    supplierCosts.value = []
    appStore.showError(t('usage.supplierCost.loadFailed'))
  } finally {
    if (seq === requestSeq) loading.value = false
  }
}

const onDateRangeChange = (range: { startDate: string; endDate: string; preset: string | null }) => {
  startDate.value = range.startDate
  endDate.value = range.endDate
  void loadSupplierCosts()
}

const onSupplierChange = () => {
  void loadSupplierCosts()
}

onMounted(() => {
  void loadSupplierCosts()
})
</script>
