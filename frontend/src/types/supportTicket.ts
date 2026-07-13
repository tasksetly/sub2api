export type SupportTicketStatus = 'open' | 'in_progress' | 'waiting_user' | 'resolved' | 'closed'
export type SupportTicketPriority = 'low' | 'normal' | 'high' | 'urgent'
export type SupportTicketCategory = 'technical' | 'billing' | 'account' | 'other'
export type SupportTicketSenderRole = 'user' | 'admin'

export interface SupportTicketAttachment {
  id: number
  ticket_id: number
  message_id: number
  uploader_id: number
  file_name: string
  content_type: string
  size_bytes: number
  url?: string
  created_at: string
}

export interface SupportTicketAttachmentPolicy {
  enabled: boolean
  max_file_size_bytes: number
  max_attachments_per_message: number
}

export interface SupportTicketMessage {
  id: number
  ticket_id: number
  sender_id: number
  sender_role: SupportTicketSenderRole
  content: string
  created_at: string
  attachments?: SupportTicketAttachment[]
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
