import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import GlobalIPBlacklistSettings from '../GlobalIPBlacklistSettings.vue'

const { getPolicy, savePolicy, showSuccess, showError } = vi.hoisted(() => ({
  getPolicy: vi.fn(), savePolicy: vi.fn(), showSuccess: vi.fn(), showError: vi.fn()
}))
vi.mock('@/api/admin/settings', () => ({ getGlobalIPBlacklistSettings: getPolicy, updateGlobalIPBlacklistSettings: savePolicy }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showSuccess, showError }) }))

const initialVersion = 'a'.repeat(64)
const latestVersion = 'b'.repeat(64)
const responseFields = { current_ip: '192.0.2.1', max_rules: 5000 }

function render() {
  return mount(GlobalIPBlacklistSettings, {
    global: { stubs: {
      Icon: true,
      Toggle: { props: ['modelValue'], emits: ['update:modelValue'], template: '<button type="button" role="switch" :aria-checked="modelValue" @click="$emit(\'update:modelValue\', !modelValue)" />' },
      ConfirmDialog: { props: ['show'], emits: ['confirm', 'cancel'], template: '<button v-if="show" data-confirm type="button" @click="$emit(\'confirm\')">Confirm</button>' }
    } }
  })
}

beforeEach(() => {
  vi.resetAllMocks()
  getPolicy.mockResolvedValue({ ...responseFields, enabled: true, rules: ['198.51.100.1'], version: initialVersion })
  savePolicy.mockImplementation(async policy => ({ ...responseFields, ...policy, version: latestVersion }))
})

describe('global IP blacklist version conflicts', () => {
  it('preserves the draft on conflict and requires a successful refresh before saving', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('textarea').setValue('198.51.100.20\n203.0.113.20')
    savePolicy.mockRejectedValueOnce({ status: 409, reason: 'IP_BLACKLIST_VERSION_CONFLICT' })
    await wrapper.get('.btn-primary').trigger('click')
    await flushPromises()
    expect(savePolicy).toHaveBeenCalledWith({
      enabled: false, rules: ['198.51.100.20', '203.0.113.20'], version: initialVersion
    })
    expect(wrapper.get('textarea').element.value).toBe('198.51.100.20\n203.0.113.20')
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('false')
    expect(wrapper.get('[role="alert"]').text()).toContain('globalIPBlacklist.conflict')
    expect(wrapper.get('.btn-primary').attributes()).toHaveProperty('disabled')
    expect(showSuccess).not.toHaveBeenCalled()
    expect(getPolicy).toHaveBeenCalledTimes(1)
    await wrapper.get('.btn-primary').trigger('click')
    expect(savePolicy).toHaveBeenCalledTimes(1)

    getPolicy.mockRejectedValueOnce(new Error('temporarily unavailable'))
    await wrapper.get('.btn-ghost').trigger('click')
    await flushPromises()
    expect(wrapper.get('textarea').element.value).toBe('198.51.100.20\n203.0.113.20')
    expect(wrapper.get('.btn-primary').attributes()).toHaveProperty('disabled')
    expect(wrapper.text()).toContain('globalIPBlacklist.conflict')

    getPolicy.mockResolvedValueOnce({ ...responseFields, enabled: true, rules: ['198.51.100.1', '203.0.113.10'], version: latestVersion })
    await wrapper.get('.btn-ghost').trigger('click')
    await flushPromises()
    expect(wrapper.find('[role="alert"]').exists()).toBe(false)
    expect(wrapper.get('textarea').element.value).toBe('198.51.100.1\n203.0.113.10')
    expect(wrapper.get('[role="switch"]').attributes('aria-checked')).toBe('true')
    expect(wrapper.get('.btn-primary').attributes()).not.toHaveProperty('disabled')
    await wrapper.get('textarea').setValue('198.51.100.1\n203.0.113.10\n203.0.113.20')
    await wrapper.get('.btn-primary').trigger('click')
    await wrapper.get('[data-confirm]').trigger('click')
    await flushPromises()
    expect(savePolicy).toHaveBeenLastCalledWith({
      enabled: true, rules: ['198.51.100.1', '203.0.113.10', '203.0.113.20'], version: latestVersion
    })
    expect(showSuccess).toHaveBeenCalledTimes(1)
  })

  it('uses the version returned by a successful save for the next edit', async () => {
    const wrapper = render()
    await flushPromises()
    await wrapper.get('[role="switch"]').trigger('click')
    await wrapper.get('.btn-primary').trigger('click')
    await flushPromises()
    await wrapper.get('textarea').setValue('203.0.113.20')
    await wrapper.get('.btn-primary').trigger('click')
    await flushPromises()
    expect(savePolicy).toHaveBeenLastCalledWith({ enabled: false, rules: ['203.0.113.20'], version: latestVersion })
    expect(showSuccess).toHaveBeenCalledTimes(2)
    expect(showError).not.toHaveBeenCalled()
  })
})
