import { defineComponent, h } from 'vue'
import { describe, expect, it, vi, beforeEach } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SupplierCostsView from '../SupplierCostsView.vue'

const { getSupplierCosts, showError } = vi.hoisted(() => ({
  getSupplierCosts: vi.fn(),
  showError: vi.fn(),
}))

vi.mock('@/api/admin/usage', async () => {
  const actual = await vi.importActual<typeof import('@/api/admin/usage')>('@/api/admin/usage')
  const usageAPI = {
    ...actual.default,
    getSupplierCosts,
  }
  return {
    ...actual,
    getSupplierCosts,
    adminUsageAPI: usageAPI,
    default: usageAPI,
  }
})

vi.mock('@/stores/app', () => ({
  useAppStore: () => ({ showError }),
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({
      t: (key: string) => key,
    }),
  }
})

const AppLayoutStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  },
})

const DateRangePickerStub = defineComponent({
  emits: ['change'],
  setup(_, { emit }) {
    return () => h('button', {
      'data-test': 'date-range',
      onClick: () => emit('change', {
        startDate: '2026-06-01',
        endDate: '2026-06-07',
        preset: null,
      }),
    }, 'date')
  },
})

const SelectStub = defineComponent({
  props: {
    modelValue: { type: [String, Number, Boolean], default: null },
  },
  emits: ['update:modelValue', 'change'],
  setup(_, { emit }) {
    return () => h('button', {
      'data-test': 'supplier-select',
      onClick: () => {
        emit('update:modelValue', 'alpha')
        emit('change', 'alpha')
      },
    }, 'supplier')
  },
})

const SupplierCostTableStub = defineComponent({
  props: {
    rows: { type: Array, default: () => [] },
    loading: { type: Boolean, default: false },
  },
  template: '<div data-test="supplier-cost-table">{{ rows.length }}|{{ loading }}</div>',
})

const IconStub = defineComponent({
  template: '<span />',
})

describe('SupplierCostsView', () => {
  beforeEach(() => {
    getSupplierCosts.mockReset()
    showError.mockReset()
    getSupplierCosts.mockResolvedValue({
      suppliers: [{
        supplier: 'alpha',
        account_count: 1,
        requests: 2,
        input_tokens: 10,
        output_tokens: 5,
        cache_tokens: 0,
        total_tokens: 15,
        standard_cost: 1,
        account_cost: 1,
        user_billed: 2,
        gross_profit: 1,
        gross_margin: 0.5,
        cost_percentage: 1,
      }],
    })
  })

  it('loads the selected date range and supplier filter through the API', async () => {
    const wrapper = mount(SupplierCostsView, {
      global: {
        stubs: {
          AppLayout: AppLayoutStub,
          DateRangePicker: DateRangePickerStub,
          Select: SelectStub,
          SupplierCostTable: SupplierCostTableStub,
          Icon: IconStub,
        },
      },
    })

    await flushPromises()
    expect(getSupplierCosts).toHaveBeenCalledWith(expect.objectContaining({
      start_date: expect.any(String),
      end_date: expect.any(String),
    }))

    await wrapper.find('[data-test="date-range"]').trigger('click')
    await flushPromises()
    expect(getSupplierCosts).toHaveBeenLastCalledWith({
      supplier: undefined,
      supplier_unset: undefined,
      start_date: '2026-06-01',
      end_date: '2026-06-07',
    })

    await wrapper.find('[data-test="supplier-select"]').trigger('click')
    await flushPromises()
    expect(getSupplierCosts).toHaveBeenLastCalledWith({
      supplier: 'alpha',
      supplier_unset: undefined,
      start_date: '2026-06-01',
      end_date: '2026-06-07',
    })
  })
})
