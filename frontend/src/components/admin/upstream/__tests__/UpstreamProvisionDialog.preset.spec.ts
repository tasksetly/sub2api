import { describe, expect, it, vi } from 'vitest'
import { mount } from '@vue/test-utils'

import UpstreamProvisionDialog from '../UpstreamProvisionDialog.vue'
import type { UpstreamGroup } from '@/types/upstreamProvider'

vi.mock('vue-i18n', () => ({
  useI18n: () => ({
    t: (key: string) => key
  })
}))

// BaseDialog teleports to body; stub it so the form renders inside the wrapper.
const BaseDialogStub = {
  props: ['show', 'title', 'width'],
  template: '<div v-if="show"><slot /></div>'
}

function makeGroup(remoteGroupID: number, name: string): UpstreamGroup {
  return {
    id: remoteGroupID * 10,
    upstream_provider_id: 1,
    remote_group_id: remoteGroupID,
    name,
    platform: 'anthropic',
    subscription_type: '',
    rate_multiplier: 1,
    effective_rate_multiplier: null,
    comparable_rate: 1,
    peak_rate_enabled: false,
    peak_rate_multiplier: null,
    daily_limit_usd: null,
    weekly_limit_usd: null,
    monthly_limit_usd: null,
    synced_at: '2026-08-30T00:00:00Z'
  }
}

function mountDialog(props: Record<string, unknown>) {
  return mount(UpstreamProvisionDialog, {
    props: {
      show: false,
      provider: { id: 1, name: 'up-a' },
      groups: [],
      groupsLoading: false,
      localGroups: [],
      submitting: false,
      results: [],
      ...props
    },
    global: {
      stubs: { BaseDialog: BaseDialogStub }
    }
  })
}

describe('UpstreamProvisionDialog preset groups', () => {
  it('pre-selects the group handed over from the comparison table', async () => {
    const wrapper = mountDialog({
      groups: [makeGroup(7, 'cheap'), makeGroup(9, 'other')],
      presetRemoteGroupIDs: [7]
    })

    // Opening is what applies the preset — the dialog resets state on show.
    await wrapper.setProps({ show: true })

    const submit = wrapper.findAll('button').find((item) =>
      item.text().includes('admin.upstreamProviders.provisionSubmit')
    )
    expect(submit).toBeDefined()
    expect(submit!.attributes('disabled')).toBeUndefined()

    // jsdom does not turn a click on type="submit" into a form submit event,
    // so drive the form directly.
    await wrapper.find('form').trigger('submit')

    const emitted = wrapper.emitted('submit')
    expect(emitted).toHaveLength(1)
    expect(emitted![0][0]).toMatchObject({ remote_group_ids: [7] })
  })

  it('drops a preset group that the upstream no longer exposes', async () => {
    const wrapper = mountDialog({
      groups: [],
      presetRemoteGroupIDs: [7]
    })
    await wrapper.setProps({ show: true })

    // Snapshot arrives after the dialog opens; group 7 is gone upstream.
    await wrapper.setProps({ groups: [makeGroup(9, 'other')] })

    const submit = wrapper.findAll('button').find((item) =>
      item.text().includes('admin.upstreamProviders.provisionSubmit')
    )
    // Nothing selectable left, so submitting is blocked instead of sending a
    // request that would only come back as a per-group failure.
    expect(submit!.attributes('disabled')).toBeDefined()
  })

  it('selects nothing when opened from the provider list', async () => {
    const wrapper = mountDialog({ groups: [makeGroup(7, 'cheap')] })
    await wrapper.setProps({ show: true })

    const submit = wrapper.findAll('button').find((item) =>
      item.text().includes('admin.upstreamProviders.provisionSubmit')
    )
    expect(submit!.attributes('disabled')).toBeDefined()
  })
})
