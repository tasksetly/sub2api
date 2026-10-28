import { apiClient } from './client'

export interface IntelligenceTestConfig {
  title?: string
  description?: string
  prompt_name?: string
  prompt_text?: string
  question_text?: string
  question_id?: string | number
  updated_at?: string
  asset_base_url?: string
  keep_count?: number
}

export interface IntelligenceTestItem {
  id?: string | number
  group?: string
  group_name?: string
  group_id?: string | number
  model?: string
  model_id?: string
  effort?: string | null
  status?: string
  ok?: boolean
  check?: boolean
  svg?: string
  svg_url?: string
  svg_content?: string
  image_url?: string
  ts?: string
  created_at?: string
  updated_at?: string
  latency_ms?: number
  ttft_ms?: number
  error?: string | null
  prompt_name?: string
  question_id?: string | number
}

export interface IntelligenceTestRun {
  ts?: string
  created_at?: string
  items?: IntelligenceTestItem[]
}

export interface IntelligenceTestResponse {
  config?: IntelligenceTestConfig
  title?: string
  description?: string
  prompt_name?: string
  prompt_text?: string
  question_text?: string
  question_id?: string | number
  asset_base_url?: string
  updated?: string
  next?: string
  keep_count?: number
  groups?: Array<string | { id?: string | number; name?: string }>
  items?: IntelligenceTestItem[]
  runs?: IntelligenceTestRun[]
}

export interface IntelligenceTestQuery {
  user_id?: string
  token?: string
  question_id?: string
  prompt_id?: string
}

const ENDPOINTS = ['/intelligence-test/history', '/pelican-test/history', '/intelligence-test']

function getQueryParams(query: IntelligenceTestQuery) {
  return Object.fromEntries(
    Object.entries(query).filter(([, value]) => typeof value === 'string' && value.trim()),
  )
}

/**
 * Loads configured prompt metadata and recent SVG results. API keys stay in backend
 * configuration; the page receives the already-resolved group name with each item.
 */
export async function getIntelligenceTest(
  query: IntelligenceTestQuery = {},
  options: { signal?: AbortSignal } = {},
): Promise<IntelligenceTestResponse> {
  let lastError: unknown
  const params = getQueryParams(query)

  for (const endpoint of ENDPOINTS) {
    try {
      const { data } = await apiClient.get<IntelligenceTestResponse>(endpoint, {
        params,
        signal: options.signal,
      })
      return data
    } catch (error) {
      lastError = error
    }
  }

  try {
    const response = await fetch(`/pelican-test/data/history.json?t=${Date.now()}`, {
      cache: 'no-store',
      signal: options.signal,
    })
    if (response.ok) return await response.json() as IntelligenceTestResponse
  } catch (error) {
    lastError = error
  }

  throw lastError || new Error('Unable to load intelligence test results')
}

export const intelligenceTestAPI = { getIntelligenceTest }

export default intelligenceTestAPI
