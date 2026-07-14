import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type {
  SupportTicket,
  SupportTicketAttachmentPolicy,
  SupportTicketFilters,
  SupportTicketPriority,
  SupportTicketStatus
} from '@/types/supportTicket'

export interface SupportTicketAttachmentStorageConfig {
  enabled: boolean
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key: string
  secret_configured: boolean
  prefix: string
  force_path_style: boolean
  max_file_size_mb: number
  max_attachments_per_message: number
  url_expiry_minutes: number
}

export interface SupportTicketAttachmentStorageTestResult {
  ok: boolean
  message: string
}

export async function listTickets(
  page = 1,
  pageSize = 20,
  filters: SupportTicketFilters = {}
): Promise<BasePaginationResponse<SupportTicket>> {
  const { data } = await apiClient.get<BasePaginationResponse<SupportTicket>>('/admin/tickets', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getTicket(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.get<SupportTicket>(`/admin/tickets/${id}`)
  return data
}

export async function getAttachmentPolicy(): Promise<SupportTicketAttachmentPolicy> {
  const { data } = await apiClient.get<SupportTicketAttachmentPolicy>('/admin/tickets/attachment-policy')
  return data
}

export async function getAttachmentStorageConfig(): Promise<SupportTicketAttachmentStorageConfig> {
  const { data } = await apiClient.get<SupportTicketAttachmentStorageConfig>('/admin/tickets/attachment-storage')
  return data
}

export async function updateAttachmentStorageConfig(
  config: SupportTicketAttachmentStorageConfig
): Promise<SupportTicketAttachmentStorageConfig> {
  const { data } = await apiClient.put<SupportTicketAttachmentStorageConfig>(
    '/admin/tickets/attachment-storage',
    config
  )
  return data
}

export async function testAttachmentStorage(
  config: SupportTicketAttachmentStorageConfig
): Promise<SupportTicketAttachmentStorageTestResult> {
  const { data } = await apiClient.post<SupportTicketAttachmentStorageTestResult>(
    '/admin/tickets/attachment-storage/test',
    config
  )
  return data
}

export async function downloadAttachment(ticketID: number, attachmentID: number): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/admin/tickets/${ticketID}/attachments/${attachmentID}`, {
    responseType: 'blob'
  })
  return data
}

export async function replyTicket(id: number, content: string, attachments: File[] = []): Promise<SupportTicket> {
  let body: FormData | { content: string } = { content }
  if (attachments.length > 0) {
    const form = new FormData()
    form.append('content', content)
    attachments.forEach((file) => form.append('attachments', file))
    body = form
  }
  const { data } = await apiClient.post<SupportTicket>(`/admin/tickets/${id}/messages`, body)
  return data
}

export async function updateTicket(
  id: number,
  update: { status?: SupportTicketStatus; priority?: SupportTicketPriority }
): Promise<SupportTicket> {
  const { data } = await apiClient.put<SupportTicket>(`/admin/tickets/${id}`, update)
  return data
}

export default {
  list: listTickets,
  attachmentPolicy: getAttachmentPolicy,
  attachmentStorage: getAttachmentStorageConfig,
  updateAttachmentStorage: updateAttachmentStorageConfig,
  testAttachmentStorage,
  downloadAttachment,
  get: getTicket,
  reply: replyTicket,
  update: updateTicket
}
