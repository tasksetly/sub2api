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
      expect(wrapper.get('#test-results-candy').isVisible()).toBe(true)
      expect(wrapper.get('#test-results-svg').isVisible()).toBe(false)
      await wrapper.get('[data-test="test-mode-svg"]').trigger('click')
      expect(wrapper.get('#test-results-svg').isVisible()).toBe(true)
      expect(wrapper.get('#test-results-candy').isVisible()).toBe(false)
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
      await wrapper.get(`[data-test="test-mode-${failedMode}"]`).trigger('click')
      expect(wrapper.text()).toContain(`${failedMode} unavailable`)
      expect(wrapper.find('[data-testid="candy-summary"]').exists()).toBe(failedMode !== 'candy')
      expect(wrapper.find('.state-panel--error').exists()).toBe(failedMode === 'svg')
      const otherMode = failedMode === 'svg' ? 'candy' : 'svg'
      await wrapper.get(`[data-test="test-mode-${otherMode}"]`).trigger('click')
      expect(wrapper.get(`#test-results-${otherMode}`).isVisible()).toBe(true)
    } finally {
      wrapper.unmount()
      vi.clearAllMocks()
    }
  })

  it('reveals only the prompts belonging to the selected SVG group button', async () => {
    vi.mocked(getIntelligenceTest).mockImplementation(async query => query?.answer_mode === 'svg' ? {
      config: { prompt_text: 'Global prompt' },
      items: [
        { id: 1, group_id: 10, group_name: 'Shared', svg_content: '<svg/>', prompt_text: 'Saved prompt A' },
        { id: 2, group_id: 20, group_name: 'Shared', svg_content: '<svg/>', prompt_text: 'Current prompt B', prompt_is_current: true },
      ],
    } : { items: [] })
    const wrapper = mount(IntelligenceTestView, {
      props: { inApp: true },
      global: { plugins: [createI18n({ legacy: false, locale: 'en', messages: { en } })], stubs: { Icon: true, RouterLink: true } },
    })
    try {
      await flushPromises()
      await wrapper.get('[data-test="test-mode-svg"]').trigger('click')
      const [first, second] = wrapper.findAll('.group-section')
      const firstButton = first.get('button[aria-controls]')
      const secondButton = second.get('button[aria-controls]')
      const firstPanel = first.get(`[id="${firstButton.attributes('aria-controls')}"]`)
      const secondPanel = second.get(`[id="${secondButton.attributes('aria-controls')}"]`)
      expect(firstPanel.isVisible()).toBe(false)
      await firstButton.trigger('pointerenter', { pointerType: 'mouse' })
      expect(firstPanel.isVisible()).toBe(true)
      expect(firstPanel.text()).toContain('Saved prompt A')
      expect(firstPanel.text()).not.toContain('Current prompt B')
      expect(secondPanel.isVisible()).toBe(false)
      await secondButton.trigger('click')
      expect(secondPanel.isVisible()).toBe(true)
      expect(secondPanel.text()).toContain('The original prompt was not recorded.')
      expect(secondPanel.text()).toContain('Current prompt B')
      expect(wrapper.text()).not.toContain('Global prompt')
    } finally {
      wrapper.unmount()
      vi.clearAllMocks()
    }
  })
})
