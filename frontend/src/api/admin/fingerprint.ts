/**
 * Admin Fingerprint API endpoints
 * 账号指纹校对：用 LM Fingerpoint Detector 核对上游账号实际回的是不是声称的模型
 */

import { apiClient } from '../client'
import type {
  FingerprintPlan,
  FingerprintResult,
  FingerprintRefModel,
  FingerprintOverview,
  FingerprintPlanRequest
} from '@/types'

/**
 * 参考库里的全部模型，供预期模型下拉使用。
 * 预期模型只能从这个清单里选——库外模型会被硬配候选，导致每轮都判不符。
 */
export async function referenceModels(): Promise<FingerprintRefModel[]> {
  const { data } = await apiClient.get<FingerprintRefModel[]>('/admin/fingerprint/reference-models')
  return data ?? []
}

/** 账号的指纹校对概览：资格、代理绕过标记、本月消耗、参考库生成时间。 */
export async function overview(accountId: number): Promise<FingerprintOverview> {
  const { data } = await apiClient.get<FingerprintOverview>(
    `/admin/accounts/${accountId}/fingerprint-overview`
  )
  return data
}

/** 列出某账号的全部指纹计划。 */
export async function listByAccount(accountId: number): Promise<FingerprintPlan[]> {
  const { data } = await apiClient.get<FingerprintPlan[]>(
    `/admin/accounts/${accountId}/fingerprint-plans`
  )
  return data ?? []
}

/** 新建指纹计划。 */
export async function create(req: FingerprintPlanRequest): Promise<FingerprintPlan> {
  const { data } = await apiClient.post<FingerprintPlan>('/admin/fingerprint-plans', req)
  return data
}

/** 更新指纹计划。 */
export async function update(id: number, req: FingerprintPlanRequest): Promise<FingerprintPlan> {
  const { data } = await apiClient.put<FingerprintPlan>(`/admin/fingerprint-plans/${id}`, req)
  return data
}

/** 删除指纹计划。 */
export async function deletePlan(id: number): Promise<void> {
  await apiClient.delete(`/admin/fingerprint-plans/${id}`)
}

/** 列出某计划的最近检测结果。 */
export async function listResults(planId: number, limit?: number): Promise<FingerprintResult[]> {
  const { data } = await apiClient.get<FingerprintResult[]>(
    `/admin/fingerprint-plans/${planId}/results`,
    { params: limit ? { limit } : undefined }
  )
  return data ?? []
}

/** 立即跑一次。检测在后台异步执行，接口只回执行中。 */
export async function runNow(planId: number): Promise<void> {
  await apiClient.post(`/admin/fingerprint-plans/${planId}/run`)
}

export const fingerprintAPI = {
  referenceModels,
  overview,
  listByAccount,
  create,
  update,
  delete: deletePlan,
  listResults,
  runNow
}

export default fingerprintAPI
