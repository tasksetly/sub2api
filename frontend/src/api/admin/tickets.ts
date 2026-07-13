import { apiClient } from '../client'
import type {
  BasePaginationResponse,
  Ticket,
  TicketCategory,
  TicketDetail,
  TicketPriority,
  TicketStatus
} from '@/types'

export interface AdminTicketFilters {
  status?: TicketStatus
  category?: TicketCategory
  priority?: TicketPriority
  search?: string
}

export interface UpdateTicketRequest {
  status?: TicketStatus
  priority?: TicketPriority
}

export const ticketsAPI = {
  list: async (page = 1, pageSize = 20, filters?: AdminTicketFilters) =>
    (
      await apiClient.get<BasePaginationResponse<Ticket>>('/admin/tickets', {
        params: { page, page_size: pageSize, ...filters }
      })
    ).data,

  getById: async (id: number) =>
    (await apiClient.get<TicketDetail>(`/admin/tickets/${id}`)).data,

  addMessage: async (id: number, content: string) =>
    (await apiClient.post<TicketDetail>(`/admin/tickets/${id}/messages`, { content })).data,

  update: async (id: number, request: UpdateTicketRequest) =>
    (await apiClient.patch<Ticket>(`/admin/tickets/${id}`, request)).data
}

export default ticketsAPI
