import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'

import type { TrendDataPoint } from '@/types'
import UserDashboardCharts from '../UserDashboardCharts.vue'

vi.mock('vue-i18n', async (importOriginal) => ({
  ...await importOriginal<typeof import('vue-i18n')>(),
  useI18n: () => ({ t: (key: string) => key, locale: { value: 'en-US' } })
}))

function point(date: string, input: number, read: number, creation: number): TrendDataPoint {
  return {
    date, requests: 1, input_tokens: input, output_tokens: 10,
    cache_read_tokens: read, cache_creation_tokens: creation,
    total_tokens: input + read + creation + 10, cost: 0, actual_cost: 0
  }
}

function mountCharts(trend: TrendDataPoint[]) {
  return mount(UserDashboardCharts, {
    props: {
      loading: false, loadFailed: false, keyLoadFailed: false,
      startDate: '2026-11-12', endDate: '2026-11-18', granularity: 'day',
      apiKeyId: null, apiKeys: [{ id: 17, name: 'Production' }], trend, models: []
    },
    global: { stubs: { Select: true, DateRangePicker: true, TokenUsageTrend: true, Doughnut: true } }
  })
}

describe('UserDashboardCharts cache hit rate', () => {
  it('weights each period by its total input, including cache reads and writes', () => {
    const wrapper = mountCharts([point('2026-11-12', 5, 0, 5), point('2026-11-13', 30, 50, 10)])
    expect(wrapper.get('[data-test="cache-hit-rate"]').text()).toBe('50%')
    wrapper.unmount()
  })

  it('distinguishes no input from a zero hit rate and hides stale values while loading', async () => {
    const wrapper = mountCharts([point('2026-11-12', 0, 0, 0)])
    const rate = wrapper.get('[data-test="cache-hit-rate"]')
    expect(rate.text()).toBe('–')
    await wrapper.setProps({ trend: [point('2026-11-12', 10, 0, 0)] })
    expect(rate.text()).toBe('0%')
    await wrapper.setProps({ loading: true })
    expect(rate.text()).toBe('–')
    wrapper.unmount()
  })

  it('emits the selected API key and supports returning to all keys', () => {
    const wrapper = mountCharts([])
    const keySelect = wrapper.findAllComponents({ name: 'Select' })
      .find((select) => select.props('ariaLabel') === 'dashboard.apiKeys')!
    keySelect.vm.$emit('change', 17)
    keySelect.vm.$emit('change', null)
    expect(wrapper.emitted('update:apiKeyId')).toEqual([[17], [null]])
    expect(wrapper.emitted('apiKeyChange')).toEqual([[], []])
    wrapper.unmount()
  })
})
