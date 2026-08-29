<template>
  <AppLayout>
    <div class="flex flex-col gap-4 lg:h-[calc(100vh-9rem)] lg:flex-row">
      <!-- ==================== 左侧：参数面板 ==================== -->
      <aside
        class="flex shrink-0 flex-col gap-4 overflow-y-auto rounded-xl border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800 lg:w-80"
      >
        <div>
          <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('playground.apiKey') }}
          </label>
          <Select
            v-model="selectedKeyId"
            :options="keyOptions"
            :placeholder="t('playground.selectKey')"
            :disabled="loadingKeys || streaming"
            searchable
          />
          <p
            v-if="!loadingKeys && keyOptions.length === 0"
            class="mt-1.5 text-xs text-amber-600 dark:text-amber-400"
          >
            {{ t('playground.noKeys') }}
          </p>
        </div>
        <div>
          <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('playground.model') }}
          </label>
          <Select
            v-model="selectedModel"
            :options="modelOptions"
            :placeholder="loadingModels ? t('playground.loadingModels') : t('playground.selectModel')"
            :disabled="!selectedKeyId || loadingModels || streaming"
            :empty-text="t('playground.noModels')"
            searchable
            creatable
          />
          <p v-if="modelsError" class="mt-1.5 text-xs text-red-600 dark:text-red-400">
            {{ modelsError }}
          </p>
          <p v-else class="mt-1.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('playground.modelHint') }}
          </p>
        </div>

        <div>
          <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('playground.systemPrompt') }}
          </label>
          <textarea
            v-model="systemPrompt"
            rows="4"
            :disabled="streaming"
            :placeholder="t('playground.systemPromptPlaceholder')"
            class="input resize-y"
          />
        </div>

        <div>
          <label class="mb-1.5 flex items-center justify-between text-sm font-medium text-gray-700 dark:text-gray-300">
            <span>{{ t('playground.temperature') }}</span>
            <span class="font-mono text-xs text-gray-500 dark:text-gray-400">{{ temperature.toFixed(2) }}</span>
          </label>
          <input
            v-model.number="temperature"
            type="range"
            min="0"
            max="2"
            step="0.05"
            :disabled="streaming"
            class="w-full accent-blue-600"
          />
        </div>

        <div>
          <label class="mb-1.5 block text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('playground.maxTokens') }}
          </label>
          <input
            v-model.number="maxTokens"
            type="number"
            min="1"
            max="200000"
            step="256"
            :disabled="streaming"
            class="input"
          />
        </div>

        <div class="mt-auto space-y-2 border-t border-gray-200 pt-4 dark:border-dark-700">
          <div v-if="lastUsage" class="rounded-lg bg-gray-50 p-3 text-xs dark:bg-dark-700/50">
            <div class="mb-1 font-medium text-gray-700 dark:text-gray-300">
              {{ t('playground.usage') }}
            </div>
            <div class="flex justify-between text-gray-500 dark:text-gray-400">
              <span>{{ t('playground.promptTokens') }}</span>
              <span class="font-mono">{{ lastUsage.prompt_tokens ?? '-' }}</span>
            </div>
            <div class="flex justify-between text-gray-500 dark:text-gray-400">
              <span>{{ t('playground.completionTokens') }}</span>
              <span class="font-mono">{{ lastUsage.completion_tokens ?? '-' }}</span>
            </div>
            <div class="flex justify-between text-gray-500 dark:text-gray-400">
              <span>{{ t('playground.totalTokens') }}</span>
              <span class="font-mono">{{ lastUsage.total_tokens ?? '-' }}</span>
            </div>
          </div>
          <button
            type="button"
            class="btn btn-secondary w-full"
            :disabled="streaming || messages.length === 0"
            @click="clearConversation"
          >
            <Icon name="trash" size="md" class="mr-2" />
            {{ t('playground.clear') }}
          </button>
        </div>
      </aside>

      <!-- ==================== 右侧：对话区 ==================== -->
      <section
        class="flex min-h-[28rem] flex-1 flex-col overflow-hidden rounded-xl border border-gray-200 bg-white dark:border-dark-700 dark:bg-dark-800"
      >
        <div ref="scrollRef" class="flex-1 space-y-5 overflow-y-auto p-4 sm:p-6">
          <div
            v-if="messages.length === 0"
            class="flex h-full flex-col items-center justify-center text-center text-gray-400 dark:text-gray-500"
          >
            <Icon name="chat" size="xl" class="mb-3 opacity-60" />
            <p class="text-sm">{{ t('playground.emptyHint') }}</p>
          </div>

          <div
            v-for="(message, index) in messages"
            :key="index"
            class="flex gap-3"
            :class="message.role === 'user' ? 'flex-row-reverse' : 'flex-row'"
          >
            <div
              class="flex h-8 w-8 shrink-0 items-center justify-center rounded-full"
              :class="message.role === 'user'
                ? 'bg-blue-600 text-white'
                : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'"
            >
              <Icon :name="message.role === 'user' ? 'user' : 'sparkles'" size="sm" />
            </div>
            <div class="min-w-0 max-w-[85%]">
              <div
                class="rounded-2xl px-4 py-2.5 text-sm"
                :class="message.role === 'user'
                  ? 'bg-blue-600 text-white'
                  : 'bg-gray-50 text-gray-800 dark:bg-dark-700/60 dark:text-gray-200'"
              >
                <!-- 用户消息保留纯文本；助手消息渲染 Markdown -->
                <p v-if="message.role === 'user'" class="whitespace-pre-wrap break-words">{{ message.content }}</p>
                <div
                  v-else-if="message.content"
                  class="markdown-body playground-markdown"
                  v-html="renderMarkdown(message.content)"
                />
                <span
                  v-else-if="streaming && index === messages.length - 1"
                  class="inline-flex gap-1 py-1"
                >
                  <span class="h-1.5 w-1.5 animate-bounce rounded-full bg-gray-400" />
                  <span class="h-1.5 w-1.5 animate-bounce rounded-full bg-gray-400 [animation-delay:150ms]" />
                  <span class="h-1.5 w-1.5 animate-bounce rounded-full bg-gray-400 [animation-delay:300ms]" />
                </span>
              </div>
              <div
                v-if="message.role === 'assistant' && message.content && !(streaming && index === messages.length - 1)"
                class="mt-1 flex gap-1"
              >
                <button
                  type="button"
                  class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                  :title="t('playground.copy')"
                  @click="copyMessage(message.content)"
                >
                  <Icon name="copy" size="sm" />
                </button>
                <button
                  v-if="index === messages.length - 1"
                  type="button"
                  class="rounded p-1 text-gray-400 transition-colors hover:bg-gray-100 hover:text-gray-600 dark:hover:bg-dark-700 dark:hover:text-gray-300"
                  :title="t('playground.regenerate')"
                  @click="regenerate"
                >
                  <Icon name="refresh" size="sm" />
                </button>
              </div>
            </div>
          </div>

          <div
            v-if="errorMessage"
            class="flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-900/20 dark:text-red-300"
          >
            <Icon name="exclamationCircle" size="md" class="mt-0.5 shrink-0" />
            <span class="break-words">{{ errorMessage }}</span>
          </div>
        </div>

        <!-- 输入区 -->
        <div class="border-t border-gray-200 p-3 dark:border-dark-700 sm:p-4">
          <div class="flex items-end gap-2">
            <textarea
              ref="inputRef"
              v-model="input"
              rows="1"
              :placeholder="t('playground.inputPlaceholder')"
              class="input max-h-40 flex-1 resize-none"
              @keydown="onInputKeydown"
              @input="autoResize"
            />
            <button
              v-if="streaming"
              type="button"
              class="btn btn-secondary shrink-0"
              @click="stopStreaming"
            >
              <Icon name="x" size="md" class="sm:mr-2" />
              <span class="hidden sm:inline">{{ t('playground.stop') }}</span>
            </button>
            <button
              v-else
              type="button"
              class="btn btn-primary shrink-0"
              :disabled="!canSend"
              @click="send"
            >
              <Icon name="arrowUp" size="md" class="sm:mr-2" />
              <span class="hidden sm:inline">{{ t('playground.send') }}</span>
            </button>
          </div>
          <p class="mt-2 text-xs text-gray-400 dark:text-gray-500">
            {{ t('playground.sendHint') }}
          </p>
        </div>
      </section>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { marked } from 'marked'
import DOMPurify from 'dompurify'
import AppLayout from '@/components/layout/AppLayout.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { keysAPI } from '@/api/keys'
import { listPlaygroundModels, streamPlaygroundChat, type PlaygroundUsage } from '@/api/playground'
import { useClipboard } from '@/composables/useClipboard'
import type { ApiKey } from '@/types'
import '@/styles/announcement-markdown.css'

interface ChatMessage {
  role: 'user' | 'assistant'
  content: string
}

const { t } = useI18n()
const { copyToClipboard } = useClipboard()

const keys = ref<ApiKey[]>([])
const loadingKeys = ref(false)
const selectedKeyId = ref<number | null>(null)

const models = ref<string[]>([])
const loadingModels = ref(false)
const modelsError = ref('')
const selectedModel = ref<string | null>(null)

const systemPrompt = ref('')
const temperature = ref(1)
const maxTokens = ref(2048)

const messages = ref<ChatMessage[]>([])
const input = ref('')
const streaming = ref(false)
const errorMessage = ref('')
const lastUsage = ref<PlaygroundUsage | null>(null)

const scrollRef = ref<HTMLElement | null>(null)
const inputRef = ref<HTMLTextAreaElement | null>(null)
let abortController: AbortController | null = null
let modelsRequestToken = 0

const selectedKey = computed(() => keys.value.find((k) => k.id === selectedKeyId.value) ?? null)

const keyOptions = computed<SelectOption[]>(() =>
  keys.value.map((key) => ({
    value: key.id,
    // 分组名帮用户区分「这条 Key 能打哪些模型」
    label: key.group?.name ? `${key.name} · ${key.group.name}` : key.name,
  }))
)

const modelOptions = computed<SelectOption[]>(() =>
  models.value.map((model) => ({ value: model, label: model }))
)

const canSend = computed(
  () => !!selectedKey.value?.key && !!selectedModel.value && !!input.value.trim() && !streaming.value
)

marked.setOptions({ breaks: true, gfm: true })

function renderMarkdown(content: string): string {
  return DOMPurify.sanitize(marked.parse(content) as string)
}

/** 只列出可用（active）的 Key：其它状态发请求必然被网关拒。 */
async function loadKeys(): Promise<void> {
  loadingKeys.value = true
  try {
    const response = await keysAPI.list(1, 200, { status: 'active' })
    keys.value = response.items ?? []
    if (!selectedKeyId.value && keys.value.length > 0) {
      selectedKeyId.value = keys.value[0].id
    }
  } catch {
    errorMessage.value = t('playground.loadKeysFailed')
  } finally {
    loadingKeys.value = false
  }
}

/** 用选中的 Key 问网关要模型列表（结果取决于该 Key 所属分组）。 */
async function loadModels(): Promise<void> {
  const apiKey = selectedKey.value?.key
  models.value = []
  selectedModel.value = null
  modelsError.value = ''
  if (!apiKey) return

  const token = ++modelsRequestToken
  loadingModels.value = true
  try {
    const list = await listPlaygroundModels(apiKey)
    if (token !== modelsRequestToken) return // 期间又换了 Key，丢弃这次结果
    models.value = list.map((model) => model.id)
    if (models.value.length > 0) {
      selectedModel.value = models.value[0]
    }
  } catch (error) {
    if (token !== modelsRequestToken) return
    modelsError.value = error instanceof Error ? error.message : String(error)
  } finally {
    if (token === modelsRequestToken) {
      loadingModels.value = false
    }
  }
}

watch(selectedKeyId, () => {
  void loadModels()
})
async function scrollToBottom(): Promise<void> {
  await nextTick()
  const el = scrollRef.value
  if (el) {
    el.scrollTop = el.scrollHeight
  }
}

function autoResize(): void {
  const el = inputRef.value
  if (!el) return
  el.style.height = 'auto'
  el.style.height = `${Math.min(el.scrollHeight, 160)}px`
}

/** Enter 发送，Shift+Enter 换行；输入法组合中（isComposing）不拦截。 */
function onInputKeydown(event: KeyboardEvent): void {
  if (event.key !== 'Enter' || event.shiftKey || event.isComposing) return
  event.preventDefault()
  if (canSend.value) {
    void send()
  }
}

/** 把当前设置 + 历史组装成请求，流式写入最后一条助手消息。 */
async function runCompletion(): Promise<void> {
  const apiKey = selectedKey.value?.key
  const model = selectedModel.value
  if (!apiKey || !model) return

  errorMessage.value = ''
  lastUsage.value = null
  streaming.value = true
  abortController = new AbortController()

  const history = messages.value
    .filter((message) => message.content.trim() !== '')
    .map((message) => ({ role: message.role, content: message.content }))
  const payload = systemPrompt.value.trim()
    ? [{ role: 'system' as const, content: systemPrompt.value.trim() }, ...history]
    : history

  // 占位的空助手消息：流式增量直接往它身上追加
  messages.value.push({ role: 'assistant', content: '' })
  const targetIndex = messages.value.length - 1
  await scrollToBottom()

  try {
    await streamPlaygroundChat({
      apiKey,
      model,
      messages: payload,
      temperature: temperature.value,
      maxTokens: maxTokens.value,
      signal: abortController.signal,
      onDelta: (text) => {
        const target = messages.value[targetIndex]
        if (target) {
          target.content += text
          void scrollToBottom()
        }
      },
      onUsage: (usage) => {
        lastUsage.value = usage
      },
    })
  } catch (error) {
    // 用户点「停止」时 fetch 抛 AbortError，不算错误
    const aborted = error instanceof DOMException && error.name === 'AbortError'
    if (!aborted) {
      errorMessage.value = error instanceof Error ? error.message : String(error)
    }
  } finally {
    streaming.value = false
    abortController = null
    // 一个字都没收到就把空占位删掉，避免留下空气泡
    if (messages.value[targetIndex]?.content === '') {
      messages.value.splice(targetIndex, 1)
    }
    await scrollToBottom()
  }
}

async function send(): Promise<void> {
  const text = input.value.trim()
  if (!text || streaming.value) return
  messages.value.push({ role: 'user', content: text })
  input.value = ''
  await nextTick()
  autoResize()
  await runCompletion()
}

function stopStreaming(): void {
  abortController?.abort()
}

/** 重新生成：丢掉最后一条助手回复，用同样的上下文再请求一次。 */
async function regenerate(): Promise<void> {
  if (streaming.value) return
  if (messages.value[messages.value.length - 1]?.role === 'assistant') {
    messages.value.pop()
  }
  if (messages.value.length === 0) return
  await runCompletion()
}

function clearConversation(): void {
  messages.value = []
  errorMessage.value = ''
  lastUsage.value = null
}

async function copyMessage(content: string): Promise<void> {
  await copyToClipboard(content, t('playground.copied'))
}

onMounted(() => {
  void loadKeys()
})

onBeforeUnmount(() => {
  abortController?.abort()
})
</script>

<style scoped>
/* 聊天气泡里的 Markdown 需要更紧凑的间距（announcement-markdown.css 是文档式排版）。 */
.playground-markdown :deep(p) {
  @apply mb-2;
}

.playground-markdown :deep(p:last-child) {
  @apply mb-0;
}

.playground-markdown :deep(pre) {
  @apply overflow-x-auto;
}

.playground-markdown :deep(h1),
.playground-markdown :deep(h2),
.playground-markdown :deep(h3),
.playground-markdown :deep(h4) {
  @apply mb-2 mt-3 border-none pb-0 text-base;
}

.playground-markdown :deep(ul),
.playground-markdown :deep(ol) {
  @apply mb-2;
}
</style>
