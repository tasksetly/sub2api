/**
 * Playground API
 * 直连网关（/v1/*）用用户自己的 API Key 做对话测试，不走 /api/v1 管理接口。
 * 因此这里用原生 fetch 而不是 apiClient：需要读取 SSE 流，且鉴权头是用户的 API Key
 * 而不是登录 token。
 */

import { buildGatewayUrl } from './url'

/** 网关 /v1/models 的单条模型（Anthropic 与 OpenAI 两种形状都带 id）。 */
export interface PlaygroundModel {
  id: string
  display_name?: string
}

interface ModelsResponse {
  data?: Array<{ id?: unknown; display_name?: unknown }>
}

export type PlaygroundRole = 'system' | 'user' | 'assistant'

export interface PlaygroundMessage {
  role: PlaygroundRole
  content: string
}

/** 流式回调收到的 token 用量（上游给了才有）。 */
export interface PlaygroundUsage {
  prompt_tokens?: number
  completion_tokens?: number
  total_tokens?: number
}

/** 从网关错误响应里抽出人类可读信息，兼容 OpenAI / Anthropic 两种错误形状。 */
async function extractErrorMessage(response: Response): Promise<string> {
  let raw = ''
  try {
    raw = await response.text()
  } catch {
    return `HTTP ${response.status}`
  }
  if (!raw) return `HTTP ${response.status}`
  try {
    const parsed = JSON.parse(raw)
    const message =
      parsed?.error?.message ?? parsed?.error?.type ?? parsed?.message ?? parsed?.error
    if (typeof message === 'string' && message.trim()) {
      return message.trim()
    }
  } catch {
    // 非 JSON（例如网关前面的反代返回 HTML），退回截断的原文
  }
  return raw.slice(0, 300).trim() || `HTTP ${response.status}`
}

/**
 * 拉取该 Key 所属分组可用的模型列表。
 * 网关按分组平台返回不同形状，但两者都有 data[].id。
 */
export async function listPlaygroundModels(
  apiKey: string,
  options?: { signal?: AbortSignal }
): Promise<PlaygroundModel[]> {
  const response = await fetch(buildGatewayUrl('/v1/models'), {
    method: 'GET',
    headers: { Authorization: `Bearer ${apiKey}` },
    signal: options?.signal,
  })
  if (!response.ok) {
    throw new Error(await extractErrorMessage(response))
  }
  const payload = (await response.json()) as ModelsResponse
  const seen = new Set<string>()
  const models: PlaygroundModel[] = []
  for (const item of payload?.data ?? []) {
    const id = typeof item?.id === 'string' ? item.id.trim() : ''
    if (!id || seen.has(id)) continue
    seen.add(id)
    models.push({
      id,
      display_name: typeof item?.display_name === 'string' ? item.display_name : undefined,
    })
  }
  return models
}

export interface StreamChatOptions {
  apiKey: string
  model: string
  messages: PlaygroundMessage[]
  temperature?: number
  maxTokens?: number
  signal?: AbortSignal
  /** 每收到一段增量文本触发。 */
  onDelta: (text: string) => void
  /** 上游返回用量时触发（可能不触发）。 */
  onUsage?: (usage: PlaygroundUsage) => void
}

/** 解析一条 SSE data 负载，返回增量文本；同时把用量透传给回调。 */
function consumeChunk(payload: string, options: StreamChatOptions): void {
  let parsed: Record<string, unknown>
  try {
    parsed = JSON.parse(payload)
  } catch {
    return // 忽略心跳/非 JSON 行
  }

  const choices = (parsed as { choices?: Array<Record<string, unknown>> }).choices
  const delta = choices?.[0]?.delta as { content?: unknown } | undefined
  if (typeof delta?.content === 'string' && delta.content) {
    options.onDelta(delta.content)
  }

  // 少数上游在非流式形状里塞完整 message（例如 bridge 回退）
  const message = choices?.[0]?.message as { content?: unknown } | undefined
  if (typeof message?.content === 'string' && message.content) {
    options.onDelta(message.content)
  }

  const usage = (parsed as { usage?: PlaygroundUsage | null }).usage
  if (usage && options.onUsage) {
    options.onUsage(usage)
  }
}

/**
 * 以流式方式请求 /v1/chat/completions。
 * 网关会按 Key 所属分组的平台自动转换（Anthropic 分组也能走这个端点）。
 * 调用方通过 AbortSignal 停止生成。
 */
export async function streamPlaygroundChat(options: StreamChatOptions): Promise<void> {
  const body: Record<string, unknown> = {
    model: options.model,
    messages: options.messages,
    stream: true,
  }
  if (typeof options.temperature === 'number') {
    body.temperature = options.temperature
  }
  if (typeof options.maxTokens === 'number' && options.maxTokens > 0) {
    body.max_tokens = options.maxTokens
  }

  const response = await fetch(buildGatewayUrl('/v1/chat/completions'), {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${options.apiKey}`,
    },
    body: JSON.stringify(body),
    signal: options.signal,
  })

  if (!response.ok) {
    throw new Error(await extractErrorMessage(response))
  }
  if (!response.body) {
    throw new Error('Streaming is not supported in this browser')
  }

  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  try {
    for (;;) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      // SSE 事件以空行分隔；按行扫描，只取 data: 字段
      let newlineIndex = buffer.indexOf('\n')
      while (newlineIndex !== -1) {
        const line = buffer.slice(0, newlineIndex).replace(/\r$/, '')
        buffer = buffer.slice(newlineIndex + 1)
        newlineIndex = buffer.indexOf('\n')

        if (!line.startsWith('data:')) continue
        const payload = line.slice(5).trim()
        if (!payload || payload === '[DONE]') continue
        consumeChunk(payload, options)
      }
    }

    // 收尾：流结束时缓冲区可能还留着最后一行（无结尾换行）
    const tail = buffer.replace(/\r$/, '')
    if (tail.startsWith('data:')) {
      const payload = tail.slice(5).trim()
      if (payload && payload !== '[DONE]') {
        consumeChunk(payload, options)
      }
    }
  } finally {
    reader.releaseLock()
  }
}

export const playgroundAPI = {
  listModels: listPlaygroundModels,
  streamChat: streamPlaygroundChat,
}

export default playgroundAPI
