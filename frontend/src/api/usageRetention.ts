import { apiClient } from './client'

export interface UsageRetentionSettings {
  enabled: boolean
  retention_days: number
}

export async function getUsageRetentionSettings(): Promise<UsageRetentionSettings> {
  const { data } = await apiClient.get<UsageRetentionSettings>('/usage/retention')
  return data
}

export async function getAdminUsageRetentionSettings(): Promise<UsageRetentionSettings> {
  const { data } = await apiClient.get<UsageRetentionSettings>('/admin/settings/usage-retention')
  return data
}

export async function saveUsageRetentionSettings(policy: UsageRetentionSettings): Promise<UsageRetentionSettings> {
  const { data } = await apiClient.put<UsageRetentionSettings>('/admin/settings/usage-retention', policy)
  return data
}
