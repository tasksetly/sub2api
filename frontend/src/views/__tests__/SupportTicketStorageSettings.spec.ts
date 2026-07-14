import { beforeEach, describe, expect, it, vi } from 'vitest'
import { flushPromises, mount } from '@vue/test-utils'
import SupportTicketStorageSettings from '../admin/settings/SupportTicketStorageSettings.vue'

const {
  getConfig,
  updateConfig,
  testConnection,
  showSuccess,
  showError
} = vi.hoisted(() => ({
  getConfig: vi.fn(),
  updateConfig: vi.fn(),
  testConnection: vi.fn(),
  showSuccess: vi.fn(),
  showError: vi.fn()
}))

vi.mock('@/api/admin', () => ({
  adminAPI: {
    tickets: {
      attachmentStorage: getConfig,
      updateAttachmentStorage: updateConfig,
      testAttachmentStorage: testConnection
    }
  }
}))

vi.mock('@/stores', () => ({
  useAppStore: () => ({ showSuccess, showError })
}))

vi.mock('vue-i18n', async () => {
  const actual = await vi.importActual<typeof import('vue-i18n')>('vue-i18n')
  return {
    ...actual,
    useI18n: () => ({ t: (key: string) => key })
  }
})

const configuredStorage = {
  enabled: true,
  endpoint: 'https://account.r2.cloudflarestorage.com',
  region: 'auto',
  bucket: 'tickets',
  access_key_id: 'access-id',
  secret_access_key: '',
  secret_configured: true,
  prefix: 'support-tickets',
  force_path_style: false,
  max_file_size_mb: 10,
  max_attachments_per_message: 4,
  url_expiry_minutes: 15
}

const ToggleStub = {
  props: ['modelValue'],
  emits: ['update:modelValue'],
  template: '<button type="button" @click="$emit(\'update:modelValue\', !modelValue)">toggle</button>'
}

describe('SupportTicketStorageSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    getConfig.mockResolvedValue({ ...configuredStorage })
    updateConfig.mockResolvedValue({ ...configuredStorage })
    testConnection.mockResolvedValue({ ok: true, message: 'ok' })
  })

  it('loads a masked secret and saves through the dedicated ticket API', async () => {
    const wrapper = mount(SupportTicketStorageSettings, {
      global: {
        stubs: {
          Icon: true,
          Toggle: ToggleStub
        }
      }
    })
    await flushPromises()

    const secret = wrapper.get<HTMLInputElement>('#ticket-storage-secret-key')
    expect(secret.element.value).toBe('')
    expect(secret.attributes('placeholder')).toContain('secretConfigured')

    await wrapper.get<HTMLInputElement>('#ticket-storage-prefix').setValue('ticket-images')
    await wrapper.get('[data-testid="ticket-storage-save"]').trigger('click')
    await flushPromises()

    expect(updateConfig).toHaveBeenCalledWith(
      expect.objectContaining({
        prefix: 'ticket-images',
        secret_access_key: ''
      })
    )
    expect(showSuccess).toHaveBeenCalled()
  })

  it('tests the current form without persisting it', async () => {
    const wrapper = mount(SupportTicketStorageSettings, {
      global: {
        stubs: {
          Icon: true,
          Toggle: ToggleStub
        }
      }
    })
    await flushPromises()

    await wrapper.get<HTMLInputElement>('#ticket-storage-bucket').setValue('new-bucket')
    await wrapper.get('[data-testid="ticket-storage-test"]').trigger('click')
    await flushPromises()

    expect(testConnection).toHaveBeenCalledWith(
      expect.objectContaining({ bucket: 'new-bucket' })
    )
    expect(updateConfig).not.toHaveBeenCalled()
    expect(showSuccess).toHaveBeenCalled()
  })
})
