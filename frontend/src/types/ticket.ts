export type TicketStatus = 'open' | 'in_progress' | 'waiting_user' | 'closed'
export type TicketPriority = 'low' | 'normal' | 'high' | 'urgent'
export type TicketCategory = 'billing' | 'technical' | 'account' | 'suggestion' | 'other'

export interface TicketAttachment {
  name: string
  content_type: string
  size: number
  url: string
}

export interface TicketMessage {
  id: number
  author_id: number
  author_role: 'user' | 'admin'
  author_name: string
  content: string
  attachments: TicketAttachment[]
  created_at: string
}

export interface Ticket {
  id: number
  user_id: number
  user_email?: string
  username?: string
  subject: string
  category: TicketCategory
  status: TicketStatus
  priority: TicketPriority
  last_message_at: string
  closed_at?: string
  created_at: string
  updated_at: string
  messages?: TicketMessage[]
}

export interface TicketStorageConfig {
  enabled: boolean
  endpoint: string
  region: string
  bucket: string
  access_key_id: string
  secret_access_key?: string
  has_secret?: boolean
  prefix: string
  force_path_style: boolean
  max_file_size_mb: number
  max_files_per_message: number
}

export interface TicketFormPayload {
  subject?: string
  category?: TicketCategory
  content: string
  images: File[]
}
