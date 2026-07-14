import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type {
  CreateSupportTicketRequest,
  SupportTicketAttachmentPolicy,
  SupportTicket,
  SupportTicketFilters
} from '@/types/supportTicket'

export async function listTickets(
  page = 1,
  pageSize = 20,
  filters: SupportTicketFilters = {}
): Promise<BasePaginationResponse<SupportTicket>> {
  const { data } = await apiClient.get<BasePaginationResponse<SupportTicket>>('/tickets', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getTicket(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.get<SupportTicket>(`/tickets/${id}`)
  return data
}

function ticketFormData(request: CreateSupportTicketRequest | { content: string }, attachments: File[]): FormData {
  const form = new FormData()
  Object.entries(request).forEach(([key, value]) => form.append(key, value))
  attachments.forEach((file) => form.append('attachments', file))
  return form
}

export async function getAttachmentPolicy(): Promise<SupportTicketAttachmentPolicy> {
  const { data } = await apiClient.get<SupportTicketAttachmentPolicy>('/tickets/attachment-policy')
  return data
}

export async function downloadAttachment(ticketID: number, attachmentID: number): Promise<Blob> {
  const { data } = await apiClient.get<Blob>(`/tickets/${ticketID}/attachments/${attachmentID}`, {
    responseType: 'blob'
  })
  return data
}

export async function createTicket(request: CreateSupportTicketRequest, attachments: File[] = []): Promise<SupportTicket> {
  const body = attachments.length > 0 ? ticketFormData(request, attachments) : request
  const { data } = await apiClient.post<SupportTicket>('/tickets', body)
  return data
}

export async function replyTicket(id: number, content: string, attachments: File[] = []): Promise<SupportTicket> {
  const body = attachments.length > 0 ? ticketFormData({ content }, attachments) : { content }
  const { data } = await apiClient.post<SupportTicket>(`/tickets/${id}/messages`, body)
  return data
}

export async function closeTicket(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>(`/tickets/${id}/close`)
  return data
}

export async function reopenTicket(id: number): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>(`/tickets/${id}/reopen`)
  return data
}

export const ticketsAPI = {
  list: listTickets,
  attachmentPolicy: getAttachmentPolicy,
  downloadAttachment,
  get: getTicket,
  create: createTicket,
  reply: replyTicket,
  close: closeTicket,
  reopen: reopenTicket
}
