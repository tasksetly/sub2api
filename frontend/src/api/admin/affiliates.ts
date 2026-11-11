/**
 * Admin Affiliate API endpoints
 * Manage per-user affiliate (邀请返利) configurations:
 * exclusive invite codes (overrides aff_code) and exclusive rebate rates.
 */

import { apiClient } from '../client'
import type { PaginatedResponse } from '@/types'
import type { AffiliatePayout, AffiliatePayoutInput, AffiliateTransferRecord } from '@/types/affiliate'
export type { AffiliateTransferRecord, AffiliateOutflowAction } from '@/types/affiliate'

export interface AffiliateAdminEntry {
  user_id: number
  email: string
  username: string
  aff_code: string
  aff_code_custom: boolean
  aff_rebate_rate_percent?: number | null
  aff_count: number
}

export interface ListAffiliateUsersParams {
  page?: number
  page_size?: number
  search?: string
}

export interface ListAffiliateRecordsParams {
  page?: number
  page_size?: number
  search?: string
  start_at?: string
  end_at?: string
  sort_by?: string
  sort_order?: 'asc' | 'desc'
  timezone?: string
}

export interface AffiliateInviteRecord {
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number
  invitee_email: string
  invitee_username: string
  aff_code: string
  total_rebate: number
  created_at: string
}

/**
 * One rebate accrual. Non-order sources (redeem codes, admin recharges) carry no
 * order, so order fields are null; invitee_id is null when the invitee account
 * was deleted.
 */
export interface AffiliateRebateRecord {
  order_id: number | null
  out_trade_no: string
  inviter_id: number
  inviter_email: string
  inviter_username: string
  invitee_id: number | null
  invitee_email: string
  invitee_username: string
  order_amount: number | null
  pay_amount: number | null
  rebate_amount: number
  payment_type: string
  order_status: string
  created_at: string
}

export interface WithdrawAffiliateQuotaRequest {
  payout?: AffiliatePayoutInput
  /** Cashback deducted from the user, in CNY. */
  amount: number
}

export interface AffiliateWithdrawResult {
  payout?: AffiliatePayout | null
  ledger_id: number
  user_id: number
  amount: number
  available_quota_after: number
  frozen_quota_after: number
  history_quota_after: number
}

export interface AffiliateWithdrawResponse {
  result: AffiliateWithdrawResult
  /** True when the key matched an earlier registration and nothing was deducted again. */
  replayed: boolean
}

export interface AffiliateUserOverview {
  usdt_exchange_rate: number
  user_id: number
  email: string
  username: string
  aff_code: string
  rebate_rate_percent: number
  invited_count: number
  rebated_invitee_count: number
  available_quota: number
  history_quota: number
}

export interface UpdateAffiliateUserRequest {
  aff_code?: string
  aff_rebate_rate_percent?: number | null
  /** Set true to explicitly clear the per-user rate (sets it to NULL). */
  clear_rebate_rate?: boolean
}

export interface BatchSetRateRequest {
  user_ids: number[]
  aff_rebate_rate_percent?: number | null
  /** Set true to clear rates instead of setting. */
  clear?: boolean
}

export interface SimpleUser {
  id: number
  email: string
  username: string
}

export async function listUsers(
  params: ListAffiliateUsersParams = {},
): Promise<PaginatedResponse<AffiliateAdminEntry>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateAdminEntry>>(
    '/admin/affiliates/users',
    {
      params: {
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
        search: params.search ?? '',
      },
    },
  )
  return data
}

export async function lookupUsers(q: string): Promise<SimpleUser[]> {
  const { data } = await apiClient.get<SimpleUser[]>(
    '/admin/affiliates/users/lookup',
    { params: { q } },
  )
  return data
}

export async function updateUserSettings(
  userId: number,
  payload: UpdateAffiliateUserRequest,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.put<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
    payload,
  )
  return data
}

export async function clearUserSettings(
  userId: number,
): Promise<{ user_id: number }> {
  const { data } = await apiClient.delete<{ user_id: number }>(
    `/admin/affiliates/users/${userId}`,
  )
  return data
}

export async function batchSetRate(
  payload: BatchSetRateRequest,
): Promise<{ affected: number }> {
  const { data } = await apiClient.post<{ affected: number }>(
    '/admin/affiliates/users/batch-rate',
    payload,
  )
  return data
}

function recordParams(params: ListAffiliateRecordsParams = {}) {
  return {
    page: params.page ?? 1,
    page_size: params.page_size ?? 20,
    search: params.search ?? '',
    start_at: params.start_at || undefined,
    end_at: params.end_at || undefined,
    sort_by: params.sort_by || undefined,
    sort_order: params.sort_order || undefined,
    timezone: params.timezone || undefined,
  }
}

export async function listInviteRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateInviteRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateInviteRecord>>(
    '/admin/affiliates/invites',
    { params: recordParams(params) },
  )
  return data
}

export async function listRebateRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateRebateRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateRebateRecord>>(
    '/admin/affiliates/rebates',
    { params: recordParams(params) },
  )
  return data
}

export async function listTransferRecords(
  params: ListAffiliateRecordsParams = {},
): Promise<PaginatedResponse<AffiliateTransferRecord>> {
  const { data } = await apiClient.get<PaginatedResponse<AffiliateTransferRecord>>(
    '/admin/affiliates/transfers',
    { params: recordParams(params) },
  )
  return data
}

export async function getUserOverview(
  userId: number,
): Promise<AffiliateUserOverview> {
  const { data } = await apiClient.get<AffiliateUserOverview>(
    `/admin/affiliates/users/${userId}/overview`,
  )
  return data
}

/**
 * Records an offline withdrawal. `idempotencyKey` identifies one registration:
 * a retry with the same key deducts nothing and returns the first result, with
 * `replayed` set.
 */
export async function withdrawUserQuota(
  userId: number,
  payload: WithdrawAffiliateQuotaRequest,
  idempotencyKey: string,
  images: File[] = [],
): Promise<AffiliateWithdrawResponse> {
  let body: WithdrawAffiliateQuotaRequest | FormData = payload
  if (payload.payout) {
    const form = new FormData()
    form.append('payload', JSON.stringify(payload))
    images.forEach(image => form.append('images', image))
    body = form
  }
  const response = await apiClient.post<AffiliateWithdrawResult>(
    `/admin/affiliates/users/${userId}/withdraw`,
    body,
    { headers: {
      'Idempotency-Key': idempotencyKey,
      ...(payload.payout ? { 'Content-Type': 'multipart/form-data' } : {}),
    } },
  )
  return {
    result: response.data,
    replayed: response.headers?.['x-idempotency-replayed'] === 'true',
  }
}

export const affiliatesAPI = {
  listUsers,
  lookupUsers,
  updateUserSettings,
  clearUserSettings,
  batchSetRate,
  listInviteRecords,
  listRebateRecords,
  listTransferRecords,
  getUserOverview,
  withdrawUserQuota,
}

export default affiliatesAPI
