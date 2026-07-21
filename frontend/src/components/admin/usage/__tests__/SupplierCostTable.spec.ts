import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import SupplierCostTable from '../SupplierCostTable.vue'
import type { SupplierCostStat } from '@/types'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key,
  }),
}))

const row = (overrides: Partial<SupplierCostStat>): SupplierCostStat => ({
  supplier: 'supplier',
  account_count: 1,
  requests: 10,
  input_tokens: 100,
  output_tokens: 50,
  cache_tokens: 0,
  total_tokens: 150,
  standard_cost: 80,
  account_cost: 60,
  user_billed: 100,
  gross_profit: 40,
  gross_margin: 0.4,
  cost_percentage: 0.6,
  ...overrides,
})

describe('SupplierCostTable', () => {
  it('shows each supplier margin and calculates the weighted total gross margin', () => {
    const wrapper = mount(SupplierCostTable, {
      props: {
        rows: [
          row({ supplier: 'alpha' }),
          row({
            supplier: 'beta',
            account_count: 2,
            account_cost: 45,
            user_billed: 50,
            gross_profit: 5,
            gross_margin: 0.1,
            cost_percentage: 0.4,
          }),
        ],
      },
    })

    const text = wrapper.text()
    expect(text).toContain('usage.supplierCost.grossMargin')
    expect(text).toContain('40.00%')
    expect(text).toContain('10.00%')
    expect(wrapper.find('tfoot').text()).toContain('30.00%')
    expect(wrapper.find('tfoot').text()).toContain('3')
  })

  it('uses zero gross margin when no user amount was billed', () => {
    const wrapper = mount(SupplierCostTable, {
      props: {
        rows: [row({ account_cost: 5, user_billed: 0, gross_profit: -5, gross_margin: 0 })],
      },
    })

    expect(wrapper.find('tfoot').text()).toContain('0.00%')
  })
})
