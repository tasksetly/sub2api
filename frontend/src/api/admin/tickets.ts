import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type {
  SupportTicket,
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

export async function replyTicket(id: number, content: string): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>(`/admin/tickets/${id}/messages`, { content })
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
  get: getTicket,
  reply: replyTicket,
  update: updateTicket
}
