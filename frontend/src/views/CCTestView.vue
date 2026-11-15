<template>
  <AppLayout>
    <div class="space-y-6">
      <div class="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h1 class="text-2xl font-bold text-gray-900 dark:text-white">{{ t('cctest.title') }}</h1>
          <p class="mt-2 text-sm text-gray-500">{{ t('cctest.description') }}</p>
        </div>
        <a href="https://cctest.ai/docs" target="_blank" rel="noopener noreferrer" class="btn btn-secondary">{{ t('cctest.officialDocs') }}</a>
      </div>

      <section v-if="admin && settings" class="card space-y-4 p-5">
        <h2 class="font-semibold">{{ t('cctest.settings') }}</h2>
        <p class="text-sm text-gray-500">{{ t('cctest.settingsHint') }}</p>
        <p v-if="!settings.encryption_key_configured" class="text-sm text-amber-600">{{ t('cctest.encryptionRequired') }}</p>
        <div class="flex flex-wrap gap-5">
          <label class="flex items-center gap-2"><input v-model="settings.enabled" type="checkbox" :disabled="saving" />{{ t('cctest.enabled') }}</label>
          <label class="flex items-center gap-2"><input v-model="settings.allow_users" type="checkbox" :disabled="saving" />{{ t('cctest.allowUsers') }}</label>
        </div>
        <div class="grid gap-4 md:grid-cols-2">
          <div>
            <label for="cctest-public-url" class="input-label">{{ t('cctest.publicURL') }}</label>
            <input id="cctest-public-url" v-model="settings.public_url" type="url" class="input" :disabled="saving" placeholder="https://example.com" />
          </div>
          <div>
            <label for="cctest-secret" class="input-label">{{ t('cctest.serviceKey') }}</label>
            <input id="cctest-secret" v-model="serviceKey" type="password" autocomplete="new-password" class="input" :disabled="saving" :placeholder="settings.has_api_key ? t('cctest.keepSecret') : 'cct-…'" />
          </div>
        </div>
        <div class="flex gap-3">
          <button class="btn btn-primary" :disabled="saving || loading || !settings.encryption_key_configured" @click="saveSettings">{{ t(saving ? 'common.saving' : 'common.save') }}</button>
          <button class="btn btn-secondary" :disabled="saving || loading" @click="loadConfig">{{ t('common.refresh') }}</button>
        </div>
        <p v-if="saved" class="text-sm text-green-600" role="status">{{ t('cctest.saved') }}</p>
      </section>

      <section class="card space-y-4 p-5">
        <p v-if="!enabled && !loading" class="text-sm text-amber-600">{{ t('cctest.disabled') }}</p>
        <template v-if="admin">
          <div class="flex flex-wrap items-center gap-3">
            <span>{{ t('cctest.selected', { count: accountIDs.length }) }}</span>
            <RouterLink to="/admin/accounts" class="btn btn-secondary">{{ t('cctest.selectAccounts') }}</RouterLink>
          </div>
          <p v-if="accountIDs.length" class="break-all text-xs text-gray-500">{{ accountIDs.join(', ') }}</p>
          <p v-if="!accountIDs.length" class="text-sm text-amber-600">{{ t('cctest.noAccounts') }}</p>
        </template>
        <div v-else>
          <label class="input-label" for="cctest-key">{{ t('playground.apiKey') }}</label>
          <Select id="cctest-key" v-model="selectedKeyID" :options="keyOptions" :disabled="submitting || loading" searchable :placeholder="t('playground.selectKey')" />
          <p v-if="!loading && !keyOptions.length" class="mt-2 text-sm text-amber-600">{{ t('playground.noKeys') }}</p>
        </div>
        <div>
          <label class="input-label" for="cctest-model">{{ t('playground.model') }}</label>
          <Select id="cctest-model" v-model="model" :options="modelOptions" :disabled="submitting || loadingModels" searchable creatable :placeholder="t('playground.selectModel')" />
          <p class="mt-2 text-xs text-gray-500">{{ t('cctest.modelHint') }}</p>
          <p v-if="modelsError" class="mt-1 text-sm text-red-600">{{ modelsError }}</p>
          <p v-else-if="admin && !loadingModels && accountIDs.length > 0 && models.length === 0" class="mt-1 text-sm text-amber-600">{{ t('cctest.noCommonModels') }}</p>
        </div>
        <label class="flex items-center gap-2 text-sm"><input v-model="tokenAudit" type="checkbox" :disabled="submitting" />{{ t('cctest.tokenAudit') }}</label>
        <div class="rounded-lg bg-amber-50 p-3 text-sm text-amber-900 dark:bg-amber-900/20 dark:text-amber-200">
          {{ t(admin ? 'cctest.adminCost' : 'cctest.userCost') }}
        </div>
        <label class="flex items-start gap-2 text-sm"><input v-model="accepted" type="checkbox" class="mt-1" :disabled="submitting" />{{ t('cctest.consent') }}</label>
        <button class="btn btn-primary" :disabled="!canStart" @click="start">{{ t(submitting ? 'cctest.submitting' : 'cctest.start') }}</button>
      </section>

      <p v-if="error" role="alert" class="rounded-lg bg-red-50 p-3 text-sm text-red-700 dark:bg-red-900/20 dark:text-red-300">{{ error }}</p>
      <section class="space-y-3">
        <div class="flex flex-wrap items-center justify-between gap-3">
          <h2 class="text-lg font-semibold">{{ t('cctest.history') }}</h2>
          <div class="flex gap-2">
            <button class="btn btn-secondary" :disabled="loadingHistory" @click="refresh">{{ t('common.refresh') }}</button>
            <button class="btn btn-secondary" :disabled="!runs.length" @click="exportResults">{{ t('cctest.export') }}</button>
          </div>
        </div>
        <p class="text-sm text-gray-500">{{ t('cctest.interpretation') }}</p>
        <p v-if="!runs.length" class="py-8 text-center text-gray-500">{{ t(loadingHistory ? 'common.loading' : 'cctest.empty') }}</p>
        <div v-for="run in runs" :key="run.id" class="card space-y-3 p-4">
          <div class="flex flex-wrap items-center justify-between gap-3">
            <div class="min-w-0">
              <div class="break-words font-medium">{{ run.target_name }} <span v-if="run.account_id" class="text-xs text-gray-500">#{{ run.account_id }}</span></div>
              <p class="break-all text-sm text-gray-500">{{ run.model }} · {{ formatDateTime(run.created_at) }}</p>
            </div>
            <div class="flex items-center gap-3">
              <span v-if="run.status === 'done' && typeof run.result.total === 'number'" class="text-2xl font-bold tabular-nums">{{ run.result.total }}<span class="text-sm text-gray-500"> / 100</span></span>
              <span class="text-sm">{{ t(`cctest.status.${run.status}`) }}</span>
              <button v-if="isActive(run)" class="btn btn-secondary btn-sm" :disabled="cancelling.has(run.id)" @click="cancel(run)">{{ t('cctest.cancel') }}</button>
            </div>
          </div>
          <p v-if="run.status === 'done' && run.result.verdictKey" class="text-sm">{{ verdictLabel(run.result.verdictKey) }}</p>
          <div v-if="isActive(run)" class="space-y-1">
            <progress class="h-2 w-full" :value="progress(run)" max="1" :aria-label="t('cctest.progress')" />
            <p class="text-xs text-gray-500">{{ run.result.stepName || t('cctest.status.queued') }} · {{ t('cctest.requests', { count: run.request_count }) }}</p>
          </div>
          <p v-if="run.error" class="break-words text-sm text-red-600">{{ run.error }}</p>
          <details v-if="run.status === 'done' || Object.keys(run.result.scores ?? {}).length" class="text-sm">
            <summary class="cursor-pointer text-primary-600">{{ t('cctest.details') }}</summary>
            <dl class="mt-3 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
              <div v-for="(score, name) in run.result.scores" :key="name" class="flex justify-between gap-2 rounded-lg bg-gray-50 p-2 dark:bg-dark-800">
                <dt>{{ dimensionLabel(String(name)) }}</dt><dd class="font-mono">{{ score }}</dd>
              </div>
            </dl>
            <p class="mt-3 break-all">{{ t('cctest.responseModel') }}: {{ run.result.responseModel || '—' }}</p>
            <p>{{ t('cctest.channel') }}: {{ run.result.streamChannel || '—' }} / {{ run.result.nonStreamChannel || '—' }}</p>
            <p v-if="run.result.metrics">{{ t('cctest.tokens') }}: {{ run.result.metrics.inputTokens ?? '—' }} / {{ run.result.metrics.outputTokens ?? '—' }}</p>
            <p v-if="run.result.tokenAudit">{{ t('cctest.auditRatio') }}: {{ run.result.tokenAudit.overallRatio ?? '—' }} · {{ t('cctest.cacheHitRate') }}: {{ run.result.tokenAudit.cacheHitRate ?? '—' }}%</p>
          </details>
        </div>
        <button v-if="hasMore" class="btn btn-secondary" :disabled="loadingHistory" @click="loadOlder">{{ t('cctest.loadMore') }}</button>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select from '@/components/common/Select.vue'
import { cctestAPI, type CCTestRun, type CCTestSettings } from '@/api/cctest'
import { keysAPI } from '@/api/keys'
import { adminAPI } from '@/api/admin'
import { listPlaygroundModels } from '@/api/playground'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatDateTime } from '@/utils/format'
import type { ApiKey } from '@/types'

const props = withDefaults(defineProps<{ admin?: boolean }>(), { admin: false })
const { t, te } = useI18n()
const route = useRoute()
const settings = ref<CCTestSettings | null>(null)
const serviceKey = ref('')
const enabled = ref(false)
const saved = ref(false)
const loading = ref(true)
const saving = ref(false)
const submitting = ref(false)
const accepted = ref(false)
const tokenAudit = ref(false)
const error = ref('')
const runs = ref<CCTestRun[]>([])
const loadingHistory = ref(false)
const hasMore = ref(false)
const cursor = ref('')
const cancelling = ref(new Set<string>())
const keys = ref<ApiKey[]>([])
const selectedKeyID = ref<number | null>(null)
const model = ref('')
const models = ref<string[]>([])
const loadingModels = ref(false)
const modelsError = ref('')
const controller = new AbortController()
let modelsVersion = 0
let timer: ReturnType<typeof setTimeout> | undefined

const accountIDs = computed(() => [...new Set(String(route.query.accounts ?? '').split(',').map(Number).filter(id => Number.isSafeInteger(id) && id > 0))])
const selectedKey = computed(() => keys.value.find(key => key.id === selectedKeyID.value))
const keyOptions = computed(() => keys.value.map(key => ({ value: key.id, label: key.group?.name ? `${key.name} · ${key.group.name}` : key.name })))
const modelOptions = computed(() => models.value.map(id => ({ value: id, label: id })))
const canStart = computed(() => enabled.value && !loading.value && !submitting.value && accepted.value && model.value.toLowerCase().includes('claude') && model.value.length <= 200 && (props.admin ? accountIDs.value.length > 0 && accountIDs.value.length <= 20 : !!selectedKeyID.value))
const isActive = (run: CCTestRun) => ['queued', 'starting', 'running'].includes(run.status)
const progress = (run: CCTestRun) => Math.min(1, Math.max(0, Number(run.result.progress) || 0))
const verdictLabel = (value: string) => te(`cctest.verdict.${value}`) ? t(`cctest.verdict.${value}`) : value
const dimensionLabel = (value: string) => te(`cctest.dimension.${value}`) ? t(`cctest.dimension.${value}`) : value
const showError = (cause: unknown) => { if (!controller.signal.aborted) error.value = extractApiErrorMessage(cause, t('cctest.failed')) }

async function loadConfig() {
  loading.value = true
  try {
    if (props.admin) {
      settings.value = await cctestAPI.settings()
      enabled.value = settings.value.enabled
    } else {
      enabled.value = (await cctestAPI.availability()).enabled
    }
  } catch (cause) { showError(cause) } finally { loading.value = false }
}

async function saveSettings() {
  if (!settings.value || saving.value) return
  saving.value = true; saved.value = false; error.value = ''
  try {
    settings.value = await cctestAPI.saveSettings(settings.value, serviceKey.value)
    enabled.value = settings.value.enabled
    serviceKey.value = ''; saved.value = true
  } catch (cause) { showError(cause) } finally { saving.value = false }
}

async function loadModels() {
  const version = ++modelsVersion
  models.value = []; model.value = ''; modelsError.value = ''
  loadingModels.value = true
  try {
    let ids: string[] = []
    if (props.admin && accountIDs.value.length > 0 && accountIDs.value.length <= 20) {
      const lists = await Promise.all(accountIDs.value.map(id => adminAPI.accounts.getAvailableModels(id)))
      ids = lists[0]?.map(item => item.id).filter(id => lists.every(list => list.some(item => item.id === id))) ?? []
    } else if (!props.admin && selectedKey.value?.key) {
      ids = (await listPlaygroundModels(selectedKey.value.key, { signal: controller.signal })).map(item => item.id)
    }
    if (version !== modelsVersion || controller.signal.aborted) return
    models.value = [...new Set(ids.filter(id => id.toLowerCase().includes('claude')))]
    model.value = models.value[0] ?? ''
  } catch (cause) {
    if (version === modelsVersion && !controller.signal.aborted) modelsError.value = extractApiErrorMessage(cause, t('cctest.failed'))
  } finally { if (version === modelsVersion) loadingModels.value = false }
}

function merge(items: CCTestRun[]) {
  const byID = new Map(runs.value.map(run => [run.id, run]))
  for (const item of items) byID.set(item.id, item)
  runs.value = [...byID.values()].sort((a, b) => b.created_at.localeCompare(a.created_at) || b.id.localeCompare(a.id))
}

async function refresh() {
  if (loadingHistory.value) return
  loadingHistory.value = true
  try {
    const items = await cctestAPI.list(props.admin, '', controller.signal)
    if (controller.signal.aborted) return
    merge(items)
    if (!cursor.value) { cursor.value = items[items.length - 1]?.id ?? ''; hasMore.value = items.length === 50 }
  } catch (cause) { showError(cause) } finally { loadingHistory.value = false }
}

async function loadOlder() {
  if (loadingHistory.value || !cursor.value) return
  loadingHistory.value = true
  try {
    const items = await cctestAPI.list(props.admin, cursor.value, controller.signal)
    if (controller.signal.aborted) return
    merge(items); cursor.value = items[items.length - 1]?.id ?? cursor.value; hasMore.value = items.length === 50
  } catch (cause) { showError(cause) } finally { loadingHistory.value = false }
}

async function start() {
  if (!canStart.value) return
  submitting.value = true; error.value = ''
  try {
    const items = await cctestAPI.create(props.admin, {
      ...(props.admin ? { account_ids: accountIDs.value } : { api_key_id: selectedKeyID.value! }),
      model: model.value.trim(), token_audit: tokenAudit.value
    })
    merge(items); accepted.value = false
  } catch (cause) { showError(cause) } finally { submitting.value = false }
}

async function cancel(run: CCTestRun) {
  if (cancelling.value.has(run.id)) return
  cancelling.value.add(run.id)
  try { await cctestAPI.cancel(props.admin, run.id); run.status = 'cancelled' }
  catch (cause) { showError(cause) }
  finally { cancelling.value.delete(run.id) }
}

function exportResults() {
  const blob = new Blob([JSON.stringify({ source: 'https://cctest.ai', exported_at: new Date().toISOString(), runs: runs.value }, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a'); link.href = url; link.download = 'cctest-results.json'; link.click()
  URL.revokeObjectURL(url)
}

async function poll() {
  if (controller.signal.aborted) return
  if (runs.value.some(isActive)) await refresh()
  if (!controller.signal.aborted) timer = setTimeout(poll, 4000)
}

watch(selectedKeyID, loadModels)
watch(accountIDs, loadModels)
onMounted(async () => {
  await Promise.all([loadConfig(), refresh(), (async () => {
    if (props.admin) { await loadModels(); return }
    try {
      let page = 1
      for (;;) {
        const data = await keysAPI.list(page, 100, { status: 'active' }, { signal: controller.signal })
        if (controller.signal.aborted) return
        keys.value.push(...data.items.filter(key => key.group_id != null))
        if (page * 100 >= data.total || data.items.length === 0) break
        page++
      }
      selectedKeyID.value = keys.value[0]?.id ?? null
    } catch (cause) { showError(cause) }
  })()])
  void poll()
})
onBeforeUnmount(() => { controller.abort(); if (timer) clearTimeout(timer); serviceKey.value = '' })
</script>
