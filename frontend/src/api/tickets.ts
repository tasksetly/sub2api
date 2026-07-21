import { apiClient } from './client'
import type { BasePaginationResponse } from '@/types'
import type { Ticket, TicketFormPayload, TicketStatus } from '@/types/ticket'

function toFormData(payload: TicketFormPayload): FormData {
  const form = new FormData()
  if (payload.subject) form.append('subject', payload.subject)
  if (payload.category) form.append('category', payload.category)
  form.append('content', payload.content)
  payload.images.forEach((image) => form.append('images', image))
  return form
}

export async function listTickets(page = 1, pageSize = 20, status?: TicketStatus | ''): Promise<BasePaginationResponse<Ticket>> {
  const { data } = await apiClient.get<BasePaginationResponse<Ticket>>('/tickets', {
    params: { page, page_size: pageSize, status: status || undefined }
  })
  return data
}

export async function getTicket(id: number): Promise<Ticket> {
  const { data } = await apiClient.get<Ticket>(`/tickets/${id}`)
  return data
}

export async function createTicket(payload: TicketFormPayload): Promise<Ticket> {
  const { data } = await apiClient.post<Ticket>('/tickets', toFormData(payload), {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data
}

export async function replyTicket(id: number, payload: TicketFormPayload): Promise<Ticket> {
  const { data } = await apiClient.post<Ticket>(`/tickets/${id}/messages`, toFormData(payload), {
    headers: { 'Content-Type': 'multipart/form-data' }
  })
  return data
}

export async function closeTicket(id: number): Promise<Ticket> {
  const { data } = await apiClient.post<Ticket>(`/tickets/${id}/close`)
  return data
}

export async function getWaitingUserCount(): Promise<number> {
  const result = await listTickets(1, 1, 'waiting_user')
  return result.total
}

export const ticketsAPI = {
  list: listTickets,
  get: getTicket,
  create: createTicket,
  reply: replyTicket,
  close: closeTicket,
  getWaitingUserCount
}
export default ticketsAPI
