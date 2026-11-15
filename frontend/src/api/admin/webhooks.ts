import { apiClient } from '../client'

export interface NotificationWebhookSettings {
  enabled: boolean
  webhook_url: string
  has_secret: boolean
  version: string
  encryption_key_configured: boolean
}

export interface UpdateNotificationWebhookSettings {
  enabled: boolean
  webhook_url: string
  secret: string
  clear_secret: boolean
  version: string
}

export interface NotificationWebhookDelivery {
  id: number
  event_id: string
  event_type: string
  order_id: number
  status: 'pending' | 'delivering' | 'delivered' | 'failed' | 'skipped'
  attempts: number
  last_status_code: number
  last_error: string
  created_at: string
  finished_at: string | null
}

export async function getNotificationWebhookSettings(): Promise<NotificationWebhookSettings> {
  const { data } = await apiClient.get<NotificationWebhookSettings>('/admin/settings/notification-webhook')
  return data
}

export async function updateNotificationWebhookSettings(input: UpdateNotificationWebhookSettings): Promise<NotificationWebhookSettings> {
  const { data } = await apiClient.put<NotificationWebhookSettings>('/admin/settings/notification-webhook', input)
  return data
}

export async function listNotificationWebhookDeliveries(): Promise<NotificationWebhookDelivery[]> {
  const { data } = await apiClient.get<NotificationWebhookDelivery[]>('/admin/settings/notification-webhook/deliveries')
  return data
}

export async function testNotificationWebhook(): Promise<void> {
  await apiClient.post('/admin/settings/notification-webhook/test')
}
