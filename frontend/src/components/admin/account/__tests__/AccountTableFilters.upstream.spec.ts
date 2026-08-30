import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import AccountTableFilters from '../AccountTableFilters.vue'
import type { UpstreamProviderWithStats } from '@/types/upstreamProvider'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

function makeProvider(id: number, name: string): UpstreamProviderWithStats {
  return {
    id,
    name,
    base_url: 'https://up.example.com',
    notes: null,
    username: 'admin',
    has_password: true,
    has_totp_secret: false,
    has_token: false,
    token_expires_at: null,
    rate_correction: 1,
    balance: null,
    frozen_balance: null,
    upstream_concurrency: null,
    status: 'active',
    last_sync_at: null,
    sync_enabled: true,
    created_at: '2026-08-30T00:00:00Z',
    updated_at: '2026-08-30T00:00:00Z',
    account_count: 0,
    group_count: 0,
    local_cost_usd: 0,
    local_requests: 0,
    min_rate: null,
    max_rate: null
  }
}

function mountFilters(props: Record<string, unknown> = {}) {
  return mount(AccountTableFilters, {
    props: {
      searchQuery: '',
      filters: { platform: '', type: '', status: '', privacy_mode: '', group: '', upstream: '' },
      ...props
    }
  })
}

describe('AccountTableFilters upstream select', () => {
  it('offers all sources, any-upstream, and one option per provider', () => {
    const wrapper = mountFilters({
      upstreamProviders: [makeProvider(3, 'up-a'), makeProvider(9, 'up-b')]
    })

    // The upstream select is the last one in the row.
    const selects = wrapper.findAllComponents({ name: 'Select' })
    const upstreamSelect = selects[selects.length - 1]
    const options = upstreamSelect.props('options') as Array<{ value: string; label: string }>

    expect(options.map((option) => option.value)).toEqual(['', 'any', '3', '9'])
    expect(options.map((option) => option.label)).toEqual([
      'admin.accounts.allUpstreams',
      'admin.accounts.anyUpstream',
      'up-a',
      'up-b'
    ])
  })

  it('emits the picked provider id without dropping sibling filters', async () => {
    const wrapper = mountFilters({
      filters: {
        platform: 'anthropic',
        type: '',
        status: '',
        privacy_mode: '',
        group: '',
        upstream: ''
      },
      upstreamProviders: [makeProvider(3, 'up-a')]
    })

    const selects = wrapper.findAllComponents({ name: 'Select' })
    const upstreamSelect = selects[selects.length - 1]
    upstreamSelect.vm.$emit('update:modelValue', '3')
    await wrapper.vm.$nextTick()

    const emitted = wrapper.emitted('update:filters')
    expect(emitted).toHaveLength(1)
    // Sibling filters must survive: the handler spreads the existing object.
    expect(emitted![0][0]).toMatchObject({ platform: 'anthropic', upstream: '3' })
  })

  it('still renders the select when no providers are loaded', () => {
    const wrapper = mountFilters()

    const selects = wrapper.findAllComponents({ name: 'Select' })
    const options = selects[selects.length - 1].props('options') as Array<{ value: string }>
    // A failed provider fetch leaves the dropdown usable rather than empty.
    expect(options.map((option) => option.value)).toEqual(['', 'any'])
  })
})
