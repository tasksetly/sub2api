import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import UsageRetentionSettings from '../UsageRetentionSettings.vue'

const { getPolicy, savePolicy } = vi.hoisted(() => ({ getPolicy: vi.fn(), savePolicy: vi.fn() }))
vi.mock('@/api/usageRetention', () => ({ getAdminUsageRetentionSettings: getPolicy, saveUsageRetentionSettings: savePolicy }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess: vi.fn(), showError: vi.fn() }) }))

function render() {
  return mount(UsageRetentionSettings, {
    global: { stubs: {
      Toggle: { props: ['modelValue'], emits: ['update:modelValue'], template: '<button type="button" role="switch" @click="$emit(\'update:modelValue\', !modelValue)" />' },
      ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<button v-if="show" data-confirm type="button" @click="$emit(\'confirm\')">Confirm</button>' }
    } }
  })
}

beforeEach(() => {
  vi.clearAllMocks()
  getPolicy.mockResolvedValue({ enabled: false, retention_days: 90 })
  savePolicy.mockImplementation(async policy => policy)
})

describe('usage retention settings', () => {
  it('requires confirmation before enabling deletion', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('.btn-primary').trigger('click')
    expect(savePolicy).not.toHaveBeenCalled()
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(savePolicy).toHaveBeenCalledWith({ enabled: true, retention_days: 90 })
  })

  it('requires confirmation before shortening an enabled window', async () => {
    getPolicy.mockResolvedValue({ enabled: true, retention_days: 90 })
    const wrapper = render()
    await flushPromises()
    await wrapper.get('input').setValue(30)
    await wrapper.get('.btn-primary').trigger('click')
    expect(wrapper.find('[data-confirm]').exists()).toBe(true)
    expect(savePolicy).not.toHaveBeenCalled()
  })

  it('keeps controls disabled when the policy cannot be read', async () => {
    getPolicy.mockRejectedValue(new Error('unavailable'))
    const wrapper = render()
    await flushPromises()
    expect(wrapper.get('fieldset').attributes()).toHaveProperty('disabled')
    expect(wrapper.get('[role="alert"]').text()).toContain('loadFailed')
    expect(savePolicy).not.toHaveBeenCalled()
  })

  it('rejects zero days instead of deleting everything', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('input').setValue(0)
    expect(wrapper.get('.btn-primary').attributes()).toHaveProperty('disabled')
    expect(savePolicy).not.toHaveBeenCalled()
  })
})
