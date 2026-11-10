import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountIntelligenceTestPanel from '../AccountIntelligenceTestPanel.vue'

const { runAccount, listAccountResults, copyToClipboard, listPromptTemplates, createPromptTemplate } = vi.hoisted(() => ({
  runAccount: vi.fn(), listAccountResults: vi.fn(), copyToClipboard: vi.fn(),
  listPromptTemplates: vi.fn(), createPromptTemplate: vi.fn()
}))
vi.mock('@/api/admin', () => ({
  adminAPI: {
    intelligenceTest: { runAccount, listAccountResults },
    scheduledTests: { listPromptTemplates, createPromptTemplate }
  }
}))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const saved = {
  id: 7, account_id: 42, model_id: 'chosen-model', prompt_text: 'Draw my bird',
  status: 'validated', check: true, svg_content: '<svg><circle r="10"/></svg>',
  created_at: '2026-11-10T12:00:00Z', latency_ms: 100
}

function mountPanel(accountId = 42) {
  return mount(AccountIntelligenceTestPanel, {
    props: {
      account: { id: accountId, name: 'Chosen account', platform: 'openai' } as any,
      models: [{ id: 'chosen-model', type: 'model', display_name: 'Chosen model', created_at: saved.created_at }], loadingModels: false
    },
    global: {
      stubs: {
        Select: {
          props: ['modelValue', 'options', 'disabled'], emits: ['update:modelValue'],
          template: `<select :value="modelValue" :disabled="disabled" @change="$emit('update:modelValue', $event.target.value)"><option value="" /><option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option></select>`
        },
        TextArea: {
          props: ['modelValue'], emits: ['update:modelValue'],
          methods: { focus() {} },
          template: '<textarea :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        }
      }
    }
  })
}

describe('AccountIntelligenceTestPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.resetAllMocks()
    listAccountResults.mockResolvedValue([])
    listPromptTemplates.mockResolvedValue([])
  })
  afterEach(() => { vi.useRealTimers() })

  it('requires a prompt and saves a test for the selected account and model', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.find('textarea').setValue('Draw my bird')
    await wrapper.find('#svg-test-effort').setValue('high')
    const running = { ...saved, status: 'running', svg_content: '' }
    runAccount.mockResolvedValue(running)
    listAccountResults.mockResolvedValue([running])
    await wrapper.find('form').trigger('submit')
    await flushPromises()
    expect(runAccount).toHaveBeenCalledWith(42, {
      model_id: 'chosen-model', prompt_text: 'Draw my bird', reasoning_effort: 'high'
    })
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    listAccountResults.mockResolvedValue([saved])
    await vi.advanceTimersByTimeAsync(3000)
    await flushPromises()
    expect(wrapper.find('iframe').attributes('sandbox')).toBe('')
    expect(wrapper.find('iframe').attributes('srcdoc')).toContain(saved.svg_content)
    expect(wrapper.find('a[download]').attributes('href')).toContain(encodeURIComponent(saved.svg_content))
    wrapper.unmount()
  })

  it('restores persisted SVG history when reopened and stops polling on close', async () => {
    listAccountResults.mockResolvedValue([{ ...saved, status: 'running', svg_content: '' }])
    const first = mountPanel()
    await flushPromises()
    first.unmount()
    const count = listAccountResults.mock.calls.length
    await vi.advanceTimersByTimeAsync(3000)
    expect(listAccountResults).toHaveBeenCalledTimes(count)

    listAccountResults.mockResolvedValue([saved])
    const reopened = mountPanel()
    await flushPromises()
    expect(reopened.text()).toContain('Draw my bird')
    expect(reopened.find('iframe').attributes('srcdoc')).toContain("script-src 'none'")
    expect(reopened.find('iframe').attributes('srcdoc')).toContain(saved.svg_content)
    expect(runAccount).not.toHaveBeenCalled()
    reopened.unmount()
  })

  it('shows a failed SVG result without treating it as a successful preview', async () => {
    listAccountResults.mockResolvedValue([{ ...saved, status: 'failed', svg_content: '', error: 'svg_missing_or_invalid' }])
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.svgTest.invalidSVG')
    expect(wrapper.find('iframe').exists()).toBe(false)
    wrapper.unmount()
  })

  it('exposes model options in the Select value/label format', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('#svg-test-model option[value="chosen-model"]').text()).toBe('Chosen model')
    expect((wrapper.find('#svg-test-model').element as HTMLSelectElement).value).toBe('chosen-model')
    wrapper.unmount()
  })

  it('saves a named prompt without running a test and reloads it for another account', async () => {
    const template = { id: 9, name: 'Bird drawing', prompt_text: 'Draw my bird', expected_answer: '' }
    createPromptTemplate.mockResolvedValue(template)
    const first = mountPanel()
    await flushPromises()
    await first.find('textarea').setValue('  Draw my bird  ')
    const saveButton = first.findAll('button').find(button => button.text() === 'admin.accounts.svgTest.saveTemplate')!
    expect(saveButton.attributes('disabled')).toBeDefined()
    await first.find('#svg-test-template-name').setValue(' Bird drawing ')
    expect(saveButton.attributes('disabled')).toBeUndefined()
    await saveButton.trigger('click')
    await flushPromises()
    expect(createPromptTemplate).toHaveBeenCalledWith({ name: 'Bird drawing', prompt_text: 'Draw my bird', expected_answer: '' })
    expect(first.text()).toContain('admin.accounts.svgTest.templateSaved')
    expect(runAccount).not.toHaveBeenCalled()
    first.unmount()

    listPromptTemplates.mockResolvedValue([template])
    const second = mountPanel(43)
    await flushPromises()
    await second.find('#svg-test-template').setValue('9')
    expect((second.find('textarea').element as HTMLTextAreaElement).value).toBe('Draw my bird')
    await second.find('textarea').setValue('Draw a different bird')
    expect((second.find('#svg-test-template').element as HTMLSelectElement).value).toBe('')
    expect(createPromptTemplate).toHaveBeenCalledTimes(1)
    expect(runAccount).not.toHaveBeenCalled()
    second.unmount()
  })

  it('reuses historical prompts while keeping the chosen model and effort', async () => {
    listAccountResults.mockResolvedValue([{ ...saved, model_id: 'previous-model', effort: 'low' }])
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.find('#svg-test-effort').setValue('high')
    const reuseButton = wrapper.findAll('button').find(button => button.text() === 'admin.accounts.svgTest.reusePrompt')!
    await reuseButton.trigger('click')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe(saved.prompt_text)
    expect((wrapper.find('#svg-test-model').element as HTMLSelectElement).value).toBe('chosen-model')
    expect((wrapper.find('#svg-test-effort').element as HTMLSelectElement).value).toBe('high')
    expect(runAccount).not.toHaveBeenCalled()
    wrapper.unmount()
  })

  it('preserves the prompt and name when saving fails', async () => {
    createPromptTemplate.mockRejectedValue({ message: 'Storage unavailable' })
    const wrapper = mountPanel()
    await flushPromises()
    await wrapper.find('textarea').setValue('Draw a bird')
    await wrapper.find('#svg-test-template-name').setValue('Bird')
    await wrapper.findAll('button').find(button => button.text() === 'admin.accounts.svgTest.saveTemplate')!.trigger('click')
    await flushPromises()
    expect(wrapper.text()).toContain('Storage unavailable')
    expect((wrapper.find('textarea').element as HTMLTextAreaElement).value).toBe('Draw a bird')
    expect((wrapper.find('#svg-test-template-name').element as HTMLInputElement).value).toBe('Bird')
    expect(wrapper.text()).not.toContain('admin.accounts.svgTest.templateSaved')
    wrapper.unmount()
  })
})
