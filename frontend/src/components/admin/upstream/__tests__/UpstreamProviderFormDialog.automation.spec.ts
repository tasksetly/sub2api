import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'
import UpstreamProviderFormDialog from '../UpstreamProviderFormDialog.vue'
import type { UpstreamProviderWithStats } from '@/types/upstreamProvider'

vi.mock('vue-i18n', () => ({ useI18n: () => ({ t: (key: string) => key }) }))

const provider: UpstreamProviderWithStats = {
  id: 1, provider_type: 'newapi', name: 'upstream', base_url: 'https://example.com', notes: null,
  has_password: false, has_totp_secret: false, has_token: true, has_refresh_token: false,
  token_expires_at: null, rate_correction: 1, balance: 5, frozen_balance: 0,
  upstream_concurrency: null, status: 'active', last_sync_at: null, sync_enabled: true,
  created_at: '', updated_at: '', account_count: 0, group_count: 0, local_cost_usd: 0,
  local_requests: 0, min_rate: null, max_rate: null,
  automation: {
    sync_cron: 'CRON_TZ=Asia/Shanghai 0 9 * * *', balance_alert_enabled: true,
    balance_alert_threshold: 10, balance_alert_email: 'ops@example.com',
    next_sync_at: null, balance_alert_sent_at: null
  }
}

function mountDialog(editing: UpstreamProviderWithStats | null) {
  return mount(UpstreamProviderFormDialog, {
    props: { show: true, editing, submitting: false },
    global: { stubs: { BaseDialog: { template: '<div><slot /></div>' } } }
  })
}

describe('upstream automation form', () => {
  it('loads and submits the saved schedule and recipient without delivery state', async () => {
    const wrapper = mountDialog(provider)
    expect((wrapper.get('#upstream-sync-cron').element as HTMLInputElement).value).toBe(provider.automation!.sync_cron)
    await wrapper.get('#upstream-balance-email').setValue('new@example.com')
    await wrapper.get('#upstream-balance-threshold').setValue('20')
    await wrapper.get('form').trigger('submit')
    expect(wrapper.emitted('submit')?.[0]?.[0]).toMatchObject({
      automation: {
        sync_cron: provider.automation!.sync_cron, balance_alert_enabled: true,
        balance_alert_threshold: 20, balance_alert_email: 'new@example.com'
      }
    })
    expect((wrapper.emitted('submit')![0][0] as { automation: object }).automation).not.toHaveProperty('balance_alert_sent_at')
  })

  it('resets automation when switching from editing to creation', async () => {
    const wrapper = mountDialog(provider)
    await wrapper.setProps({ show: false })
    await wrapper.setProps({ show: true, editing: null })
    expect((wrapper.get('#upstream-sync-cron').element as HTMLInputElement).value).toBe('*/30 * * * *')
    expect(wrapper.find('#upstream-balance-email').exists()).toBe(false)
  })
})
