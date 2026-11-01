import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import { getModelPlaza, type ModelPlazaResponse } from '@/api/modelPlaza'
import ModelPlazaContent from '@/components/modelPlaza/ModelPlazaContent.vue'
import ModelPlazaView from '../ModelPlazaView.vue'

vi.mock('@/api/modelPlaza', () => ({ getModelPlaza: vi.fn() }))
vi.mock('vue-router', () => ({ useRoute: () => ({ query: {} }) }))
vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ fetchPublicSettings: vi.fn() })
}))
vi.mock('@/stores/auth', () => ({
  useAuthStore: () => ({ isAuthenticated: false })
}))

function mountView() {
  const i18n = createI18n({ legacy: false, locale: 'en', messages: {} })
  const wrapper = mount(ModelPlazaView, {
    global: {
      plugins: [i18n],
      stubs: { AppLayout: true, PlazaNavBar: true, ModelPlazaContent: true }
    }
  })
  return { wrapper, i18n }
}

beforeEach(() => vi.mocked(getModelPlaza).mockReset())

describe('ModelPlazaView request locale', () => {
  it('refreshes translated group data when the UI language changes', async () => {
    const english: ModelPlazaResponse = { description: '', groups: [] }
    const chinese: ModelPlazaResponse = { description: '目录', groups: [] }
    vi.mocked(getModelPlaza).mockResolvedValueOnce(english).mockResolvedValueOnce(chinese)
    const { wrapper, i18n } = mountView()
    await flushPromises()
    expect(wrapper.getComponent(ModelPlazaContent).props('response')).toEqual(english)

    i18n.global.locale.value = 'zh'
    await flushPromises()
    expect(getModelPlaza).toHaveBeenCalledTimes(2)
    expect(wrapper.getComponent(ModelPlazaContent).props('response')).toEqual(chinese)
    wrapper.unmount()
  })

  it('ignores an older response after a locale switch and aborts on unmount', async () => {
    let resolveOld!: (response: ModelPlazaResponse) => void
    vi.mocked(getModelPlaza).mockImplementationOnce(() => new Promise((resolve) => {
      resolveOld = resolve
    }))
    const latest: ModelPlazaResponse = { description: '最新目录', groups: [] }
    vi.mocked(getModelPlaza).mockResolvedValueOnce(latest)
    const { wrapper, i18n } = mountView()
    const firstSignal = vi.mocked(getModelPlaza).mock.calls[0]?.[0]?.signal
    i18n.global.locale.value = 'zh'
    await flushPromises()
    expect(firstSignal?.aborted).toBe(true)

    resolveOld({ description: 'stale', groups: [] })
    await flushPromises()
    expect(wrapper.getComponent(ModelPlazaContent).props('response')).toEqual(latest)
    const lastSignal = vi.mocked(getModelPlaza).mock.calls[1]?.[0]?.signal
    wrapper.unmount()
    expect(lastSignal?.aborted).toBe(true)
  })
})
