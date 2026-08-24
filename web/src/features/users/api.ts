/*
Copyright (C) 2023-2026 QuantumNous

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
import type { PermissionCatalog } from '@/lib/admin-permissions'
import { api } from '@/lib/api'
import type { CustomOAuthBinding } from '@/lib/oauth'

import type {
  User,
  GetUsersParams,
  GetUsersResponse,
  SearchUsersParams,
  UserFormData,
  ManageUserAction,
  ManageUserQuotaPayload,
  ApiResponse,
  InviteesListData,
  UserDashboardPayload,
} from './types'
import type { ExportUsersPayload } from './lib/export-utils'

// ============================================================================
// User Management APIs
// ============================================================================

/**
 * Get paginated users list
 */
export async function getUsers(
  params: GetUsersParams = {}
): Promise<GetUsersResponse> {
  const { p = 1, page_size = 10, sort_by, sort_order } = params
  const res = await api.get('/api/user/', {
    params: {
      p,
      page_size,
      sort_by,
      sort_order,
    },
  })
  return res.data
}

/**
 * Search users by keyword or group
 */
export async function searchUsers(
  params: SearchUsersParams
): Promise<GetUsersResponse> {
  const {
    keyword = '',
    group = '',
    role = '',
    status = '',
    p = 1,
    page_size = 10,
    sort_by,
    sort_order,
  } = params
  const queryParams = new URLSearchParams()
  queryParams.set('keyword', keyword)
  queryParams.set('group', group)
  if (role) queryParams.set('role', role)
  if (status) queryParams.set('status', status)
  queryParams.set('p', String(p))
  queryParams.set('page_size', String(page_size))
  if (sort_by) queryParams.set('sort_by', sort_by)
  if (sort_order) queryParams.set('sort_order', sort_order)
  const res = await api.get(`/api/user/search?${queryParams.toString()}`)
  return res.data
}

/**
 * Get single user by ID
 */
export async function getUser(id: number): Promise<ApiResponse<User>> {
  const res = await api.get(`/api/user/${id}`)
  return res.data
}

/**
 * Create a new user
 */
export async function createUser(
  data: UserFormData
): Promise<ApiResponse<User>> {
  const res = await api.post('/api/user/', data)
  return res.data
}

/**
 * Update an existing user
 */
export async function updateUser(
  data: UserFormData & { id: number }
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.put('/api/user/', data)
  return res.data
}

/**
 * Delete a single user (hard delete)
 */
export async function deleteUser(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/`)
  return res.data
}

/**
 * Manage user (promote, demote, enable, disable, delete)
 */
export async function manageUser(
  id: number,
  action: ManageUserAction
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', { id, action })
  return res.data
}

/**
 * Adjust user quota atomically (add/subtract/override)
 */
export async function adjustUserQuota(
  payload: ManageUserQuotaPayload
): Promise<ApiResponse<Partial<User>>> {
  const res = await api.post('/api/user/manage', payload)
  return res.data
}

/**
 * Reset user's Passkey registration
 */
export async function resetUserPasskey(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/reset_passkey`)
  return res.data
}

/**
 * Reset user's Two-Factor Authentication setup
 */
export async function resetUserTwoFA(id: number): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${id}/2fa`)
  return res.data
}

/**
 * Get all available groups
 */
export async function getGroups(): Promise<ApiResponse<string[]>> {
  const res = await api.get('/api/group/')
  return res.data
}

/**
 * Get the permission catalog (resources, actions, and role baselines).
 * Source of truth lives in the backend authz package.
 */
export async function getPermissionCatalog(): Promise<PermissionCatalog> {
  const res = await api.get('/api/authz/catalog')
  return {
    resources: res.data?.data?.resources ?? [],
    roles: res.data?.data?.roles ?? [],
  }
}

// ============================================================================
// Admin Binding Management APIs
// ============================================================================

/**
 * Get user's custom OAuth bindings (admin)
 */
export async function getUserOAuthBindings(
  userId: number
): Promise<ApiResponse<CustomOAuthBinding[]>> {
  const res = await api.get(`/api/user/${userId}/oauth/bindings`)
  return res.data
}

/**
 * Clear a user's built-in binding (admin)
 */
export async function adminClearUserBinding(
  userId: number,
  bindingType: string
): Promise<ApiResponse> {
  const res = await api.delete(`/api/user/${userId}/bindings/${bindingType}`)
  return res.data
}

/**
 * Unbind custom OAuth for a user (admin)
 */
export async function adminUnbindCustomOAuth(
  userId: number,
  providerId: number
): Promise<ApiResponse> {
  const res = await api.delete(
    `/api/user/${userId}/oauth/bindings/${providerId}`
  )
  return res.data
}

// ============================================================================
// Admin User Invitees / Export / Dashboard APIs
// ============================================================================

/**
 * Get the paginated invitee list of a user (admin)
 */
export async function getUserInvitees(
  userId: number,
  params: { p: number; page_size: number }
): Promise<ApiResponse<InviteesListData>> {
  const res = await api.get(`/api/user/${userId}/invitees`, { params })
  return res.data
}

/**
 * Get a user's quota/usage trend (admin, role-hierarchy gated)
 */
export async function getUserQuotaDates(
  userId: number,
  params: { start_timestamp: number; end_timestamp: number }
): Promise<ApiResponse<UserDashboardPayload>> {
  const res = await api.get(`/api/user/${userId}/quota-dates`, { params })
  return res.data
}

export interface ExportUsersResult {
  blob: Blob
  filename?: string
}

/**
 * The backend error envelope for the export endpoint. Errors are returned as
 * HTTP 200 with `{"success":false,"message":...}` (common.ApiError/ApiErrorI18n),
 * which axios with `responseType: 'blob'` delivers as a blob.
 */
interface ExportErrorEnvelope {
  success: boolean
  message?: string
}

/**
 * Export users to CSV (admin). ids take precedence over keyword/group.
 *
 * The backend streams export errors as HTTP 200 with a JSON error envelope
 * instead of a non-2xx status, so without this check a failed batch would be
 * downloaded as a bogus CSV. A successful CSV always starts with the UTF-8
 * BOM (EF BB BF), which never parses as JSON, so only the error envelope
 * matches here. Throwing makes the caller's existing catch surface the
 * localized toast.
 */
export async function exportUsers(
  payload: ExportUsersPayload
): Promise<ExportUsersResult> {
  const res = await api.post('/api/user/export', payload, {
    responseType: 'blob',
    // Error responses are blobs here, so the axios error interceptor cannot
    // read the server message; let the caller surface a localized toast.
    skipErrorHandler: true,
  })
  const blob = res.data as Blob
  const text = await blob.text()
  let envelope: ExportErrorEnvelope | null = null
  try {
    envelope = JSON.parse(text) as ExportErrorEnvelope
  } catch {
    // Not JSON — a real CSV (starts with the UTF-8 BOM).
  }
  if (envelope && typeof envelope === 'object' && envelope.success === false) {
    throw new Error(envelope.message ?? 'Export failed')
  }
  const disposition = res.headers['content-disposition'] as string | undefined
  const filename = disposition?.match(/filename="([^"]+)"/)?.[1]
  return { blob, filename }
}
