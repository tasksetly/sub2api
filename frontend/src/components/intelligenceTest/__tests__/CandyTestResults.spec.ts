import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import { createI18n } from 'vue-i18n'
import CandyTestResults from '../CandyTestResults.vue'
import en from '@/i18n/locales/en/intelligenceTest'
import type { IntelligenceTestItem } from '@/api/intelligenceTest'

const items: IntelligenceTestItem[] = [
  { id: 3, answer_mode: 'candy', group_id: 1, group_name: 'Group A', status: 'failed', created_at: '2026-11-11T12:03:00Z' },
  { id: 2, answer_mode: 'candy', group_id: 1, group_name: 'Group A', status: 'incorrect', response_text: '<script>alert(1)</script>29', created_at: '2026-11-11T12:02:00Z' },
  { id: 1, answer_mode: 'candy', group_id: 1, group_name: 'Group A', status: 'validated', response_text: '21', created_at: '2026-11-11T12:01:00Z' },
]
function render(results = items) {
  return mount(CandyTestResults, {
    props: { response: { items: results, groups: [{ id: 1, name: 'Group A' }, { id: 2, name: 'Group B' }] }, loading: false, error: '' },
    global: {
      plugins: [createI18n({ legacy: false, locale: 'en', messages: { en } })],
      stubs: { BaseDialog: { props: ['show'], template: '<div v-if="show"><slot /></div>' } },
    },
  })
}

describe('candy results', () => {
  it('excludes request failures from the pass rate and displays history from oldest to newest', async () => {
    const wrapper = render()
    try {
      expect(wrapper.get('[data-testid="candy-summary"]').text()).toContain('50.0%')
      const runs = wrapper.findAll('[data-testid="candy-run"]')
      expect(runs).toHaveLength(3)
      expect(runs[0].attributes('aria-label')).toContain('Passed')
      expect(runs[2].attributes('aria-label')).toContain('Request failed')
      await runs[1].trigger('click')
      expect(wrapper.get('[data-testid="candy-answer"]').text()).toBe('<script>alert(1)</script>29')
      expect(wrapper.find('script').exists()).toBe(false)
      await wrapper.get('select').setValue('2')
      expect(wrapper.findAll('[data-testid="candy-run"]')).toHaveLength(0)
      expect(wrapper.get('[data-testid="candy-summary"]').text()).toContain('—')
      expect(wrapper.text()).toContain('No data')
    } finally { wrapper.unmount() }
  })

  it('does not count SVG results or invent a zero percent pass rate when every request fails', () => {
    const wrapper = render([
      items[0],
      { id: 4, answer_mode: 'svg', status: 'validated', svg_content: '<svg/>' },
    ])
    try {
      expect(wrapper.findAll('[data-testid="candy-run"]')).toHaveLength(1)
      expect(wrapper.get('[data-testid="candy-summary"]').text()).toContain('—')
      expect(wrapper.get('[data-testid="candy-summary"]').text()).not.toContain('0.0%')
    } finally { wrapper.unmount() }
  })
})
