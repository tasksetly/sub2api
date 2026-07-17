import { apiClient } from './client'

const API_PREFIX = '/api/v1'
const ATTACHMENT_PATH = /^\/(?:admin\/)?tickets\/\d+\/messages\/\d+\/attachments\/\d+$/

export function ticketAttachmentRequestPath(url: string): string {
  let path = String(url || '').trim()
  if (path.startsWith(`${API_PREFIX}/`)) {
    path = path.slice(API_PREFIX.length)
  }
  if (!ATTACHMENT_PATH.test(path)) {
    throw new Error('Invalid ticket attachment URL')
  }
  return path
}

export async function downloadTicketAttachment(url: string): Promise<Blob> {
  const path = ticketAttachmentRequestPath(url)
  const response = await apiClient.get<Blob>(path, { responseType: 'blob' })
  const blob = response.data
  if (!(blob instanceof Blob) || !blob.type.toLowerCase().startsWith('image/')) {
    throw new Error('Invalid ticket attachment response')
  }
  return blob
}
