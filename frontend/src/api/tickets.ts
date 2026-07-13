import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type {
  CreateSupportTicketRequest,
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

export async function createTicket(request: CreateSupportTicketRequest): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>('/tickets', request)
  return data
}

export async function replyTicket(id: number, content: string): Promise<SupportTicket> {
  const { data } = await apiClient.post<SupportTicket>(`/tickets/${id}/messages`, { content })
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
  get: getTicket,
  create: createTicket,
  reply: replyTicket,
  close: closeTicket,
  reopen: reopenTicket
}
