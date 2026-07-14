import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const settingsView = readFileSync(resolve(__dirname, '../admin/SettingsView.vue'), 'utf8')
const templateEditor = readFileSync(resolve(__dirname, '../admin/settings/EmailTemplateEditor.vue'), 'utf8')
const settingsAPI = readFileSync(resolve(__dirname, '../../api/admin/settings.ts'), 'utf8')

describe('support ticket email notification settings', () => {
  it('keeps the notification controls in the existing email settings section', () => {
    const quotaCard = settingsView.indexOf('<!-- Account Quota Notification -->')
    const ticketCard = settingsView.indexOf('<!-- Support Ticket Notification -->')
    const emailSectionEnd = settingsView.indexOf('<!-- /Tab: Email -->')

    expect(quotaCard).toBeGreaterThan(-1)
    expect(ticketCard).toBeGreaterThan(quotaCard)
    expect(emailSectionEnd).toBeGreaterThan(ticketCard)
    expect(settingsView).toContain('v-model="form.support_ticket_notify_enabled"')
    expect(settingsView).toContain('form.support_ticket_notify_emails')
  })

  it('exposes all four editable ticket email events', () => {
    for (const event of [
      'support_ticket.created',
      'support_ticket.user_reply',
      'support_ticket.admin_reply',
      'support_ticket.status_changed'
    ]) {
      expect(templateEditor).toContain(`"${event}"`)
    }
  })

  it('includes ticket notification fields in the admin settings contract', () => {
    expect(settingsAPI).toContain('support_ticket_notify_enabled')
    expect(settingsAPI).toContain('support_ticket_notify_emails')
  })
})
