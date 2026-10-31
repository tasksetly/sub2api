import { describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { getIntelligenceTest } from '@/api/intelligenceTest'
import IntelligenceTestView from '../IntelligenceTestView.vue'

vi.mock('@/api/intelligenceTest', () => ({ getIntelligenceTest: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ siteName: 'Test', siteLogo: '', fetchPublicSettings: vi.fn() }),
}))

describe('SVG results loading', () => {
  it('loads on mount, permits manual refresh, and polls for updates', async () => {
    vi.useFakeTimers()
    vi.mocked(getIntelligenceTest).mockResolvedValue({ items: [], groups: [] })
    const wrapper = mount(IntelligenceTestView, {
      props: { inApp: true },
      global: { stubs: { Icon: true, RouterLink: true } },
    })
    try {
      await flushPromises()
      expect(getIntelligenceTest).toHaveBeenCalledTimes(1)
      expect(wrapper.find('.loader').exists()).toBe(false)

      await wrapper.get('.refresh-button').trigger('click')
      await flushPromises()
      expect(getIntelligenceTest).toHaveBeenCalledTimes(2)

      await vi.advanceTimersByTimeAsync(60_000)
      expect(getIntelligenceTest).toHaveBeenCalledTimes(3)
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
      vi.clearAllMocks()
    }
  })
})
