import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, shallowMount } from '@vue/test-utils'
import type { AdminGroup, ScheduledTestPlan } from '@/types'
import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import ScheduledTestsPanel from '../ScheduledTestsPanel.vue'

const { listByAccount, listPromptTemplates, create, update, showError } = vi.hoisted(() => ({
  listByAccount: vi.fn(),
  listPromptTemplates: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  showError: vi.fn(),
}))
vi.mock('@/api/admin', () => ({
  adminAPI: { scheduledTests: { listByAccount, listPromptTemplates, create, update } },
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess: vi.fn() }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key }),
}))
enableAutoUnmount(afterEach)

const groups = [
  { id: 11, name: 'OpenAI One', platform: 'openai' },
  { id: 12, name: 'OpenAI Two', platform: 'openai' },
  { id: 21, name: 'Gemini Only', platform: 'gemini' },
] as AdminGroup[]
const plan = {
  id: 1, account_id: 9, model_id: 'model-one', cron_expression: '*/30 * * * *',
  enabled: true, max_results: 100, success_add_group_ids: [11, 12],
} as ScheduledTestPlan

async function openPanel() {
  const wrapper = shallowMount(ScheduledTestsPanel, {
    props: {
      show: false, accountId: 9, accountPlatform: 'openai', groups,
      accountGroupIds: [11, 21], modelOptions: [{ value: 'model-one', label: 'Model One' }],
    },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } },
  })
  await wrapper.setProps({ show: true })
  await flushPromises()
  return wrapper
}

describe('scheduled test success groups', () => {
  beforeEach(() => {
    vi.resetAllMocks()
    listByAccount.mockResolvedValue([])
    listPromptTemplates.mockResolvedValue([])
    create.mockResolvedValue(plan)
    update.mockResolvedValue(plan)
  })

  it('lists only account-platform groups and saves multiple selections when creating', async () => {
    const wrapper = await openPanel()
    await wrapper.findAll('button').find(button => button.text().includes('admin.scheduledTests.addPlan'))!.trigger('click')
    expect(wrapper.text()).not.toContain('Gemini Only')
    expect(wrapper.findAll('input[type="checkbox"]').map(input => input.attributes('value'))).toEqual(['11', '11', '12'])

    wrapper.findComponent(Select).vm.$emit('update:modelValue', 'model-one')
    wrapper.findAllComponents(Input).find(input => input.props('placeholder') === '*/30 * * * *')!.vm.$emit('update:modelValue', '*/30 * * * *')
    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    await checkboxes[1].setValue(true)
    await checkboxes[2].setValue(true)
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(create).toHaveBeenCalledWith(expect.objectContaining({ account_id: 9, success_add_group_ids: [11, 12] }))
    expect(showError).not.toHaveBeenCalled()
  })

  it.each([
    { success_add_group_ids: [11, 12], success_add_group_id: null, selected: ['11', '12'] },
    { success_add_group_ids: undefined, success_add_group_id: 11, selected: ['11'] },
  ])('restores saved selections and supports clearing them: $selected', async ({ selected, ...fields }) => {
    listByAccount.mockResolvedValue([{ ...plan, ...fields }])
    const wrapper = await openPanel()
    await wrapper.find('button[title="admin.scheduledTests.editPlan"]').trigger('click')
    expect(wrapper.text()).not.toContain('Gemini Only')
    const checked = wrapper.findAll('input[type="checkbox"]').filter(input => (input.element as HTMLInputElement).checked)
    expect(checked.map(input => input.attributes('value'))).toEqual(selected)
    for (const checkbox of checked) await checkbox.setValue(false)
    await wrapper.findAll('button').find(button => button.text() === 'common.save')!.trigger('click')
    await flushPromises()

    expect(update).toHaveBeenCalledWith(1, expect.objectContaining({ success_add_group_ids: [] }))
    expect(showError).not.toHaveBeenCalled()
  })

  it('discards selections when closing before switching accounts', async () => {
    const wrapper = await openPanel()
    await wrapper.findAll('button').find(button => button.text().includes('admin.scheduledTests.addPlan'))!.trigger('click')
    await wrapper.findAll('input[type="checkbox"]')[1].setValue(true)
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, accountId: 10, accountPlatform: 'gemini', accountGroupIds: [] })
    await flushPromises()
    await wrapper.findAll('button').find(button => button.text().includes('admin.scheduledTests.addPlan'))!.trigger('click')

    expect(wrapper.text()).not.toContain('OpenAI One')
    const checkboxes = wrapper.findAll('input[type="checkbox"]')
    expect(checkboxes.map(input => input.attributes('value'))).toEqual(['21'])
    expect(checkboxes.every(input => !(input.element as HTMLInputElement).checked)).toBe(true)
  })
})
