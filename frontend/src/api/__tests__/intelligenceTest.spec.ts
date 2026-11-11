import { afterEach, describe, expect, it, vi } from 'vitest'
import { getIntelligenceTest } from '../intelligenceTest'
import { apiClient } from '../client'

vi.mock('../client', () => ({ apiClient: { get: vi.fn() } }))
afterEach(() => { vi.clearAllMocks(); vi.unstubAllGlobals() })

describe('candy history requests', () => {
  it('passes the selected test type to the history endpoint', async () => {
    vi.mocked(apiClient.get).mockResolvedValue({ data: { items: [] } })
    await getIntelligenceTest({ answer_mode: 'candy' })
    expect(apiClient.get).toHaveBeenCalledWith('/intelligence-test/history', expect.objectContaining({ params: { answer_mode: 'candy' } }))
  })

  it('does not replace an unavailable candy history with legacy SVG data', async () => {
    vi.mocked(apiClient.get).mockRejectedValue(new Error('unavailable'))
    const fetchMock = vi.fn()
    vi.stubGlobal('fetch', fetchMock)
    await expect(getIntelligenceTest({ answer_mode: 'candy' })).rejects.toThrow('unavailable')
    expect(apiClient.get).toHaveBeenCalledTimes(1)
    expect(fetchMock).not.toHaveBeenCalled()
  })
})
