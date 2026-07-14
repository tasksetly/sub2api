import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const userView = readFileSync(resolve(__dirname, '../user/TicketsView.vue'), 'utf8')
const adminView = readFileSync(resolve(__dirname, '../admin/TicketsView.vue'), 'utf8')
const workspace = readFileSync(resolve(__dirname, '../../components/tickets/TicketWorkspace.vue'), 'utf8')

describe('ticket page layout integration', () => {
  it('renders both ticket routes inside the shared application layout', () => {
    expect(userView).toContain('<AppLayout>')
    expect(adminView).toContain('<AppLayout>')
    expect(userView).toContain("import AppLayout from '@/components/layout/AppLayout.vue'")
    expect(adminView).toContain("import AppLayout from '@/components/layout/AppLayout.vue'")
  })

  it('uses the existing form, button, card, dialog, and icon primitives', () => {
    expect(workspace).not.toContain('input-field')
    expect(workspace).toContain('class="input')
    expect(workspace).toContain('class="btn btn-primary')
    expect(workspace).toContain('class="card')
    expect(workspace).toContain('<BaseDialog')
    expect(workspace).toContain('<Icon')
  })

  it('keeps image attachments inside the shared ticket workspace controls', () => {
    expect(workspace).toContain('accept="image/jpeg,image/png,image/gif,image/webp"')
    expect(workspace).toContain('name="paperclip"')
    expect(workspace).toContain('message.attachments')
    expect(workspace).toContain('attachmentPolicy.enabled')
  })

  it('loads attachment previews through the authenticated ticket API and exposes downloads', () => {
    expect(workspace).toContain('loadAttachmentPreviews(ticket)')
    expect(workspace).toContain('downloadTicketAttachment(attachment)')
    expect(workspace).toContain('name="download"')
  })
})
