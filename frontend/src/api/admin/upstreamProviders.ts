/**
 * Admin Upstream Providers API endpoints
 *
 * 上游 sub2api 供应商管理：存后台凭据 → 登录/续期 → 拉分组倍率/余额/并发
 * → 直接在上游建 API Key 并落地成本地账号。
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type {
  UpstreamProvider,
  UpstreamProviderWithStats,
  UpstreamGroup,
  UpstreamGroupComparison,
  UpstreamProfile,
  CreateUpstreamProviderRequest,
  UpdateUpstreamProviderRequest,
  ProvisionAccountsRequest,
  ProvisionAccountsResponse,
  SyncAllResult
} from '@/types/upstreamProvider'

/**
 * 列出上游供应商（含余额、并发、分组倍率区间、本地实际成本）
 */
export async function list(
  page: number = 1,
  pageSize: number = 20,
  filters?: {
    status?: 'active' | 'inactive'
    search?: string
  },
  options?: {
    signal?: AbortSignal
  }
): Promise<PaginatedResponse<UpstreamProviderWithStats>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamProviderWithStats>>(
    '/admin/upstream-providers',
    {
      params: {
        page,
        page_size: pageSize,
        ...filters
      },
      signal: options?.signal
    }
  )
  return data
}

export async function getByID(id: number): Promise<UpstreamProvider> {
  const { data } = await apiClient.get<UpstreamProvider>(`/admin/upstream-providers/${id}`)
  return data
}

export async function create(
  payload: CreateUpstreamProviderRequest
): Promise<UpstreamProvider> {
  const { data } = await apiClient.post<UpstreamProvider>('/admin/upstream-providers', payload)
  return data
}

export async function update(
  id: number,
  payload: UpdateUpstreamProviderRequest
): Promise<UpstreamProvider> {
  const { data } = await apiClient.put<UpstreamProvider>(
    `/admin/upstream-providers/${id}`,
    payload
  )
  return data
}

export async function remove(id: number): Promise<void> {
  await apiClient.delete(`/admin/upstream-providers/${id}`)
}

/**
 * 测试连接：验证当前保存的上游会话或登录凭据能否访问上游，不落任何快照
 */
export async function testConnection(id: number): Promise<UpstreamProfile> {
  const { data } = await apiClient.post<UpstreamProfile>(
    `/admin/upstream-providers/${id}/test`
  )
  return data
}

/**
 * 手动刷新单个上游的账户信息（余额、并发）与分组倍率
 */
export async function sync(id: number): Promise<UpstreamProvider> {
  const { data } = await apiClient.post<UpstreamProvider>(
    `/admin/upstream-providers/${id}/sync`
  )
  return data
}

/**
 * 手动刷新全部启用了同步的上游。单个失败不影响其余。
 */
export async function syncAll(): Promise<SyncAllResult> {
  const { data } = await apiClient.post<SyncAllResult>('/admin/upstream-providers/sync-all')
  return data
}

/**
 * 该上游同步下来的分组快照（含倍率与限额），供勾选建号
 */
export async function listGroups(id: number): Promise<UpstreamGroup[]> {
  const { data } = await apiClient.get<UpstreamGroup[]>(
    `/admin/upstream-providers/${id}/groups`
  )
  return data
}

/**
 * 跨上游拉平所有分组做横向比价，按「修正后倍率」升序。
 *
 * 后端返回分页信封（items/total/...），不是裸数组——直接当数组用会渲染出空表。
 * @param platform 可选，按平台过滤（跨平台比倍率没意义）
 */
export async function compareGroups(
  platform?: string,
  page: number = 1,
  pageSize: number = 50
): Promise<PaginatedResponse<UpstreamGroupComparison>> {
  const { data } = await apiClient.get<PaginatedResponse<UpstreamGroupComparison>>(
    '/admin/upstream-providers/groups/compare',
    {
      params: {
        page,
        page_size: pageSize,
        ...(platform ? { platform } : {})
      }
    }
  )
  return data
}

/**
 * 对勾选的上游分组创建 API Key 并落地本地账号
 */
export async function provisionAccounts(
  id: number,
  payload: ProvisionAccountsRequest
): Promise<ProvisionAccountsResponse> {
  const { data } = await apiClient.post<ProvisionAccountsResponse>(
    `/admin/upstream-providers/${id}/provision`,
    payload
  )
  return data
}

export const upstreamProvidersAPI = {
  list,
  getByID,
  create,
  update,
  remove,
  testConnection,
  sync,
  syncAll,
  listGroups,
  compareGroups,
  provisionAccounts
}

export default upstreamProvidersAPI
