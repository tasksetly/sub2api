import { apiClient } from './client'

export interface CCTestSettings {
  enabled: boolean
  allow_users: boolean
  public_url: string
  has_api_key: boolean
  encryption_key_configured: boolean
  version: string
}

export interface CCTestResult {
  progress?: number
  stepName?: string
  expectedModel?: string
  responseModel?: string
  total?: number
  verdictKey?: string
  scores?: Record<string, number>
  streamChannel?: string | null
  nonStreamChannel?: string | null
  metrics?: { inputTokens?: number; outputTokens?: number; latencyMs?: number }
  tokenAudit?: { totalCost?: number; baselineTotalCost?: number; overallRatio?: number; cacheHitRate?: number }
}

export interface CCTestRun {
  id: string
  account_id?: number
  api_key_id?: number
  target_name: string
  model: string
  token_audit: boolean
  status: 'queued' | 'starting' | 'running' | 'done' | 'error' | 'cancelled'
  request_count: number
  result: CCTestResult
  error: string
  created_at: string
  expires_at: string
}

const prefix = (admin: boolean) => admin ? '/admin/cctest' : '/cctest'

export const cctestAPI = {
  async settings() {
    return (await apiClient.get<CCTestSettings>('/admin/cctest/settings')).data
  },
  async saveSettings(settings: CCTestSettings, apiKey: string) {
    return (await apiClient.put<CCTestSettings>('/admin/cctest/settings', {
      enabled: settings.enabled, allow_users: settings.allow_users,
      public_url: settings.public_url, version: settings.version, api_key: apiKey
    })).data
  },
  async availability() {
    return (await apiClient.get<{ enabled: boolean }>('/cctest/availability')).data
  },
  async list(admin: boolean, before = '', signal?: AbortSignal) {
    return (await apiClient.get<CCTestRun[]>(`${prefix(admin)}/runs`, { params: { before }, signal })).data
  },
  async create(admin: boolean, input: { account_ids?: number[]; api_key_id?: number; model: string; token_audit: boolean }) {
    return (await apiClient.post<CCTestRun[]>(`${prefix(admin)}/runs`, input)).data
  },
  async cancel(admin: boolean, id: string) {
    await apiClient.delete(`${prefix(admin)}/runs/${id}`)
  }
}
