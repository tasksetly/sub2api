import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import BulkTestAccountModal from '../BulkTestAccountModal.vue'

const { getAvailableModels } = vi.hoisted(() => ({
  getAvailableModels: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    accounts: {
      getAvailableModels
    }
  }
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key
    })
  }
})

function mountModal() {
  return mount(BulkTestAccountModal, {
    props: {
      show: false,
      accountIds: [11, 22],
      testing: false
    },
    global: {
      stubs: {
        BaseDialog: { template: '<div><slot /><slot name="footer" /></div>' },
        Select: {
          props: ['modelValue', 'options'],
          emits: ['update:modelValue'],
          template: '<select data-test="model-select" :value="modelValue" @change="$emit(\'update:modelValue\', $event.target.value)"><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>'
        },
        Icon: true
      }
    }
  })
}

describe('BulkTestAccountModal', () => {
  beforeEach(() => {
    getAvailableModels.mockReset()
  })

  it('shows only models shared by every selected account and confirms the selected model', async () => {
    getAvailableModels
      .mockResolvedValueOnce([
        { id: 'gpt-5.4', display_name: 'GPT-5.4' },
        { id: 'gpt-4.1', display_name: 'GPT-4.1' }
      ])
      .mockResolvedValueOnce([
        { id: 'gpt-5.4', display_name: 'GPT-5.4' },
        { id: 'gpt-5.3', display_name: 'GPT-5.3' }
      ])

    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    expect(getAvailableModels).toHaveBeenCalledTimes(2)
    expect(getAvailableModels).toHaveBeenCalledWith(11)
    expect(getAvailableModels).toHaveBeenCalledWith(22)
    expect(wrapper.findAll('[data-test="model-select"] option').map(option => option.text())).toEqual(['GPT-5.4'])

    const startButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.bulkActions.startTest'))
    expect(startButton).toBeTruthy()
    await startButton!.trigger('click')

    expect(wrapper.emitted('confirm')).toEqual([['gpt-5.4']])
  })

  it('disables testing when selected accounts have no common model', async () => {
    getAvailableModels
      .mockResolvedValueOnce([{ id: 'gpt-5.4', display_name: 'GPT-5.4' }])
      .mockResolvedValueOnce([{ id: 'claude-sonnet-4', display_name: 'Claude Sonnet 4' }])

    const wrapper = mountModal()
    await wrapper.setProps({ show: true })
    await flushPromises()

    const startButton = wrapper.findAll('button').find(button => button.text().includes('admin.accounts.bulkActions.startTest'))
    expect(startButton).toBeTruthy()
    expect((startButton!.element as HTMLButtonElement).disabled).toBe(true)
    await startButton!.trigger('click')

    expect(wrapper.emitted('confirm')).toBeUndefined()
  })
})
