import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { listPlaygroundModels, streamPlaygroundChat } from '@/api/playground'

/**
 * 构造一个假的 fetch Response：body.getReader() 按 chunks 逐段吐出。
 * 直接造 reader 而不用 ReadableStream，避免依赖 jsdom 的流实现。
 */
function streamResponse(chunks: string[]): Response {
  const encoder = new TextEncoder()
  let index = 0
  return {
    ok: true,
    status: 200,
    body: {
      getReader: () => ({
        read: async () =>
          index < chunks.length
            ? { done: false, value: encoder.encode(chunks[index++]) }
            : { done: true, value: undefined },
        releaseLock: () => {},
      }),
    },
  } as unknown as Response
}

function jsonResponse(payload: unknown): Response {
  return {
    ok: true,
    status: 200,
    json: async () => payload,
  } as unknown as Response
}

const fetchMock = vi.fn()

beforeEach(() => {
  fetchMock.mockReset()
  vi.stubGlobal('fetch', fetchMock)
})

afterEach(() => {
  vi.unstubAllGlobals()
})

describe('listPlaygroundModels', () => {
  it('accepts both gateway shapes and drops duplicates/blanks', async () => {
    fetchMock.mockResolvedValue(
      jsonResponse({
        object: 'list',
        data: [
          { id: 'claude-sonnet-4', display_name: 'Claude Sonnet 4' },
          { id: 'claude-sonnet-4' },
          { id: '  ' },
          { id: 'gpt-5' },
          { notAnId: true },
        ],
      })
    )

    const models = await listPlaygroundModels('sk-test')

    expect(models.map((m) => m.id)).toEqual(['claude-sonnet-4', 'gpt-5'])
    expect(models[0].display_name).toBe('Claude Sonnet 4')
  })

  it('sends the API key as a bearer token, not the login token', async () => {
    fetchMock.mockResolvedValue(jsonResponse({ data: [] }))

    await listPlaygroundModels('sk-my-key')

    const [, init] = fetchMock.mock.calls[0]
    expect(init.headers.Authorization).toBe('Bearer sk-my-key')
  })

  it('surfaces the gateway error message', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 401,
      text: async () => JSON.stringify({ error: { message: 'Invalid API key' } }),
    } as unknown as Response)

    await expect(listPlaygroundModels('bad')).rejects.toThrow('Invalid API key')
  })
})

describe('streamPlaygroundChat', () => {
  const baseOptions = {
    apiKey: 'sk-test',
    model: 'claude-sonnet-4',
    messages: [{ role: 'user' as const, content: 'hi' }],
  }

  it('assembles deltas across SSE events', async () => {
    fetchMock.mockResolvedValue(
      streamResponse([
        'data: {"choices":[{"delta":{"content":"Hel"}}]}\n\n',
        'data: {"choices":[{"delta":{"content":"lo"}}]}\n\n',
        'data: [DONE]\n\n',
      ])
    )

    const deltas: string[] = []
    await streamPlaygroundChat({ ...baseOptions, onDelta: (text) => deltas.push(text) })

    expect(deltas.join('')).toBe('Hello')
  })

  it('handles an SSE event split across two network reads', async () => {
    // 关键回归点：分片可能切在一行中间，解析必须靠缓冲区而不是单次 chunk
    fetchMock.mockResolvedValue(
      streamResponse(['data: {"choices":[{"delta":{"co', 'ntent":"split"}}]}\n\n'])
    )

    const deltas: string[] = []
    await streamPlaygroundChat({ ...baseOptions, onDelta: (text) => deltas.push(text) })

    expect(deltas.join('')).toBe('split')
  })

  it('reads a trailing event that has no final newline', async () => {
    fetchMock.mockResolvedValue(
      streamResponse(['data: {"choices":[{"delta":{"content":"tail"}}]}'])
    )

    const deltas: string[] = []
    await streamPlaygroundChat({ ...baseOptions, onDelta: (text) => deltas.push(text) })

    expect(deltas.join('')).toBe('tail')
  })

  it('ignores heartbeats and non-data lines instead of throwing', async () => {
    fetchMock.mockResolvedValue(
      streamResponse([
        ': ping\n\n',
        'event: message\n',
        'data: not-json\n\n',
        'data: {"choices":[{"delta":{"content":"ok"}}]}\n\n',
      ])
    )

    const deltas: string[] = []
    await streamPlaygroundChat({ ...baseOptions, onDelta: (text) => deltas.push(text) })

    expect(deltas.join('')).toBe('ok')
  })

  it('reports usage when the upstream includes it', async () => {
    fetchMock.mockResolvedValue(
      streamResponse([
        'data: {"choices":[{"delta":{"content":"x"}}],"usage":{"prompt_tokens":3,"completion_tokens":1,"total_tokens":4}}\n\n',
      ])
    )

    const onUsage = vi.fn()
    await streamPlaygroundChat({ ...baseOptions, onDelta: () => {}, onUsage })

    expect(onUsage).toHaveBeenCalledWith({
      prompt_tokens: 3,
      completion_tokens: 1,
      total_tokens: 4,
    })
  })

  it('requests streaming and omits max_tokens when not positive', async () => {
    fetchMock.mockResolvedValue(streamResponse(['data: [DONE]\n\n']))

    await streamPlaygroundChat({ ...baseOptions, maxTokens: 0, onDelta: () => {} })

    const body = JSON.parse(fetchMock.mock.calls[0][1].body)
    expect(body.stream).toBe(true)
    expect(body.model).toBe('claude-sonnet-4')
    expect(body).not.toHaveProperty('max_tokens')
  })

  it('surfaces gateway errors before reading the stream', async () => {
    fetchMock.mockResolvedValue({
      ok: false,
      status: 429,
      text: async () => JSON.stringify({ error: { message: 'rate limited' } }),
    } as unknown as Response)

    await expect(
      streamPlaygroundChat({ ...baseOptions, onDelta: () => {} })
    ).rejects.toThrow('rate limited')
  })
})
