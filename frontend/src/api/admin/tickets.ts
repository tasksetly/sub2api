import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type {
  SupportTicket,
  SupportTicketAttachmentPolicy,
  SupportTicketFilters,
  SupportTicketPriority,
  SupportTicketStatus
} from '@/types/supportTicket'

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
  get: getTicket,
  reply: replyTicket,
  update: updateTicket
}
