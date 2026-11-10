<template>
  <div class="space-y-4">
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.accounts.svgTest.hint') }}</p>
    <form class="space-y-3" @submit.prevent="run">
      <div>
        <label class="input-label" for="svg-test-model">{{ t('admin.accounts.selectTestModel') }}</label>
        <Select id="svg-test-model" v-model="modelId" :options="textModels" value-key="id" label-key="display_name" searchable creatable :disabled="submitting || running" :placeholder="t('admin.accounts.svgTest.modelPlaceholder')" />
      </div>
      <TextArea id="svg-test-prompt" v-model="prompt" :label="t('admin.accounts.svgTest.prompt')" :placeholder="t('admin.accounts.svgTest.promptPlaceholder')" :hint="t('admin.accounts.svgTest.promptHint')" :rows="5" required :disabled="submitting || running" />
      <div>
        <label class="input-label" for="svg-test-effort">{{ t('admin.accounts.svgTest.effort') }}</label>
        <select id="svg-test-effort" v-model="effort" class="input" :disabled="submitting || running">
          <option value="">{{ t('admin.accounts.svgTest.defaultEffort') }}</option>
          <option v-for="value in ['low', 'medium', 'high', 'xhigh', 'max']" :key="value" :value="value">{{ value }}</option>
        </select>
      </div>
      <button type="submit" class="btn btn-primary" :disabled="!canRun">
        {{ t(submitting || running ? 'admin.accounts.testing' : 'admin.accounts.svgTest.run') }}
      </button>
    </form>
    <p v-if="running" class="text-sm text-primary-600" role="status">{{ t('admin.accounts.svgTest.runningHint') }}</p>
    <p v-if="error" class="text-sm text-red-600" role="alert">{{ error }}</p>

    <div class="space-y-2 border-t border-gray-200 pt-4 dark:border-dark-500">
      <div class="flex items-center justify-between">
        <h3 class="font-medium">{{ t('admin.accounts.svgTest.history') }}</h3>
        <button type="button" class="btn btn-secondary btn-sm" :disabled="loading || submitting" @click="load()">{{ t('common.refresh') }}</button>
      </div>
      <p v-if="!results.length" class="text-sm text-gray-500">{{ t(loading ? 'common.loading' : 'admin.accounts.svgTest.empty') }}</p>
      <div class="max-h-48 space-y-1 overflow-y-auto">
        <button v-for="item in results" :key="item.id" type="button" class="flex w-full items-center justify-between gap-2 rounded-lg border p-2 text-left text-xs" :class="selectedId === item.id ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20' : 'border-gray-200 dark:border-dark-500'" :aria-pressed="selectedId === item.id" @click="selectedId = item.id">
          <span>{{ item.model_id }} · {{ formatDateTime(item.created_at) }}</span>
          <span>{{ t(`admin.accounts.svgTest.status.${item.status}`) }}</span>
        </button>
      </div>
      <button v-if="hasMore" type="button" class="btn btn-secondary btn-sm" :disabled="loading || submitting" @click="load(true)">{{ t('admin.accounts.svgTest.loadMore') }}</button>
    </div>

    <div v-if="selected" class="space-y-2">
      <p class="whitespace-pre-wrap break-words text-sm text-gray-600 dark:text-gray-300">{{ selected.prompt_text }}</p>
      <p class="text-xs text-gray-500">{{ selected.model_id }} · {{ selected.effort || t('admin.accounts.svgTest.defaultEffort') }} · {{ selected.latency_ms }} ms</p>
      <p v-if="selected.error" class="text-sm text-red-600">{{ resultError(selected.error) }}</p>
      <template v-if="selected.svg_content">
        <iframe :srcdoc="getSVGPreviewDocument(selected.svg_content)" sandbox="" :title="t('admin.accounts.svgTest.preview')" class="h-96 w-full rounded-lg border border-gray-200 bg-white dark:border-dark-500" />
        <div class="flex gap-2">
          <button type="button" class="btn btn-secondary btn-sm" @click="copyToClipboard(selected.svg_content, t('admin.accounts.outputCopied'))">{{ t('admin.accounts.svgTest.copy') }}</button>
          <a class="btn btn-secondary btn-sm" :href="`data:image/svg+xml;charset=utf-8,${encodeURIComponent(selected.svg_content)}`" :download="`account-${account.id}-svg-${selected.id}.svg`">{{ t('admin.accounts.svgTest.download') }}</a>
        </div>
        <details>
          <summary class="cursor-pointer text-sm">{{ t('admin.accounts.svgTest.source') }}</summary>
          <pre class="max-h-64 overflow-auto rounded-lg bg-gray-100 p-3 text-xs dark:bg-dark-700">{{ selected.svg_content }}</pre>
        </details>
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select from '@/components/common/Select.vue'
import TextArea from '@/components/common/TextArea.vue'
import { adminAPI } from '@/api/admin'
import type { IntelligenceTestResult } from '@/api/admin/intelligenceTest'
import type { Account, ClaudeModel } from '@/types'
import { useClipboard } from '@/composables/useClipboard'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import { getSVGPreviewDocument } from '@/utils/svgPreview'

const props = defineProps<{ account: Account; models: ClaudeModel[]; loadingModels: boolean }>()
const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const modelId = ref('')
const prompt = ref('')
const effort = ref('')
const results = ref<IntelligenceTestResult[]>([])
const selectedId = ref<number | null>(null)
const selected = computed(() => results.value.find(item => item.id === selectedId.value))
const running = computed(() => results.value.some(item => item.status === 'running'))
const submitting = ref(false)
const loading = ref(false)
const loaded = ref(false)
const hasMore = ref(false)
const error = ref('')
const controller = new AbortController()
let timer: ReturnType<typeof setTimeout> | undefined
const textModels = computed(() => props.models.filter(item => !/gpt-image-|gemini-.*image|grok-imagine|grok-video/i.test(item.id)))
const canRun = computed(() => loaded.value && !loading.value && !submitting.value && !running.value && !props.loadingModels && !!modelId.value.trim() && !!prompt.value.trim() && Array.from(modelId.value.trim()).length <= 200 && Array.from(prompt.value.trim()).length <= 20000)
watch(textModels, models => { if (!modelId.value && models.length) modelId.value = models[0].id }, { immediate: true })

function resultError(message: string) {
  const keys: Record<string, string> = {
    svg_missing_or_invalid: 'invalidSVG',
    'benchmark interrupted': 'interrupted',
    'benchmark timed out': 'timedOut',
    'upstream test failed': 'upstreamFailed'
  }
  return keys[message] ? t(`admin.accounts.svgTest.${keys[message]}`) : message
}

async function load(older = false) {
  if (loading.value || controller.signal.aborted) return
  if (timer) clearTimeout(timer)
  loading.value = true
  error.value = ''
  try {
    const beforeId = older ? results.value[results.value.length - 1]?.id || 0 : 0
    const items = await adminAPI.intelligenceTest.listAccountResults(props.account.id, beforeId, controller.signal)
    if (controller.signal.aborted) return
    results.value = older ? [...results.value, ...items] : items
    hasMore.value = items.length === 20
    loaded.value = true
    if (!results.value.some(item => item.id === selectedId.value)) selectedId.value = items[0]?.id ?? null
  } catch (cause) {
    if (!controller.signal.aborted) error.value = extractApiErrorMessage(cause, t('common.error'))
  } finally {
    loading.value = false
    if (!controller.signal.aborted && running.value) timer = setTimeout(() => { void load() }, 3000)
  }
}

async function run() {
  if (!canRun.value) return
  if (timer) clearTimeout(timer)
  submitting.value = true
  error.value = ''
  try {
    const item = await adminAPI.intelligenceTest.runAccount(props.account.id, {
      model_id: modelId.value.trim(), prompt_text: prompt.value.trim(), reasoning_effort: effort.value
    })
    if (controller.signal.aborted) return
    results.value.unshift(item)
    selectedId.value = item.id
    await load()
  } catch (cause) {
    if (!controller.signal.aborted) {
      await load()
      error.value = extractApiErrorMessage(cause, t('common.error'))
    }
  } finally {
    submitting.value = false
  }
}

onMounted(() => { void load() })
onBeforeUnmount(() => {
  controller.abort()
  if (timer) clearTimeout(timer)
})
</script>
