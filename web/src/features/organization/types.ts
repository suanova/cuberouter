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
import { z } from 'zod'

import { organizationCapabilitiesSchema } from '@/lib/account-context'

// ============================================================================
// Organization Schema & Types
// ============================================================================

export const organizationStatusSchema = z.enum([
  'active',
  'disabled',
  'dissolved',
])
export type OrganizationStatus = z.infer<typeof organizationStatusSchema>

export const organizationRoleSchema = z.enum(['owner', 'admin', 'member'])
export type OrganizationRole = z.infer<typeof organizationRoleSchema>

/** How the current actor reaches this organization; see service/organization_policy.go. */
export const organizationAccessModeSchema = z.enum([
  'workspace',
  'management',
  'read_only',
  'admin',
])
export type OrganizationAccessMode = z.infer<
  typeof organizationAccessModeSchema
>

export const organizationSchema = z.object({
  id: z.number(),
  name: z.string(),
  slug: z.string(),
  description: z.string().optional(),
  group: z.string().optional(),
  status: organizationStatusSchema,
  quota: z.number(),
  used_quota: z.number(),
  request_count: z.number(),
  owner_user_id: z.number(),
  created_by: z.number(),
  created_at: z.number(),
  updated_at: z.number(),
  dissolved_at: z.number().optional(),
})
export type Organization = z.infer<typeof organizationSchema>

/**
 * Which source disabled the organization. `self` can be undone by the
 * organization's own administrators, `platform` cannot — that is what
 * `can_self_enable` reports.
 */
export const organizationDisableStateSchema = z.object({
  active_sources: z.array(z.string()),
  effective_source: z.string(),
  can_self_enable: z.boolean(),
})
export type OrganizationDisableState = z.infer<
  typeof organizationDisableStateSchema
>

/**
 * One row of `GET /api/organizations`.
 *
 * The backend always sends the full capability set (see
 * organizationActorCapabilitiesFromPolicy), so the UI reads these booleans
 * instead of re-deriving permissions from the role. The policy engine lives in
 * the backend only.
 */
export const userOrganizationSchema = organizationSchema.extend({
  role: organizationRoleSchema,
  disable_state: organizationDisableStateSchema,
  can_self_enable: z.boolean(),
  capabilities: organizationCapabilitiesSchema,
  access_mode: organizationAccessModeSchema,
})
export type UserOrganization = z.infer<typeof userOrganizationSchema>

export const organizationMemberSchema = z.object({
  id: z.number(),
  organization_id: z.number(),
  user_id: z.number(),
  role: organizationRoleSchema,
  status: z.string(),
  disabled_source: z.string().optional(),
  invited_by: z.number().optional(),
  joined_at: z.number().optional(),
  last_active_at: z.number().optional(),
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
  disabled_at: z.number().optional(),
  exited_at: z.number().optional(),
  removed_at: z.number().optional(),
})
export type OrganizationMember = z.infer<typeof organizationMemberSchema>

/**
 * `GET /api/organizations/:id` payload.
 *
 * `actor` carries the capability set and the access mode for the caller;
 * `member` is the caller's own membership row and is absent for a platform
 * administrator who is not a member.
 */
export const organizationDetailSchema = z.object({
  organization: organizationSchema,
  member: organizationMemberSchema.nullable().optional(),
  actor: z.object({
    user_id: z.number(),
    organization_id: z.number(),
    access_mode: organizationAccessModeSchema,
    role: z.string(),
    organization_role: z.string(),
    platform_role: z.string(),
    is_organization_member: z.boolean(),
    is_organization_admin: z.boolean(),
    is_platform_admin: z.boolean(),
    is_platform_root: z.boolean(),
    read_only: z.boolean(),
    capabilities: organizationCapabilitiesSchema,
  }),
})
export type OrganizationDetail = z.infer<typeof organizationDetailSchema>

// ============================================================================
// Response Envelopes
// ============================================================================

export interface OrganizationListResponse {
  success: boolean
  message?: string
  data?: UserOrganization[]
}

export interface OrganizationResponse {
  success: boolean
  message?: string
  data?: Organization
}

export interface OrganizationDetailResponse {
  success: boolean
  message?: string
  data?: OrganizationDetail
}

export interface OrganizationGroupsResponse {
  success: boolean
  message?: string
  data?: Record<string, { desc: string; ratio: number | string }>
}

// ============================================================================
// UI State
// ============================================================================

/**
 * Which overlay the organization center has open. Registering the target row in
 * a separate `currentRow` keeps the dialog mounted while it animates out, which
 * is why the type is not just a boolean per dialog.
 */
export type OrganizationDialogType = 'create' | 'update' | 'status' | 'dissolve'
