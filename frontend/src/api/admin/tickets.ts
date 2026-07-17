import { apiClient } from '../client'
import type { BasePaginationResponse } from '@/types'
import type { Ticket, TicketFormPayload, TicketPriority, TicketStatus, TicketStorageConfig } from '@/types/ticket'

function replyForm(payload: TicketFormPayload): FormData {
  const form = new FormData()
  form.append('content', payload.content)
  payload.images.forEach((image) => form.append('images', image))
  return form
}

export async function listTickets(
  page = 1,
  pageSize = 20,
  filters: { status?: string; priority?: string; category?: string; search?: string } = {}
): Promise<BasePaginationResponse<Ticket>> {
  const { data } = await apiClient.get<BasePaginationResponse<Ticket>>('/admin/tickets', {
    params: { page, page_size: pageSize, ...filters }
  })
  return data
}

export async function getTicket(id: number): Promise<Ticket> {
  const { data } = await apiClient.get<Ticket>(`/admin/tickets/${id}`)
  return data
}

export async function replyTicket(id: number, payload: TicketFormPayload): Promise<Ticket> {
  const { data } = await apiClient.post<Ticket>(`/admin/tickets/${id}/messages`, replyForm(payload), {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data
}

export async function updateTicket(id: number, payload: { status?: TicketStatus; priority?: TicketPriority }): Promise<Ticket> {
  const { data } = await apiClient.put<Ticket>(`/admin/tickets/${id}`, payload)
  return data
}

export async function getStorageConfig(): Promise<TicketStorageConfig> {
  const { data } = await apiClient.get<TicketStorageConfig>('/admin/settings/ticket-storage')
  return data
}

export async function updateStorageConfig(config: TicketStorageConfig): Promise<TicketStorageConfig> {
  const { data } = await apiClient.put<TicketStorageConfig>('/admin/settings/ticket-storage', config)
  return data
}

export async function testStorage(config: TicketStorageConfig): Promise<{ ok: boolean; message: string }> {
  const { data } = await apiClient.post<{ ok: boolean; message: string }>('/admin/settings/ticket-storage/test', config)
  return data
}

export const ticketsAPI = {
  list: listTickets,
  get: getTicket,
  reply: replyTicket,
  update: updateTicket,
  getStorageConfig,
  updateStorageConfig,
  testStorage
}

export default ticketsAPI
