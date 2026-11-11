import { describe, expect, it, vi } from 'vitest'
import { createI18n } from 'vue-i18n'
import en from '@/i18n/locales/en/intelligenceTest'
import { flushPromises, mount } from '@vue/test-utils'
import { getIntelligenceTest } from '@/api/intelligenceTest'
import IntelligenceTestView from '../IntelligenceTestView.vue'

vi.mock('@/api/intelligenceTest', () => ({ getIntelligenceTest: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }), useRouter: () => ({ push: vi.fn() }) }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ siteName: 'Test', siteLogo: '', fetchPublicSettings: vi.fn() }),
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: false }),
}))

describe('SVG results loading', () => {
  it('loads on mount, permits manual refresh, and polls for updates', async () => {
    vi.useFakeTimers()
    vi.mocked(getIntelligenceTest).mockResolvedValue({ items: [], groups: [] })
    const wrapper = mount(IntelligenceTestView, {
      props: { inApp: true },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en } })], stubs: { Icon: true, RouterLink: true } },
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

      await wrapper.get('button[aria-pressed="false"]').trigger('click')
      await flushPromises()
      expect(getIntelligenceTest).toHaveBeenLastCalledWith(expect.objectContaining({ answer_mode: 'candy' }))
      expect(wrapper.find('[data-testid="candy-summary"]').exists()).toBe(true)
      expect(wrapper.find('.prompt-panel').exists()).toBe(false)

      await wrapper.get('button[aria-pressed="false"]').trigger('click')
      await flushPromises()
      expect(getIntelligenceTest).toHaveBeenLastCalledWith(expect.objectContaining({ answer_mode: 'svg' }))
      expect(wrapper.find('.prompt-panel').exists()).toBe(true)
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
      vi.clearAllMocks()
    }
  })
})
