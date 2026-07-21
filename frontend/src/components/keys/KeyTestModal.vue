<template>
  <BaseDialog
    :show="show"
    :title="t('keys.test.title')"
    width="normal"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div
        v-if="apiKey"
        class="flex items-center justify-between rounded-lg border border-gray-200 bg-gray-50 p-3 dark:border-dark-600 dark:bg-dark-700"
      >
        <div class="flex min-w-0 items-center gap-3">
          <div class="flex h-10 w-10 flex-none items-center justify-center rounded-lg bg-primary-500">
            <Icon name="key" size="md" class="text-white" :stroke-width="2" />
          </div>
          <div class="min-w-0">
            <div class="truncate font-semibold text-gray-900 dark:text-gray-100">
              {{ apiKey.name }}
            </div>
            <div class="flex items-center gap-1.5 text-xs text-gray-500 dark:text-gray-400">
              <span
                v-if="apiKey.group?.platform"
                class="rounded bg-gray-200 px-1.5 py-0.5 text-[10px] font-medium uppercase dark:bg-dark-500"
              >
                {{ apiKey.group.platform }}
              </span>
              <span class="truncate">{{ apiKey.group?.name || t('keys.noGroup') }}</span>
            </div>
          </div>
        </div>
        <span
          :class="[
            'ml-3 flex-none rounded-full px-2.5 py-1 text-xs font-semibold',
            apiKey.status === 'active'
              ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
              : 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-400'
          ]"
        >
          {{ t(`keys.status.${apiKey.status}`) }}
        </span>
      </div>

      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('keys.test.selectModel') }}
        </label>
        <Select
          v-model="selectedModelId"
          :options="availableModels"
          :disabled="loadingModels || status === 'connecting'"
          value-key="id"
          label-key="display_name"
          :placeholder="loadingModels ? t('keys.test.loadingModels') : t('keys.test.selectModel')"
        />
      </div>

      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('keys.test.requestMode') }}
        </label>
        <div
          class="grid grid-cols-2 gap-1 rounded-lg bg-gray-100 p-1 dark:bg-dark-700"
          role="group"
          :aria-label="t('keys.test.requestMode')"
        >
          <button
            type="button"
            :disabled="status === 'connecting'"
            :aria-pressed="requestMode === 'chat'"
            :class="[
              'rounded-md px-3 py-2 text-sm font-medium transition-colors',
              requestMode === 'chat'
                ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white'
                : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            ]"
            @click="requestMode = 'chat'"
          >
            Chat Completions
          </button>
          <button
            type="button"
            :disabled="status === 'connecting'"
            :aria-pressed="requestMode === 'responses'"
            :class="[
              'rounded-md px-3 py-2 text-sm font-medium transition-colors',
              requestMode === 'responses'
                ? 'bg-white text-gray-900 shadow-sm dark:bg-dark-600 dark:text-white'
                : 'text-gray-500 hover:text-gray-800 dark:text-gray-400 dark:hover:text-gray-200'
            ]"
            @click="requestMode = 'responses'"
          >
            Responses
          </button>
        </div>
      </div>

      <div class="group relative">
        <div
          ref="terminalRef"
          class="max-h-[260px] min-h-[140px] overflow-y-auto rounded-lg border border-gray-700 bg-gray-900 p-4 font-mono text-sm dark:border-gray-800 dark:bg-black"
        >
          <div v-if="status === 'idle'" class="flex items-center gap-2 text-gray-500">
            <Icon name="play" size="sm" :stroke-width="2" />
            <span>{{ t('keys.test.ready') }}</span>
          </div>
          <div v-else-if="status === 'connecting'" class="flex items-center gap-2 text-yellow-400">
            <Icon name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            <span>{{ t('keys.test.connecting') }}</span>
          </div>

          <div v-for="(line, index) in outputLines" :key="index" :class="line.class">
            {{ line.text }}
          </div>

          <div v-if="streamingContent" class="whitespace-pre-wrap text-green-400">
            {{ streamingContent }}<span class="animate-pulse">_</span>
          </div>

          <div
            v-if="status === 'success'"
            class="mt-3 flex items-center gap-2 border-t border-gray-700 pt-3 text-green-400"
          >
            <Icon name="check" size="sm" :stroke-width="2" />
            <span>{{ t('keys.test.completed') }}</span>
          </div>
          <div
            v-else-if="status === 'error'"
            class="mt-3 flex items-start gap-2 border-t border-gray-700 pt-3 text-red-400"
          >
            <Icon name="x" size="sm" class="mt-0.5 flex-none" :stroke-width="2" />
            <span class="break-words">{{ errorMessage }}</span>
          </div>
        </div>

        <button
          v-if="outputLines.length > 0 || streamingContent"
          type="button"
          class="absolute right-2 top-2 rounded-lg bg-gray-800/90 p-1.5 text-gray-400 opacity-0 transition-all hover:bg-gray-700 hover:text-white focus:opacity-100 group-hover:opacity-100"
          :title="t('keys.test.copyOutput')"
          @click="copyOutput"
        >
          <Icon name="clipboard" size="sm" :stroke-width="2" />
        </button>
      </div>

      <div class="flex items-center justify-between px-1 text-xs text-gray-500 dark:text-gray-400">
        <span class="flex items-center gap-1">
          <Icon name="grid" size="sm" :stroke-width="2" />
          {{ t('keys.test.testModel') }}
        </span>
        <span class="flex items-center gap-1">
          <Icon name="chat" size="sm" :stroke-width="2" />
          {{ requestEndpoint }}
        </span>
      </div>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" @click="handleClose">
          {{ t('common.close') }}
        </button>
        <button
          type="button"
          :disabled="loadingModels || status === 'connecting' || !selectedModelId"
          :class="[
            'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition-colors',
            loadingModels || status === 'connecting' || !selectedModelId
              ? 'cursor-not-allowed bg-primary-400 text-white'
              : status === 'success'
                ? 'bg-green-500 text-white hover:bg-green-600'
                : status === 'error'
                  ? 'bg-orange-500 text-white hover:bg-orange-600'
                  : 'bg-primary-500 text-white hover:bg-primary-600'
          ]"
          @click="startTest"
        >
          <Icon
            v-if="loadingModels || status === 'connecting'"
            name="refresh"
            size="sm"
            class="animate-spin"
            :stroke-width="2"
          />
          <Icon v-else-if="status === 'idle'" name="play" size="sm" :stroke-width="2" />
          <Icon v-else name="refresh" size="sm" :stroke-width="2" />
          <span>
            {{
              loadingModels
                ? t('keys.test.loadingModels')
                : status === 'connecting'
                  ? t('keys.test.testing')
                  : status === 'idle'
                    ? t('keys.test.start')
                    : t('keys.test.retry')
            }}
          </span>
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { computed, nextTick, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'
import { Icon } from '@/components/icons'
import { useClipboard } from '@/composables/useClipboard'
import { buildGatewayUrl } from '@/api/client'
import type { ApiKey, ClaudeModel } from '@/types'

interface OutputLine {
  text: string
  class: string
}

interface GatewayModel {
  id?: unknown
  type?: unknown
  object?: unknown
  display_name?: unknown
  created_at?: unknown
}

const props = defineProps<{
  show: boolean
  apiKey: ApiKey | null
}>()

const emit = defineEmits<{
  (event: 'close'): void
}>()

const { t } = useI18n()
const { copyToClipboard } = useClipboard()
const terminalRef = ref<HTMLElement | null>(null)
const status = ref<'idle' | 'connecting' | 'success' | 'error'>('idle')
const requestMode = ref<'chat' | 'responses'>('chat')
const requestEndpoint = computed(() =>
  requestMode.value === 'chat' ? '/v1/chat/completions' : '/v1/responses'
)
const loadingModels = ref(false)
const availableModels = ref<ClaudeModel[]>([])
const selectedModelId = ref('')
const outputLines = ref<OutputLine[]>([])
const streamingContent = ref('')
const errorMessage = ref('')
let abortController: AbortController | null = null

const prioritizedGeminiModels = [
  'gemini-3.5-flash',
  'gemini-2.5-flash',
  'gemini-2.5-pro',
  'gemini-3-flash-preview',
  'gemini-3-pro-preview',
  'gemini-2.0-flash'
]

watch(
  () => props.show,
  async (show) => {
    if (show && props.apiKey) {
      requestMode.value = 'chat'
      resetState()
      await loadAvailableModels()
      return
    }
    abortRequest()
  }
)

function resetState() {
  status.value = 'idle'
  outputLines.value = []
  streamingContent.value = ''
  errorMessage.value = ''
}

function abortRequest() {
  abortController?.abort()
  abortController = null
}

function handleClose() {
  abortRequest()
  emit('close')
}

function addLine(text: string, className = 'text-gray-300') {
  outputLines.value.push({ text, class: className })
  void scrollToBottom()
}

async function scrollToBottom() {
  await nextTick()
  if (terminalRef.value) {
    terminalRef.value.scrollTop = terminalRef.value.scrollHeight
  }
}

function isDedicatedImageModel(modelID: string) {
  const normalized = modelID.toLowerCase()
  return normalized.startsWith('gpt-image-') || /-image(?:-|$)/.test(normalized)
}

function normalizeModels(payload: unknown): ClaudeModel[] {
  const body = payload as { data?: unknown }
  const rawModels = Array.isArray(payload) ? payload : body && Array.isArray(body.data) ? body.data : []
  const models = rawModels
    .map((raw) => raw as GatewayModel)
    .filter((raw) => typeof raw.id === 'string' && raw.id.trim().length > 0)
    .map((raw) => ({
      id: String(raw.id),
      type: String(raw.type || raw.object || 'model'),
      display_name: String(raw.display_name || raw.id),
      created_at: String(raw.created_at || '')
    }))

  const textModels = models.filter((model) => !isDedicatedImageModel(model.id))
  return textModels.length > 0 ? textModels : models
}

function sortModels(models: ClaudeModel[]) {
  if (props.apiKey?.group?.platform !== 'gemini') return models
  const priority = new Map(prioritizedGeminiModels.map((id, index) => [id, index]))
  return [...models].sort((a, b) => {
    return (priority.get(a.id) ?? Number.MAX_SAFE_INTEGER) -
      (priority.get(b.id) ?? Number.MAX_SAFE_INTEGER)
  })
}

function selectDefaultModel(models: ClaudeModel[]) {
  const sonnet = models.find((model) => model.id.toLowerCase().includes('sonnet'))
  return sonnet?.id || models[0]?.id || ''
}

async function responseError(response: Response) {
  let message = ''
  try {
    const text = await response.text()
    if (text) {
      try {
        const payload = JSON.parse(text) as {
          error?: string | { message?: string }
          message?: string
          detail?: string
        }
        message = typeof payload.error === 'string'
          ? payload.error
          : payload.error?.message || payload.message || payload.detail || ''
      } catch {
        message = text.trim()
      }
    }
  } catch {
    // Keep the status-based fallback below.
  }
  return message || t('keys.test.httpError', { status: response.status })
}

async function loadAvailableModels() {
  const key = props.apiKey
  availableModels.value = []
  selectedModelId.value = ''

  if (!key?.group_id) {
    setLoadError(t('keys.test.noGroup'))
    return
  }
  if (key.status !== 'active') {
    setLoadError(t('keys.test.keyUnavailable'))
    return
  }

  loadingModels.value = true
  const controller = new AbortController()
  abortController = controller
  try {
    const response = await fetch(buildGatewayUrl('/v1/models'), {
      headers: {
        Authorization: `Bearer ${key.key}`,
        Accept: 'application/json'
      },
      signal: controller.signal
    })
    if (!response.ok) {
      throw new Error(await responseError(response))
    }

    const models = sortModels(normalizeModels(await response.json()))
    if (models.length === 0) {
      throw new Error(t('keys.test.noModels'))
    }
    availableModels.value = models
    selectedModelId.value = selectDefaultModel(models)
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') return
    const message = error instanceof Error ? error.message : t('keys.test.loadModelsFailed')
    setLoadError(message)
  } finally {
    if (abortController === controller) {
      abortController = null
      loadingModels.value = false
    }
  }
}

function setLoadError(message: string) {
  status.value = 'error'
  errorMessage.value = message
  addLine(t('keys.test.loadModelsFailed'), 'text-red-400')
}

function objectErrorMessage(value: unknown) {
  if (!value || typeof value !== 'object') return ''
  const error = value as Record<string, unknown>
  return typeof error.message === 'string' ? error.message : ''
}

function gatewayEventError(event: Record<string, unknown>) {
  if (typeof event.error === 'string') return event.error
  const directError = objectErrorMessage(event.error)
  if (directError) return directError

  if (event.type === 'error' && typeof event.message === 'string') return event.message
  if (event.type === 'response.failed') {
    const response = event.response && typeof event.response === 'object'
      ? event.response as Record<string, unknown>
      : null
    return objectErrorMessage(response?.error) || t('keys.test.responseFailed')
  }
  return ''
}

function chatEventContent(event: Record<string, unknown>) {
  if (!Array.isArray(event.choices)) return { text: '', finished: false }

  let text = ''
  let finished = false
  for (const rawChoice of event.choices) {
    if (!rawChoice || typeof rawChoice !== 'object') continue
    const choice = rawChoice as Record<string, unknown>
    const delta = choice.delta && typeof choice.delta === 'object'
      ? choice.delta as Record<string, unknown>
      : null
    const message = choice.message && typeof choice.message === 'object'
      ? choice.message as Record<string, unknown>
      : null
    const content = delta?.content ?? message?.content
    if (typeof content === 'string') text += content
    if (typeof choice.finish_reason === 'string' && choice.finish_reason) finished = true
  }
  return { text, finished }
}

function responsesEventContent(event: Record<string, unknown>) {
  const type = typeof event.type === 'string' ? event.type : ''
  const text = type === 'response.output_text.delta' && typeof event.delta === 'string'
    ? event.delta
    : ''
  return {
    text,
    finished: type === 'response.completed'
  }
}

function eventContent(event: Record<string, unknown>, mode: 'chat' | 'responses') {
  return mode === 'chat' ? chatEventContent(event) : responsesEventContent(event)
}

async function startTest() {
  const key = props.apiKey
  if (!key || !selectedModelId.value) return

  resetState()
  status.value = 'connecting'
  addLine(t('keys.test.starting', { name: key.name }), 'text-blue-400')
  addLine(t('keys.test.groupLabel', { group: key.group?.name || t('keys.noGroup') }), 'text-gray-400')
  addLine('', 'text-gray-300')

  abortRequest()
  const controller = new AbortController()
  abortController = controller
  const mode = requestMode.value
  const endpoint = mode === 'chat' ? '/v1/chat/completions' : '/v1/responses'
  const requestBody = mode === 'chat'
    ? {
        model: selectedModelId.value,
        messages: [{ role: 'user', content: 'hi' }],
        stream: true
      }
    : {
        model: selectedModelId.value,
        input: [{
          role: 'user',
          content: [{ type: 'input_text', text: 'hi' }]
        }],
        stream: true
      }

  try {
    const response = await fetch(buildGatewayUrl(endpoint), {
      method: 'POST',
      headers: {
        Authorization: `Bearer ${key.key}`,
        'Content-Type': 'application/json',
        Accept: 'text/event-stream'
      },
      body: JSON.stringify(requestBody),
      signal: controller.signal
    })

    if (!response.ok) {
      throw new Error(await responseError(response))
    }
    const reader = response.body?.getReader()
    if (!reader) {
      throw new Error(t('keys.test.noResponseBody'))
    }

    addLine(t('keys.test.connected'), 'text-green-400')
    addLine(t('keys.test.usingModel', { model: selectedModelId.value }), 'text-cyan-400')
    addLine(t('keys.test.sending'), 'text-gray-400')
    addLine('', 'text-gray-300')
    addLine(t('keys.test.response'), 'text-yellow-400')

    const decoder = new TextDecoder()
    let buffer = ''
    let sawJSON = false
    let finished = false

    const processLine = (line: string) => {
      const match = /^data:\s*(.*)$/.exec(line.trim())
      if (!match || !match[1]) return
      if (match[1] === '[DONE]') {
        finished = true
        return
      }

      let event: Record<string, unknown>
      try {
        event = JSON.parse(match[1]) as Record<string, unknown>
      } catch {
        throw new Error(t('keys.test.invalidStream'))
      }
      sawJSON = true

      const eventError = gatewayEventError(event)
      if (eventError) throw new Error(eventError)

      const content = eventContent(event, mode)
      if (content.text) {
        streamingContent.value += content.text
        void scrollToBottom()
      }
      finished = finished || content.finished
    }

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split('\n')
      buffer = lines.pop() || ''
      for (const line of lines) processLine(line)
    }
    buffer += decoder.decode()
    if (buffer.trim()) processLine(buffer)

    if (!sawJSON || !finished) {
      throw new Error(t('keys.test.streamEndedUnexpectedly'))
    }
    if (streamingContent.value) {
      addLine(streamingContent.value, 'whitespace-pre-wrap text-green-300')
      streamingContent.value = ''
    }
    addLine('', 'text-gray-300')
    addLine(t('keys.test.gatewayVerified', { endpoint }), 'text-cyan-300')
    status.value = 'success'
  } catch (error) {
    if (error instanceof DOMException && error.name === 'AbortError') {
      status.value = 'idle'
      return
    }
    const message = error instanceof Error ? error.message : t('keys.test.unknownError')
    status.value = 'error'
    errorMessage.value = message
    addLine(`Error: ${message}`, 'text-red-400')
  } finally {
    if (abortController === controller) abortController = null
  }
}

function copyOutput() {
  const text = [...outputLines.value.map((line) => line.text), streamingContent.value]
    .filter(Boolean)
    .join('\n')
  void copyToClipboard(text, t('keys.test.outputCopied'))
}
</script>
