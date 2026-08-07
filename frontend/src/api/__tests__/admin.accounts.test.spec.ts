import { beforeEach, describe, expect, it, vi } from 'vitest'

const { fetchMock } = vi.hoisted(() => ({
  fetchMock: vi.fn()
}))

vi.mock('@/api/client', () => ({
  apiClient: {},
  buildApiUrl: (path: string) => path
}))

import { testAccount } from '@/api/admin/accounts'

function streamReader(chunks: string[]) {
  let index = 0
  return {
    read: vi.fn(async () => {
      if (index >= chunks.length) return { done: true, value: undefined }
      const value = new TextEncoder().encode(chunks[index])
      index += 1
      return { done: false, value }
    })
  }
}

describe('admin account test API', () => {
  beforeEach(() => {
    fetchMock.mockReset()
    vi.stubGlobal('fetch', fetchMock)
    localStorage.setItem('auth_token', 'test-token')
  })

  it('parses a successful SSE result across network chunks', async () => {
    const reader = streamReader([
      'data: {"type":"test_start"}\n\ndata: {"type":"test_complete","success":true',
      ',"error":""}\n\n'
    ])
    fetchMock.mockResolvedValue({ ok: true, body: { getReader: () => reader } })

    await expect(testAccount(42)).resolves.toMatchObject({
      success: true,
      message: 'Test completed successfully'
    })
    expect(fetchMock).toHaveBeenCalledWith('/admin/accounts/42/test', expect.objectContaining({
      method: 'POST',
      body: '{}',
      credentials: 'include',
      headers: expect.objectContaining({
        Authorization: 'Bearer test-token',
        'X-Admin-UI-Request': '1'
      })
    }))
  })

  it('sends the selected model ID for a batch test', async () => {
    const reader = streamReader(['data: {"type":"test_complete","success":true}\n\n'])
    fetchMock.mockResolvedValue({ ok: true, body: { getReader: () => reader } })

    await expect(testAccount(42, { modelId: 'gpt-5.4' })).resolves.toMatchObject({ success: true })

    expect(fetchMock).toHaveBeenCalledWith('/admin/accounts/42/test', expect.objectContaining({
      method: 'POST',
      body: '{"model_id":"gpt-5.4"}'
    }))
  })

  it('keeps a failed test selected by returning a false result for error events', async () => {
    const reader = streamReader(['data: {"type":"error","error":"invalid token"}\n\n'])
    fetchMock.mockResolvedValue({ ok: true, body: { getReader: () => reader } })

    await expect(testAccount(43)).resolves.toMatchObject({
      success: false,
      message: 'invalid token'
    })
  })
})
