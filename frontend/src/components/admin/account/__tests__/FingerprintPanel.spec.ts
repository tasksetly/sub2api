import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { enableAutoUnmount, flushPromises, mount } from '@vue/test-utils'

import FingerprintPanel from '../FingerprintPanel.vue'
import type { FingerprintOverview, FingerprintPlan, FingerprintResult } from '@/types'

const { overview, referenceModels, listByAccount, listResults, create, runNow, showError, showSuccess } =
  vi.hoisted(() => ({
    overview: vi.fn(),
    referenceModels: vi.fn(),
    listByAccount: vi.fn(),
    listResults: vi.fn(),
    create: vi.fn(),
    runNow: vi.fn(),
    showError: vi.fn(),
    showSuccess: vi.fn()
  }))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    fingerprint: {
      overview,
      referenceModels,
      listByAccount,
      listResults,
      create,
      runNow,
      update: vi.fn(),
      delete: vi.fn()
    }
  }
}))
vi.mock('@/stores/app', () => ({ useAppStore: () => ({ showError, showSuccess }) }))
vi.mock('vue-i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('vue-i18n')>()),
  useI18n: () => ({
    // 保留插值参数，否则 cost / consecutiveMismatches 里的数字不会出现在渲染结果里，
    // 断言就会变成"看起来过了其实什么都没测"。
    t: (key: string, params?: Record<string, unknown>) =>
      params ? `${key} ${Object.values(params).join(' ')}` : key
  })
}))

enableAutoUnmount(afterEach)

const supportedOverview: FingerprintOverview = {
  eligibility: { supported: true, proxy_bypassed: false },
  monthly_estimated_tokens: 48210,
  reference_built_at: '2026-11-03T12:06:39Z'
}

const models = [
  { id: 'claude-opus-5', display_name: 'claude-opus-5', family: 'claude', family_name: 'Claude' },
  { id: 'deepseek-v4-flash', display_name: 'deepseek-v4-flash', family: 'deepseek', family_name: 'DeepSeek' }
]

const plan = {
  id: 1,
  account_id: 42,
  test_model: 'opus-max',
  expected_model: 'claude-opus-5',
  api: 'chat_completions',
  challenge_count: 3,
  cron_expression: '0 */6 * * *',
  enabled: true,
  max_results: 50,
  consecutive_mismatches: 1,
  mismatch_action_threshold: 2,
  failure_disable: false,
  failure_temp_unschedulable_minutes: 0,
  last_run_at: null,
  next_run_at: null,
  created_at: '2026-11-04T00:00:00Z',
  updated_at: '2026-11-04T00:00:00Z'
} as FingerprintPlan

function result(partial: Partial<FingerprintResult>): FingerprintResult {
  return {
    id: 1,
    plan_id: 1,
    verdict: 'match',
    decision: 'scored',
    predicted_model: 'claude-opus-5',
    predicted_model_name: 'claude-opus-5',
    predicted_family: 'claude',
    probability: 0.92,
    error_message: '',
    skip_reason: '',
    estimated_output_tokens: 2400,
    latency_ms: 8300,
    started_at: '2026-11-04T01:00:00Z',
    finished_at: '2026-11-04T01:00:09Z',
    created_at: '2026-11-04T01:00:09Z',
    ...partial
  }
}

// 默认 mock 放在 beforeEach：用例在自己的函数体里改 mock 时才能真正生效。
// 之前默认值写在 setup() 里，而用例是先改 mock 再调 setup()，被无条件覆盖回空数组，
// 四态区分 / 失败原因 / 候选排名 / 单次消耗这四条断言实际上什么都没验证。
beforeEach(() => {
  overview.mockResolvedValue(supportedOverview)
  referenceModels.mockResolvedValue(models)
  listByAccount.mockResolvedValue([plan])
  listResults.mockResolvedValue([])
})

afterEach(() => vi.resetAllMocks())

function setup(over: Partial<FingerprintOverview> = {}, plans: FingerprintPlan[] = [plan]) {
  if (Object.keys(over).length > 0) {
    overview.mockResolvedValue({ ...supportedOverview, ...over })
  }
  listByAccount.mockResolvedValue(plans)
  return mount(FingerprintPanel, { props: { accountId: 42 } })
}

describe('FingerprintPanel', () => {
  it('账号不支持时禁用新建并显示原因', async () => {
    const wrapper = setup({
      eligibility: { supported: false, reason: 'unsupported_account_type', proxy_bypassed: false }
    })
    await flushPromises()

    expect(wrapper.find('[data-testid="fingerprint-unsupported"]').exists()).toBe(true)
    expect(wrapper.text()).toContain('unsupported_account_type')
    expect(wrapper.find('[data-testid="fingerprint-create"]').exists()).toBe(false)
  })

  it('预期模型下拉只来自参考库', async () => {
    const wrapper = setup()
    await flushPromises()

    const select = wrapper.findComponent({ name: 'Select' })
    expect(select.exists()).toBe(true)
    const options = select.props('options') as Array<{ value: string }>
    expect(options.map((option) => option.value)).toEqual(['claude-opus-5', 'deepseek-v4-flash'])
  })

  it('参考库拉不到时不允许建计划', async () => {
    referenceModels.mockResolvedValue([])
    const wrapper = setup({}, [])
    await flushPromises()

    const button = wrapper.find('[data-testid="fingerprint-create"]')
    expect(button.attributes('disabled')).toBeDefined()
  })

  it('配了代理的账号显示结论仅供参考的提示', async () => {
    const wrapper = setup({ eligibility: { supported: true, proxy_bypassed: true } })
    await flushPromises()

    expect(wrapper.text()).toContain('admin.accounts.fingerprint.proxyBypassHint')
  })

  it('四态结果各自显示不同样式', async () => {
    listResults.mockResolvedValue([
      result({ id: 1, verdict: 'match' }),
      result({ id: 2, verdict: 'mismatch', predicted_model: 'deepseek-v4-flash' }),
      result({ id: 3, verdict: 'inconclusive', error_message: '有效回答不足，无法评分' }),
      result({ id: 4, verdict: 'error', error_message: '429 Too Many Requests' })
    ])
    const wrapper = setup()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text().includes('results'))!.trigger('click')
    await flushPromises()

    const verdicts = wrapper.findAll('[data-verdict]').map((el) => el.attributes('data-verdict'))
    expect(verdicts).toEqual(['match', 'mismatch', 'inconclusive', 'error'])
  })

  it('无法判定与检测失败必须显示具体原因', async () => {
    listResults.mockResolvedValue([
      result({ id: 3, verdict: 'error', error_message: '429 Too Many Requests' }),
      result({ id: 4, verdict: 'error', skip_reason: 'daily_limit_global', error_message: '' })
    ])
    const wrapper = setup()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text().includes('results'))!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="fingerprint-error-message"]').text()).toContain('429')
    expect(wrapper.find('[data-testid="fingerprint-skip-reason"]').text()).toContain('daily_limit_global')
  })

  it('展示完整候选排名与置信度', async () => {
    listResults.mockResolvedValue([
      result({
        verdict: 'mismatch',
        predicted_model: 'deepseek-v4-flash',
        analysis_json: JSON.stringify({
          results: [
            { model: 'deepseek-v4-flash', display_name: 'deepseek-v4-flash', probability: 0.87 },
            { model: 'claude-opus-5', display_name: 'claude-opus-5', probability: 0.09 }
          ]
        })
      })
    ])
    const wrapper = setup()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text().includes('results'))!.trigger('click')
    await flushPromises()

    const candidates = wrapper.find('[data-testid="fingerprint-candidates"]')
    expect(candidates.text()).toContain('deepseek-v4-flash')
    expect(candidates.text()).toContain('87.0%')
    expect(candidates.text()).toContain('claude-opus-5')
  })

  it('analysis_json 损坏时不炸面板', async () => {
    listResults.mockResolvedValue([result({ analysis_json: 'not json' })])
    const wrapper = setup()
    await flushPromises()

    await wrapper.findAll('button').find((b) => b.text().includes('results'))!.trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="fingerprint-result"]').exists()).toBe(true)
    expect(wrapper.find('[data-testid="fingerprint-candidates"]').exists()).toBe(false)
  })

  it('显示每次检测的消耗与本月累计消耗', async () => {
    listResults.mockResolvedValue([result({ estimated_output_tokens: 2400 })])
    const wrapper = setup()
    await flushPromises()

    expect(wrapper.text()).toContain('48,210')

    await wrapper.findAll('button').find((b) => b.text().includes('results'))!.trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="fingerprint-result"]').text()).toContain('2400')
  })

  it('显示连续不符计数与动账号阈值', async () => {
    const wrapper = setup()
    await flushPromises()
    expect(wrapper.text()).toContain('admin.accounts.fingerprint.consecutiveMismatches')
  })

  it('概览拉不到时不允许建计划（资格未知不放行）', async () => {
    overview.mockRejectedValue(new Error('offline'))
    const wrapper = mount(FingerprintPanel, { props: { accountId: 42 } })
    await flushPromises()

    const button = wrapper.find('[data-testid="fingerprint-create"]')
    expect(button.exists()).toBe(true)
    expect(button.attributes('disabled')).toBeDefined()
  })
})
