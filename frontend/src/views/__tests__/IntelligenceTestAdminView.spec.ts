import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import IntelligenceTestAdminView from '../admin/IntelligenceTestAdminView.vue'

const { listConfigs, listAPIKeys, listModels, create, listPromptTemplates, createPromptTemplate } = vi.hoisted(() => ({
  listConfigs: vi.fn(), listAPIKeys: vi.fn(), listModels: vi.fn(), create: vi.fn(),
  listPromptTemplates: vi.fn(), createPromptTemplate: vi.fn(),
}))
vi.mock('@/api/admin', () => ({ adminAPI: {
  intelligenceTest: { listConfigs, listAPIKeys, listModels, create },
  scheduledTests: { listPromptTemplates, createPromptTemplate },
} }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))

const keys = [1, 2].map(id => ({ id, name: `Key ${id}`, group_name: `Group ${id}`, key_prefix: `prefix-${id}` }))
function mountView() {
  return mount(IntelligenceTestAdminView, { global: { stubs: {
    AppLayout: { template: '<div><slot /></div>' },
    BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' },
    Icon: true, RouterLink: true,
    Select: {
      props: ['modelValue', 'options', 'disabled'], emits: ['update:modelValue'],
      template: `<select :value="modelValue" :disabled="disabled" @change="$emit('update:modelValue', $event.target.value)"><option value="" /><option v-for="item in options" :key="item.value" :value="item.value">{{ item.label }}</option></select>`,
    },
    TextArea: {
      props: ['modelValue'], emits: ['update:modelValue'], methods: { focus() {} },
      template: `<textarea :value="modelValue" @input="$emit('update:modelValue', $event.target.value)" />`,
    },
  } } })
}

async function openCreate(wrapper: ReturnType<typeof mountView>) {
  await flushPromises()
  await wrapper.get('button.btn-primary').trigger('click')
  await flushPromises()
}

beforeEach(() => {
  vi.resetAllMocks()
  listConfigs.mockResolvedValue([])
  listAPIKeys.mockResolvedValue(keys)
  listModels.mockImplementation(async (id: number) => [{ id: `model-${id}`, display_name: `Model ${id}` }])
  listPromptTemplates.mockResolvedValue([{ id: 4, name: 'Saved SVG', prompt_text: 'Draw a bird', expected_answer: '' }])
  create.mockResolvedValue({ id: 1 })
})

describe('Intelligence test configuration', () => {
  it('loads models for the selected key and reuses the shared prompt templates', async () => {
    const wrapper = mountView()
    try {
      await openCreate(wrapper)
      expect(listModels).toHaveBeenCalledWith(1, expect.any(AbortSignal))
      expect((wrapper.get('#intelligence-model').element as HTMLSelectElement).value).toBe('model-1')
      await wrapper.get('input[required]').setValue('Scheduled SVG')
      await wrapper.get('#svg-test-template').setValue('4')
      expect((wrapper.get('textarea').element as HTMLTextAreaElement).value).toBe('Draw a bird')
      await wrapper.get('#intelligence-api-key').setValue('2')
      await flushPromises()
      expect((wrapper.get('#intelligence-model').element as HTMLSelectElement).value).toBe('model-2')
      expect(wrapper.find('input[placeholder="例如：gpt-4o"]').exists()).toBe(false)
      await wrapper.get('form').trigger('submit')
      await flushPromises()
      expect(create).toHaveBeenCalledWith(expect.objectContaining({ api_key_id: 2, model_id: 'model-2', prompt_text: 'Draw a bird' }))
    } finally { wrapper.unmount() }
  })

  it('ignores model responses for a key that is no longer selected', async () => {
    let resolveFirst!: (models: { id: string }[]) => void
    listModels.mockImplementationOnce(() => new Promise(resolve => { resolveFirst = resolve }))
    const wrapper = mountView()
    try {
      await openCreate(wrapper)
      const firstSignal = listModels.mock.calls[0][1] as AbortSignal
      await wrapper.get('#intelligence-api-key').setValue('2')
      await flushPromises()
      expect(firstSignal.aborted).toBe(true)
      resolveFirst([{ id: 'stale-model' }])
      await flushPromises()
      expect((wrapper.get('#intelligence-model').element as HTMLSelectElement).value).toBe('model-2')
      expect(wrapper.get('#intelligence-model').text()).not.toContain('stale-model')
    } finally { wrapper.unmount() }
  })

  it('blocks saving when model discovery fails and retries without manual model entry', async () => {
    listModels.mockRejectedValueOnce({ message: 'Model discovery unavailable' })
    const wrapper = mountView()
    try {
      await openCreate(wrapper)
      await wrapper.get('#intelligence-mode').setValue('candy')
      await wrapper.get('input[required]').setValue('Scheduled candy')
      expect(wrapper.text()).toContain('Model discovery unavailable')
      expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeDefined()
      await wrapper.get('form').trigger('submit')
      expect(create).not.toHaveBeenCalled()
      await wrapper.findAll('button').find(button => button.text() === 'common.refresh')!.trigger('click')
      await flushPromises()
      expect(wrapper.get('button[type="submit"]').attributes('disabled')).toBeUndefined()
      await wrapper.get('form').trigger('submit')
      await flushPromises()
      expect(create).toHaveBeenCalledWith(expect.objectContaining({ answer_mode: 'candy', api_key_id: 1, model_id: 'model-1' }))
    } finally { wrapper.unmount() }
  })
})
