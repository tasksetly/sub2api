<template>
  <div class="space-y-5">
    <details class="rounded-xl border border-blue-200 bg-blue-50 p-4 dark:border-blue-900 dark:bg-blue-950/40">
      <summary class="cursor-pointer text-sm font-semibold text-blue-900 dark:text-blue-100">{{ t('intelligenceTest.question') }}</summary>
      <p class="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-gray-700 dark:text-gray-300">{{ response.config?.prompt_text }}</p>
      <p class="mt-3 text-sm font-medium text-blue-800 dark:text-blue-200">{{ t('intelligenceTest.rule') }}</p>
    </details>

    <div class="flex flex-wrap items-center justify-between gap-3">
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('intelligenceTest.failureNote') }}</p>
      <label class="flex items-center gap-2 text-sm">
        <span>{{ t('intelligenceTest.groups') }}</span>
        <select v-model="selectedGroup" class="input w-auto max-w-64">
          <option value="">{{ t('intelligenceTest.allGroups') }}</option>
          <option v-for="group in groups" :key="group.key" :value="group.key">{{ group.name }}</option>
        </select>
      </label>
    </div>

    <div v-if="loading" role="status" class="py-12 text-center text-gray-500">{{ t('intelligenceTest.loading') }}</div>
    <div v-else-if="error" role="alert" class="rounded-xl border border-red-200 p-6 text-center dark:border-red-900">
      <p class="text-red-600 dark:text-red-300">{{ t('intelligenceTest.loadFailed') }}</p>
      <p class="mt-2 text-sm text-gray-500">{{ error }}</p>
      <button class="btn btn-secondary mt-4" type="button" @click="$emit('refresh')">{{ t('intelligenceTest.retry') }}</button>
    </div>
    <template v-else>
      <div class="grid grid-cols-2 gap-3 lg:grid-cols-4" data-testid="candy-summary">
        <div v-for="metric in metrics" :key="metric.label" class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
          <p class="text-xs text-gray-500 dark:text-gray-400">{{ metric.label }}</p>
          <p class="mt-2 text-2xl font-semibold tabular-nums">{{ metric.value }}</p>
        </div>
      </div>
      <div class="flex flex-wrap gap-4 text-xs text-gray-600 dark:text-gray-300">
        <span v-for="status in (['validated', 'incorrect', 'failed'] as const)" :key="status" class="flex items-center gap-1.5">
          <span class="h-2.5 w-2.5 rounded-full" :class="statusColor(status)"></span>{{ statusLabel(status) }}
        </span>
      </div>
      <div v-if="!visibleGroups.length" class="rounded-xl border border-dashed border-gray-300 p-10 text-center dark:border-dark-600">
        <p class="font-medium">{{ t('intelligenceTest.empty') }}</p>
        <p class="mt-2 text-sm text-gray-500">{{ t('intelligenceTest.emptyHint') }}</p>
      </div>
      <section v-for="group in visibleGroups" :key="group.key" class="rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-900">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div>
            <h2 class="font-semibold">{{ group.name }}</h2>
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('intelligenceTest.total') }} {{ group.items.length }} · {{ t('intelligenceTest.passRate') }} {{ summary(group.items).rate }}</p>
          </div>
          <button v-if="group.items[0]" type="button" class="text-sm underline underline-offset-4" @click="selected = group.items[0]">
            {{ t('intelligenceTest.latest') }}: {{ statusLabel(group.items[0].status) }}
          </button>
          <span v-else class="text-sm text-gray-400">{{ t('intelligenceTest.noData') }}</span>
        </div>
        <div class="mt-4 flex gap-1 overflow-x-auto py-1" :aria-label="t('intelligenceTest.history')">
          <button v-for="item in [...group.items].reverse()" :key="item.id" type="button" class="h-8 min-w-3 max-w-10 flex-1 rounded-sm transition-opacity hover:opacity-70 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500" :class="statusColor(item.status)" :title="runLabel(item)" :aria-label="runLabel(item)" data-testid="candy-run" @click="selected = item"></button>
        </div>
        <div v-if="group.items.length" class="mt-2 flex justify-between text-xs text-gray-400">
          <time>{{ formatDate(group.items[group.items.length - 1]) }}</time><time>{{ formatDate(group.items[0]) }}</time>
        </div>
      </section>
    </template>

    <BaseDialog :show="!!selected" :title="t('intelligenceTest.detail')" width="wide" @close="selected = null">
      <template v-if="selected">
        <dl class="grid grid-cols-2 gap-4 text-sm sm:grid-cols-3">
          <div><dt class="text-gray-500">{{ t('intelligenceTest.groups') }}</dt><dd class="mt-1 break-words">{{ selected.group_name || t('intelligenceTest.unknownGroup') }}</dd></div>
          <div><dt class="text-gray-500">{{ t('intelligenceTest.model') }}</dt><dd class="mt-1 break-words">{{ selected.model_name || selected.model_id || t('intelligenceTest.unknownModel') }}</dd></div>
          <div><dt class="text-gray-500">{{ t('intelligenceTest.status') }}</dt><dd class="mt-1">{{ statusLabel(selected.status) }}</dd></div>
          <div><dt class="text-gray-500">{{ t('intelligenceTest.effort') }}</dt><dd class="mt-1">{{ selected.effort || t('intelligenceTest.defaultEffort') }}</dd></div>
          <div><dt class="text-gray-500">{{ t('intelligenceTest.latency') }}</dt><dd class="mt-1">{{ selected.latency_ms != null ? `${(selected.latency_ms / 1000).toFixed(1)}s` : '—' }}</dd></div>
          <div><dt class="text-gray-500">{{ t('intelligenceTest.time') }}</dt><dd class="mt-1">{{ formatDate(selected) }}</dd></div>
        </dl>
        <h3 class="mt-5 text-sm font-semibold">{{ t('intelligenceTest.response') }}</h3>
        <pre class="mt-2 max-h-96 overflow-y-auto whitespace-pre-wrap break-words rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-800" data-testid="candy-answer">{{ selected.response_text || t('intelligenceTest.noResponse') }}</pre>
        <p class="mt-3 text-xs text-gray-500">{{ t('intelligenceTest.rule') }}</p>
      </template>
    </BaseDialog>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import type { IntelligenceTestItem, IntelligenceTestResponse } from '@/api/intelligenceTest'

const props = defineProps<{ response: IntelligenceTestResponse; loading: boolean; error: string }>()
defineEmits<{ refresh: [] }>()
const { t, locale } = useI18n()
const selectedGroup = ref('')
const selected = ref<IntelligenceTestItem | null>(null)

const groups = computed(() => {
  const result = new Map<string, { key: string; name: string; items: IntelligenceTestItem[] }>()
  for (const group of props.response.groups || []) {
    const name = typeof group === 'string' ? group : group.name || t('intelligenceTest.unknownGroup')
    const key = typeof group === 'object' && group.id != null ? String(group.id) : name
    result.set(key, { key, name, items: [] })
  }
  for (const item of props.response.items || []) {
    if (item.answer_mode !== 'candy') continue
    const name = item.group_name || t('intelligenceTest.unknownGroup')
    const key = item.group_id != null ? String(item.group_id) : name
    if (!result.has(key)) result.set(key, { key, name, items: [] })
    result.get(key)!.items.push(item)
  }
  for (const group of result.values()) group.items.sort((a, b) => (b.created_at || '').localeCompare(a.created_at || '') || Number(b.id) - Number(a.id))
  return [...result.values()]
})
watch(groups, (value) => {
  if (!value.some(group => group.key === selectedGroup.value)) selectedGroup.value = ''
})
const visibleGroups = computed(() => groups.value.filter(group => !selectedGroup.value || group.key === selectedGroup.value))

function summary(items: IntelligenceTestItem[]) {
  const passed = items.filter(item => item.status === 'validated').length
  const incorrect = items.filter(item => item.status === 'incorrect').length
  const failed = items.filter(item => item.status === 'failed').length
  return { total: items.length, incorrect, failed, rate: passed + incorrect ? `${(passed / (passed + incorrect) * 100).toFixed(1)}%` : '—' }
}
const metrics = computed(() => {
  const totals = summary(visibleGroups.value.flatMap(group => group.items))
  return [
    { label: t('intelligenceTest.total'), value: totals.total },
    { label: t('intelligenceTest.passRate'), value: totals.rate },
    { label: t('intelligenceTest.incorrect'), value: totals.incorrect },
    { label: t('intelligenceTest.failed'), value: totals.failed },
  ]
})
function statusLabel(status?: string) {
  if (status === 'validated') return t('intelligenceTest.validated')
  if (status === 'incorrect') return t('intelligenceTest.incorrect')
  if (status === 'running') return t('intelligenceTest.running')
  return t('intelligenceTest.failed')
}
function statusColor(status?: string) {
  if (status === 'validated') return 'bg-emerald-500'
  if (status === 'incorrect') return 'bg-red-500'
  if (status === 'running') return 'bg-blue-300'
  return 'bg-gray-300 dark:bg-gray-600'
}
function formatDate(item: IntelligenceTestItem) {
  const date = new Date(item.created_at || '')
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString(locale.value, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
function runLabel(item: IntelligenceTestItem) {
  return `${formatDate(item)} · ${item.model_name || item.model_id || ''} · ${statusLabel(item.status)}`
}
</script>
