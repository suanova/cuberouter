/*
Copyright (C) 2023-2026 QuantumNous
Copyright (C) 2026 CubeRouter

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { api } from '@/lib/api'

import { withOrganizationIdempotencyKey } from './lib/organization-idempotency'
import type {
  OrganizationApiResponse,
  OrganizationAuditLogRow,
  OrganizationBillingMonthlySummaryResponse,
  OrganizationBillingRecord,
  OrganizationBillingSummary,
  OrganizationBillingUserSummaryResponse,
  OrganizationDetailResponse,
  OrganizationGroupsResponse,
  OrganizationInvitePublicView,
  OrganizationInviteRow,
  OrganizationListResponse,
  OrganizationLogRow,
  OrganizationLogStats,
  OrganizationMemberBillingSummary,
  OrganizationMemberRow,
  OrganizationMidjourneyTaskRow,
  OrganizationQuotaDataRow,
  OrganizationResponse,
  OrganizationTaskRow,
  OrganizationTokenBatchCreateResult,
  OrganizationTokenRow,
} from './types'
import type { PagedResult } from './lib/organization-pagination'

interface ApiEnvelope {
  success: boolean
  message?: string
  data?: null
}

// ============================================================================
// Organization Center
// ============================================================================

export async function getOrganizations(): Promise<OrganizationListResponse> {
  const res = await api.get('/api/organizations')
  return res.data
}

export async function createOrganization(data: {
  name: string
  description?: string
}): Promise<OrganizationResponse> {
  const res = await api.post('/api/organizations', data)
  return res.data
}

/**
 * `silent` keeps the API error interceptor from toasting: the caller is probing
 * whether the organization is readable in the current account context and
 * handles the failure itself.
 */
export async function getOrganization(
  organizationId: number,
  options: { silent?: boolean } = {}
): Promise<OrganizationDetailResponse> {
  const res = await api.get(`/api/organizations/${organizationId}`, {
    skipErrorHandler: options.silent,
  })
  return res.data
}

export async function updateOrganization(
  organizationId: number,
  data: {
    name: string
    description?: string
    group?: string
    reason?: string
  }
): Promise<OrganizationResponse> {
  const res = await api.patch(`/api/organizations/${organizationId}`, data)
  return res.data
}

/**
 * Enable or disable the organization. `confirm_name` has to repeat the
 * organization name exactly; the backend rejects the request otherwise, which is
 * what keeps a mis-clicked dialog from taking an organization offline.
 */
export async function updateOrganizationStatus(
  organizationId: number,
  data: { status: 'active' | 'disabled'; confirm_name: string; reason?: string }
): Promise<ApiEnvelope> {
  const res = await api.patch(
    `/api/organizations/${organizationId}/status`,
    data
  )
  return res.data
}

/**
 * Dissolve the organization. Irreversible, so the caller must supply an
 * `Idempotency-Key` that it reuses when retrying the same intent.
 */
export async function dissolveOrganization(
  organizationId: number,
  data: { confirm_name: string; reason?: string },
  idempotencyKey: string
): Promise<ApiEnvelope> {
  const res = await api.delete(`/api/organizations/${organizationId}`, {
    data,
    headers: { 'Idempotency-Key': idempotencyKey },
  })
  return res.data
}

export async function transferOrganizationOwner(
  organizationId: number,
  data: { owner_user_id: number; reason?: string },
  idempotencyKey: string
): Promise<ApiEnvelope> {
  const res = await api.put(`/api/organizations/${organizationId}/owner`, data, {
    headers: { 'Idempotency-Key': idempotencyKey },
  })
  return res.data
}

export async function getOrganizationGroups(
  organizationId: number
): Promise<OrganizationGroupsResponse> {
  const res = await api.get(`/api/organizations/${organizationId}/groups`)
  return res.data
}

// ============================================================================
// Members
// ============================================================================

export async function listOrganizationMembers(
  organizationId: number,
  params: { p: number; page_size: number }
): Promise<OrganizationApiResponse<PagedResult<OrganizationMemberRow>>> {
  const res = await api.get(`/api/organizations/${organizationId}/members`, {
    params,
  })
  return res.data
}

/**
 * Adds a user directly, without an invitation. Only a platform administrator
 * may do this, and only through the admin surface — the organization-scoped
 * group has no create-member route.
 */
export async function addOrganizationMember(
  organizationId: number,
  data: { user_id: number; role?: string; reason?: string }
): Promise<OrganizationApiResponse<OrganizationMemberRow>> {
  const res = await api.post(
    `/api/admin/organizations/${organizationId}/members`,
    data
  )
  return res.data
}

/**
 * Changes a member's role or status.
 *
 * Disabling goes through an idempotency key because a retry that lands twice
 * would record a second disable — and, more importantly, a second audit entry
 * for one action.
 */
export async function updateOrganizationMember(
  organizationId: number,
  userId: number,
  data: {
    role?: string
    status?: string
    transfer_to_user_id?: number
    reason?: string
  }
): Promise<ApiEnvelope> {
  const requiresIdempotency = data.status === 'disabled'
  if (!requiresIdempotency) {
    const res = await api.patch(
      `/api/organizations/${organizationId}/members/${userId}`,
      data
    )
    return res.data
  }

  return withOrganizationIdempotencyKey(
    'disable-member',
    `${organizationId}:${userId}`,
    data,
    async (idempotencyKey) => {
      const res = await api.patch(
        `/api/organizations/${organizationId}/members/${userId}`,
        data,
        { headers: { 'Idempotency-Key': idempotencyKey } }
      )
      return res.data
    }
  )
}

export async function removeOrganizationMember(
  organizationId: number,
  userId: number,
  data: { transfer_to_user_id?: number; reason?: string } = {}
): Promise<ApiEnvelope> {
  return withOrganizationIdempotencyKey(
    'remove-member',
    `${organizationId}:${userId}`,
    data,
    async (idempotencyKey) => {
      const res = await api.delete(
        `/api/organizations/${organizationId}/members/${userId}`,
        {
          data,
          headers: { 'Idempotency-Key': idempotencyKey },
        }
      )
      return res.data
    }
  )
}

/**
 * Leaves the organization. The transfer target is a query parameter here, not a
 * body field, because the endpoint is a `DELETE` the backend reads from the URL.
 */
export async function exitOrganization(
  organizationId: number,
  params: { transfer_to_user_id?: number; reason?: string } = {}
): Promise<ApiEnvelope> {
  return withOrganizationIdempotencyKey(
    'exit-member',
    organizationId,
    params,
    async (idempotencyKey) => {
      const res = await api.delete(
        `/api/organizations/${organizationId}/members/me`,
        {
          params,
          headers: { 'Idempotency-Key': idempotencyKey },
        }
      )
      return res.data
    }
  )
}

// ============================================================================
// Invitations
// ============================================================================

export async function listOrganizationInvites(
  organizationId: number,
  params: { p: number; page_size: number }
): Promise<OrganizationApiResponse<PagedResult<OrganizationInviteRow>>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/invitations`,
    { params }
  )
  return res.data
}

/**
 * Sends an invitation.
 *
 * `force_rotate` replaces a still-pending invitation for the same address. It
 * is retry-sensitive — re-sending mails the recipient again — so it carries an
 * idempotency key; a plain first send does not need one.
 *
 * Errors are left to the caller (`skipErrorHandler`): the failure modes are
 * distinguished by code, and each one needs a different action, not just a
 * different message.
 */
export async function createOrganizationInvite(
  organizationId: number,
  data: { email: string; role?: string; force_rotate?: boolean }
): Promise<OrganizationApiResponse<OrganizationInviteRow>> {
  if (data.force_rotate !== true) {
    const res = await api.post(
      `/api/organizations/${organizationId}/invitations`,
      data,
      { skipErrorHandler: true }
    )
    return res.data
  }

  return withOrganizationIdempotencyKey(
    'rotate-invite',
    organizationId,
    data,
    async (idempotencyKey) => {
      const res = await api.post(
        `/api/organizations/${organizationId}/invitations`,
        data,
        {
          skipErrorHandler: true,
          headers: { 'Idempotency-Key': idempotencyKey },
        }
      )
      return res.data
    }
  )
}

export async function revokeOrganizationInvite(
  organizationId: number,
  inviteId: number,
  params: { reason?: string } = {}
): Promise<ApiEnvelope> {
  const res = await api.delete(
    `/api/organizations/${organizationId}/invitations/${inviteId}`,
    { params }
  )
  return res.data
}

// ============================================================================
// Invitation Landing
// ============================================================================

/** Readable without signing in, so the invitation can be previewed first. */
export async function getOrganizationInvite(
  token: string
): Promise<OrganizationApiResponse<OrganizationInvitePublicView>> {
  const res = await api.get(`/api/organization-invitations/${token}`)
  return res.data
}

export async function acceptOrganizationInvite(
  token: string
): Promise<ApiEnvelope> {
  const res = await api.patch(`/api/organization-invitations/${token}`)
  return res.data
}

// ============================================================================
// Organization API Keys
// ============================================================================

export interface OrganizationTokenPayload {
  name: string
  status?: number
  expired_time?: number
  remain_quota?: number
  unlimited_quota?: boolean
  model_limits_enabled?: boolean
  model_limits?: string
  allow_ips?: string
  group?: string
  cross_group_retry?: boolean
  visibility?: string
  responsible_user_id?: number
}

export interface OrganizationTokenListParams {
  p: number
  page_size: number
  keyword?: string
  status?: number
  visibility?: string
  group?: string
  responsible_user_id?: number
}

export async function listOrganizationTokens(
  organizationId: number,
  params: OrganizationTokenListParams
): Promise<OrganizationApiResponse<PagedResult<OrganizationTokenRow>>> {
  const res = await api.get(`/api/organizations/${organizationId}/tokens`, {
    params,
  })
  return res.data
}

export async function getOrganizationToken(
  organizationId: number,
  tokenId: number
): Promise<OrganizationApiResponse<OrganizationTokenRow>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/tokens/${tokenId}`
  )
  return res.data
}

export async function createOrganizationToken(
  organizationId: number,
  data: OrganizationTokenPayload
): Promise<OrganizationApiResponse<OrganizationTokenRow>> {
  const res = await api.post(
    `/api/organizations/${organizationId}/tokens`,
    data
  )
  return res.data
}

/**
 * Creates several keys at once. Idempotent: a retry returns the batch the first
 * call created rather than creating a second one, which is why the plaintext
 * keys stay retrievable.
 */
export async function batchCreateOrganizationTokens(
  organizationId: number,
  data: OrganizationTokenPayload & { token_count: number }
): Promise<OrganizationApiResponse<OrganizationTokenBatchCreateResult>> {
  return withOrganizationIdempotencyKey(
    'batch-create-token',
    organizationId,
    data,
    async (idempotencyKey) => {
      const res = await api.post(
        `/api/organizations/${organizationId}/token-batches`,
        data,
        { headers: { 'Idempotency-Key': idempotencyKey } }
      )
      return res.data
    }
  )
}

export async function updateOrganizationToken(
  organizationId: number,
  tokenId: number,
  data: OrganizationTokenPayload
): Promise<OrganizationApiResponse<OrganizationTokenRow>> {
  const res = await api.patch(
    `/api/organizations/${organizationId}/tokens/${tokenId}`,
    data
  )
  return res.data
}

export async function deleteOrganizationToken(
  organizationId: number,
  tokenId: number
): Promise<ApiEnvelope> {
  const res = await api.delete(
    `/api/organizations/${organizationId}/tokens/${tokenId}`
  )
  return res.data
}

/** Returns the number of keys actually deleted. */
export async function batchDeleteOrganizationTokens(
  organizationId: number,
  ids: number[]
): Promise<OrganizationApiResponse<number>> {
  const data = { ids }
  return withOrganizationIdempotencyKey(
    'batch-delete-token',
    organizationId,
    data,
    async (idempotencyKey) => {
      const res = await api.post(
        `/api/organizations/${organizationId}/token-deletions`,
        data,
        { headers: { 'Idempotency-Key': idempotencyKey } }
      )
      return res.data
    }
  )
}

export async function updateOrganizationTokenResponsibility(
  organizationId: number,
  tokenId: number,
  data: { responsible_user_id: number; reason?: string }
): Promise<OrganizationApiResponse<OrganizationTokenRow>> {
  const res = await api.patch(
    `/api/organizations/${organizationId}/tokens/${tokenId}/responsible-user`,
    data
  )
  return res.data
}

// ============================================================================
// Usage, Logs and Tasks
// ============================================================================

/**
 * Per-model usage rows for the dashboard. The backend rejects a window wider
 * than 30 days with `success: false`, so the caller picks the granularity.
 */
export async function getOrganizationQuotaData(
  organizationId: number,
  params: { start_timestamp?: number; end_timestamp?: number } = {}
): Promise<OrganizationApiResponse<OrganizationQuotaDataRow[]>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/quota-data`,
    { params }
  )
  return res.data
}

export interface OrganizationLogListParams {
  p: number
  page_size: number
  type?: number
  token_id?: number
  token_name?: string
  model_name?: string
  group?: string
  request_id?: string
  responsible_user_id?: number
  responsible_name?: string
  start_timestamp?: number
  end_timestamp?: number
}

export async function listOrganizationLogs(
  organizationId: number,
  params: OrganizationLogListParams
): Promise<OrganizationApiResponse<PagedResult<OrganizationLogRow>>> {
  const res = await api.get(`/api/organizations/${organizationId}/logs`, {
    params,
  })
  return res.data
}

export async function getOrganizationLogStats(
  organizationId: number,
  params: Omit<OrganizationLogListParams, 'p' | 'page_size'> = {}
): Promise<OrganizationApiResponse<OrganizationLogStats>> {
  const res = await api.get(`/api/organizations/${organizationId}/logs/stats`, {
    params,
  })
  return res.data
}

export interface OrganizationTaskListParams {
  p: number
  page_size: number
  platform?: string
  task_id?: string
  status?: string
  action?: string
  start_timestamp?: number
  end_timestamp?: number
}

export async function listOrganizationTasks(
  organizationId: number,
  params: OrganizationTaskListParams
): Promise<OrganizationApiResponse<PagedResult<OrganizationTaskRow>>> {
  const res = await api.get(`/api/organizations/${organizationId}/tasks`, {
    params,
  })
  return res.data
}

export async function listOrganizationMidjourneyTasks(
  organizationId: number,
  params: { p: number; page_size: number; mj_id?: string; channel_id?: string }
): Promise<
  OrganizationApiResponse<PagedResult<OrganizationMidjourneyTaskRow>>
> {
  const res = await api.get(
    `/api/organizations/${organizationId}/midjourney-tasks`,
    { params }
  )
  return res.data
}

// ============================================================================
// Audit Logs
// ============================================================================

export interface OrganizationAuditLogParams {
  p: number
  page_size: number
  organization_id?: number
  organization_slug?: string
  operator_user_id?: number
  target_type?: string
  target_id?: number
  action_type?: string
  start_timestamp?: number
  end_timestamp?: number
}

export async function listOrganizationAuditLogs(
  organizationId: number,
  params: OrganizationAuditLogParams
): Promise<OrganizationApiResponse<PagedResult<OrganizationAuditLogRow>>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/audit-logs`,
    { params }
  )
  return res.data
}

/** Cross-organization audit trail, platform administrators only. */
export async function listAllOrganizationAuditLogs(
  params: OrganizationAuditLogParams
): Promise<OrganizationApiResponse<PagedResult<OrganizationAuditLogRow>>> {
  const res = await api.get('/api/admin/organization-audit-logs/', { params })
  return res.data
}

// ============================================================================
// Billing
// ============================================================================

export async function getOrganizationBillingSummary(
  organizationId: number
): Promise<OrganizationApiResponse<OrganizationBillingSummary>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/billing/summary`
  )
  return res.data
}

/** The caller's own numbers inside the organization. */
export async function getMyOrganizationMemberBilling(
  organizationId: number
): Promise<OrganizationApiResponse<OrganizationMemberBillingSummary>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/billing/members/me`
  )
  return res.data
}

export async function listOrganizationBillingUserSummaries(
  organizationId: number,
  params: { month?: string; start_month?: string; end_month?: string } = {}
): Promise<OrganizationApiResponse<OrganizationBillingUserSummaryResponse>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/billing/user-summaries`,
    { params }
  )
  return res.data
}

export async function listOrganizationBillingMonthlySummaries(
  organizationId: number,
  params: { months?: number } = {}
): Promise<OrganizationApiResponse<OrganizationBillingMonthlySummaryResponse>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/billing/monthly-summaries`,
    { params }
  )
  return res.data
}

export interface OrganizationBillingDetailParams {
  p: number
  page_size: number
  month?: string
  token_name?: string
  responsible_name?: string
  model_name?: string
  group?: string
  request_id?: string
  start_timestamp?: number
  end_timestamp?: number
}

export async function listOrganizationBillingDetails(
  organizationId: number,
  params: OrganizationBillingDetailParams
): Promise<OrganizationApiResponse<PagedResult<OrganizationBillingRecord>>> {
  const res = await api.get(
    `/api/organizations/${organizationId}/billing/records`,
    { params }
  )
  return res.data
}
