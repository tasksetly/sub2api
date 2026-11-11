import { apiClient } from '../client'

export interface IntelligenceTestConfig {
  answer_mode?: 'svg' | 'candy'
  id: number
  name: string
  model_id: string
  model_name: string
  reasoning_effort: string
  prompt_text: string
  api_key_id: number
  api_key_name?: string
  group_id?: number | null
  group_name?: string
  cron_expression: string
  keep_count: number
  enabled: boolean
  last_run_at?: string | null
  next_run_at?: string | null
  updated_at?: string
}

export interface IntelligenceTestAPIKeyOption {
  id: number
  name: string
  key_prefix: string
  status: string
  group_id: number
  group_name: string
}

export interface IntelligenceTestResult {
  answer_mode?: 'svg' | 'candy'
  response_text?: string
  account_id?: number
  account_name?: string
  prompt_text?: string
  svg_content?: string
  id: number
  config_id: number
  group_name: string
  model_id: string
  model_name: string
  effort?: string
  status: string
  check: boolean
  latency_ms: number
  error?: string
  created_at: string
}

export interface IntelligenceTestRunStatus {
  running: boolean
  status: string
  started_at?: string
  finished_at?: string
  result_id?: number
  error?: string
}

export interface IntelligenceTestConfigInput {
  answer_mode?: 'svg' | 'candy'
  name: string
  model_id: string
  model_name: string
  reasoning_effort: string
  prompt_text: string
  api_key_id: number
  cron_expression: string
  keep_count: number
  enabled: boolean
}

export async function listConfigs() {
  const { data } = await apiClient.get<IntelligenceTestConfig[]>('/admin/intelligence-test/configs')
  return data ?? []
}

export async function listAPIKeys() {
  const { data } = await apiClient.get<IntelligenceTestAPIKeyOption[]>('/admin/intelligence-test/api-keys')
  return data ?? []
}

export async function create(input: IntelligenceTestConfigInput) {
  const { data } = await apiClient.post<IntelligenceTestConfig>('/admin/intelligence-test/configs', input)
  return data
}

export async function update(id: number, input: IntelligenceTestConfigInput) {
  const { data } = await apiClient.put<IntelligenceTestConfig>(`/admin/intelligence-test/configs/${id}`, input)
  return data
}

export async function remove(id: number) {
  await apiClient.delete(`/admin/intelligence-test/configs/${id}`)
}

export async function runNow(id: number) {
  const { data } = await apiClient.post<IntelligenceTestRunStatus>(`/admin/intelligence-test/configs/${id}/run`)
  return data
}

export async function getRunStatus(id: number) {
  const { data } = await apiClient.get<IntelligenceTestRunStatus>(`/admin/intelligence-test/configs/${id}/run-status`)
  return data
}

export async function listResults(id: number, limit = 20) {
  const { data } = await apiClient.get<IntelligenceTestResult[]>(`/admin/intelligence-test/configs/${id}/results`, { params: { limit } })
  return data ?? []
}

export interface AccountIntelligenceTestInput {
  model_id: string
  prompt_text: string
  reasoning_effort: string
}

export async function runAccount(accountId: number, input: AccountIntelligenceTestInput) {
  const { data } = await apiClient.post<IntelligenceTestResult>(`/admin/accounts/${accountId}/intelligence-tests`, input)
  return data
}

export async function listAccountResults(accountId: number, beforeId = 0, signal?: AbortSignal) {
  const { data } = await apiClient.get<IntelligenceTestResult[]>(`/admin/accounts/${accountId}/intelligence-tests`, {
    params: { before_id: beforeId }, signal
  })
  return data ?? []
}

export const intelligenceTestAPI = { runAccount, listAccountResults, listConfigs, listAPIKeys, create, update, remove, runNow, getRunStatus, listResults }
export default intelligenceTestAPI
