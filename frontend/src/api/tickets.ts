import { apiClient } from './client'
import type { BasePaginationResponse, CreateTicketRequest, Ticket, TicketDetail } from '@/types'

export const ticketsAPI = {
  list: async (page = 1, pageSize = 20) =>
    (
      await apiClient.get<BasePaginationResponse<Ticket>>('/tickets', {
        params: { page, page_size: pageSize }
      })
    ).data,

  create: async (request: CreateTicketRequest) =>
    (await apiClient.post<TicketDetail>('/tickets', request)).data,

  getById: async (id: number) =>
    (await apiClient.get<TicketDetail>(`/tickets/${id}`)).data,

  addMessage: async (id: number, content: string) =>
    (await apiClient.post<TicketDetail>(`/tickets/${id}/messages`, { content })).data
}

export default ticketsAPI
