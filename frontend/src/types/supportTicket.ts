export type SupportTicketStatus = 'open' | 'in_progress' | 'waiting_user' | 'resolved' | 'closed'
export type SupportTicketPriority = 'low' | 'normal' | 'high' | 'urgent'
export type SupportTicketCategory = 'technical' | 'billing' | 'account' | 'other'
export type SupportTicketSenderRole = 'user' | 'admin'

export interface SupportTicketMessage {
  id: number
  ticket_id: number
  sender_id: number
  sender_role: SupportTicketSenderRole
  content: string
  created_at: string
}

export interface SupportTicket {
  id: number
  user_id: number
  user_email?: string
  username?: string
  subject: string
  category: SupportTicketCategory
  priority: SupportTicketPriority
  status: SupportTicketStatus
  admin_unread: boolean
  user_unread: boolean
  last_message_at: string
  closed_at?: string
  created_at: string
  updated_at: string
  messages?: SupportTicketMessage[]
}

export interface SupportTicketFilters {
  status?: SupportTicketStatus | ''
  category?: SupportTicketCategory | ''
  priority?: SupportTicketPriority | ''
  search?: string
}

export interface CreateSupportTicketRequest {
  subject: string
  category: SupportTicketCategory
  priority: SupportTicketPriority
  content: string
}
