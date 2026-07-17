import { beforeEach, describe, expect, it, vi } from 'vitest'

import { apiClient } from '@/api/client'
import { downloadTicketAttachment, ticketAttachmentRequestPath } from '@/api/ticketAttachments'

vi.mock('@/api/client', () => ({
  apiClient: { get: vi.fn() }
}))

describe('ticket attachments API', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it.each([
    ['/api/v1/tickets/1/messages/2/attachments/0', '/tickets/1/messages/2/attachments/0'],
    ['/api/v1/admin/tickets/1/messages/2/attachments/3', '/admin/tickets/1/messages/2/attachments/3'],
    ['/tickets/9/messages/8/attachments/7', '/tickets/9/messages/8/attachments/7']
  ])('normalizes %s without duplicating the API prefix', (url, expected) => {
    expect(ticketAttachmentRequestPath(url)).toBe(expected)
  })

  it.each([
    '',
    '/api/v1/users/1',
    '/api/v1/tickets/1',
    'https://example.com/api/v1/tickets/1/messages/2/attachments/0'
  ])('rejects an invalid attachment URL: %s', (url) => {
    expect(() => ticketAttachmentRequestPath(url)).toThrow('Invalid ticket attachment URL')
  })

  it('downloads the image through the authenticated API client', async () => {
    const blob = new Blob(['image'], { type: 'image/png' })
    vi.mocked(apiClient.get).mockResolvedValue({ data: blob } as never)

    await expect(downloadTicketAttachment('/api/v1/tickets/1/messages/2/attachments/0')).resolves.toBe(blob)
    expect(apiClient.get).toHaveBeenCalledWith('/tickets/1/messages/2/attachments/0', {
      responseType: 'blob'
    })
  })

  it('rejects a non-image response', async () => {
    vi.mocked(apiClient.get).mockResolvedValue({
      data: new Blob(['error'], { type: 'application/json' })
    } as never)

    await expect(downloadTicketAttachment('/api/v1/tickets/1/messages/2/attachments/0')).rejects.toThrow(
      'Invalid ticket attachment response'
    )
  })
})
