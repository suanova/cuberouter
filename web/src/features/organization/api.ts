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

import type {
  OrganizationDetailResponse,
  OrganizationGroupsResponse,
  OrganizationListResponse,
  OrganizationResponse,
} from './types'

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
