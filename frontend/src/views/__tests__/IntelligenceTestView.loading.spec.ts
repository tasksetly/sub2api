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

describe('Intelligence results loading', () => {
  it('loads on mount, permits manual refresh, and polls for updates', async () => {
    vi.useFakeTimers()
    vi.mocked(getIntelligenceTest).mockResolvedValue({ items: [], groups: [] })
    const wrapper = mount(IntelligenceTestView, {
      props: { inApp: true },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en } })], stubs: { Icon: true, RouterLink: true } },
    })
    try {
      await flushPromises()
      expect(getIntelligenceTest).toHaveBeenCalledTimes(2)
      expect(wrapper.find('.loader').exists()).toBe(false)

      await wrapper.get('.refresh-button').trigger('click')
      await flushPromises()
      expect(getIntelligenceTest).toHaveBeenCalledTimes(4)

      await vi.advanceTimersByTimeAsync(60_000)
      expect(getIntelligenceTest).toHaveBeenCalledTimes(6)

      expect(getIntelligenceTest).toHaveBeenCalledWith(expect.objectContaining({ answer_mode: 'candy' }))
      expect(getIntelligenceTest).toHaveBeenCalledWith(expect.objectContaining({ answer_mode: 'svg' }))
      expect(wrapper.find('[data-testid="candy-summary"]').exists()).toBe(true)
      expect(wrapper.find('.prompt-panel').exists()).toBe(true)
      expect(wrapper.find('button[aria-pressed]').exists()).toBe(false)
      expect(wrapper.html().indexOf('candy-summary')).toBeLessThan(wrapper.html().indexOf('prompt-panel'))
    } finally {
      wrapper.unmount()
      vi.useRealTimers()
      vi.clearAllMocks()
    }
  })

  it.each(['candy', 'svg'] as const)('keeps the other section visible when %s fails', async failedMode => {
    vi.mocked(getIntelligenceTest).mockImplementation(async query => {
      if (query?.answer_mode === failedMode) throw new Error(`${failedMode} unavailable`)
      return { items: [], groups: [] }
    })
    const wrapper = mount(IntelligenceTestView, {
      props: { inApp: true },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en } })], stubs: { Icon: true, RouterLink: true } },
    })
    try {
      await flushPromises()
      expect(wrapper.text()).toContain(`${failedMode} unavailable`)
      expect(wrapper.find('[data-testid="candy-summary"]').exists()).toBe(failedMode !== 'candy')
      expect(wrapper.find('.state-panel--error').exists()).toBe(failedMode === 'svg')
      expect(wrapper.find('.prompt-panel').exists()).toBe(true)
    } finally {
      wrapper.unmount()
      vi.clearAllMocks()
    }
  })

})
