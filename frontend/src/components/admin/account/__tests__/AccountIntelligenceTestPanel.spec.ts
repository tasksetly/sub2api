import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import AccountIntelligenceTestPanel from '../AccountIntelligenceTestPanel.vue'

const { runAccount, listAccountResults, copyToClipboard } = vi.hoisted(() => ({
  runAccount: vi.fn(), listAccountResults: vi.fn(), copyToClipboard: vi.fn()
}))
vi.mock('@/api/admin', () => ({ adminAPI: { intelligenceTest: { runAccount, listAccountResults } } }))
vi.mock('@/composables/useClipboard', () => ({ useClipboard: () => ({ copyToClipboard }) }))
vi.mock('@/utils/format', () => ({ formatDateTime: (value: string) => value }))
vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const saved = {
  id: 7, account_id: 42, model_id: 'chosen-model', prompt_text: 'Draw my bird',
  status: 'validated', check: true, svg_content: '<svg><circle r="10"/></svg>',
  created_at: '2026-11-10T12:00:00Z', latency_ms: 100
}

function mountPanel() {
  return mount(AccountIntelligenceTestPanel, {
    props: {
      account: { id: 42, name: 'Chosen account', platform: 'openai' } as any,
      models: [{ id: 'chosen-model', display_name: 'Chosen model' }], loadingModels: false
    },
    global: {
      stubs: {
        Select: { props: ['modelValue'], template: '<div />' },
        TextArea: {
          props: ['modelValue'], emits: ['update:modelValue'],
          template: '<textarea :value="modelValue" @input="$emit(\'update:modelValue\', $event.target.value)" />'
        }
      }
    }
  })
}

describe('AccountIntelligenceTestPanel', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    vi.clearAllMocks()
    listAccountResults.mockResolvedValue([])
  })
  afterEach(() => { vi.useRealTimers() })

  it('requires a prompt and saves a test for the selected account and model', async () => {
    const wrapper = mountPanel()
    await flushPromises()
    expect(wrapper.find('button[type="submit"]').attributes('disabled')).toBeDefined()
    await wrapper.find('textarea').setValue('Draw my bird')
    await wrapper.find('select').setValue('high')
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
})
