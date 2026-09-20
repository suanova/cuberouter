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

/**
 * Every organization endpoint answers with the same envelope, so the tab APIs
 * differ only in the payload. `success: false` carries the localized message and
 * sometimes a machine-readable `code` (see types/organization_error.go).
 */
export interface OrganizationApiResponse<T> {
  success: boolean
  message?: string
  code?: string
  data?: T
}

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
// Members
// ============================================================================

/**
 * A member row. The backend joins the user record in, so the display fields are
 * present even though they are not part of the membership itself.
 */
export interface OrganizationMemberRow {
  id: number
  organization_id: number
  user_id: number
  role: OrganizationRole
  status: string
  disabled_source?: string
  invited_by?: number
  joined_at?: number
  last_active_at?: number
  created_at?: number
  updated_at?: number
  disabled_at?: number
  exited_at?: number
  removed_at?: number
  username?: string
  display_name?: string
  email?: string
  /** Platform account status; `1` is active. */
  user_status?: number
}

// ============================================================================
// Invitations
// ============================================================================

export interface OrganizationInviteRow {
  id: number
  organization_id: number
  type?: string
  target_email: string
  role: OrganizationRole
  /** `pending` | `accepted` | `expired` | `revoked`. */
  status: string
  inviter_user_id?: number
  accepted_user_id?: number
  created_at?: number
  updated_at?: number
  expired_at?: number
  accepted_at?: number
  revoked_at?: number
  reason?: string
  /** `pending` | `sent` | `rejected` | `unknown`. */
  delivery_status?: string
  delivery_attempts?: number
  delivered_at?: number
  inviter_username?: string
  inviter_display_name?: string
}

/**
 * The invitation as an unauthenticated visitor sees it.
 *
 * `email_matched` is the server's verdict on whether the signed-in viewer may
 * accept this invitation; it is false when nobody is signed in.
 */
export interface OrganizationInvitePublicView {
  id: number
  organization_id: number
  organization_name: string
  organization_slug: string
  role: OrganizationRole
  target_email: string
  status: string
  inviter_user_id?: number
  inviter_username?: string
  inviter_display_name?: string
  current_user_id?: number
  current_user_email?: string
  email_matched: boolean
  expired_at?: number
}

// ============================================================================
// Platform Administration
// ============================================================================

/**
 * One row of `GET /api/admin/organizations`.
 *
 * The platform list is not scoped to the caller, so it cannot carry the
 * per-caller `role` and `capabilities` the organization center's rows do — an
 * outside administrator holds no role in the organization they are looking at.
 * What it carries instead is the set of counters an administrator triages on:
 * how many members and keys the organization has, and how many of each are
 * switched off.
 */
export interface OrganizationManagementView {
  id: number
  name: string
  slug: string
  description: string
  group: string
  status: OrganizationStatus
  quota: number
  used_quota: number
  request_count: number
  owner_user_id: number
  created_by: number
  created_at: number
  updated_at: number
  dissolved_at: number
  owner_username: string
  owner_display_name: string
  owner_email: string
  active_member_count: number
  disabled_member_count: number
  total_member_count: number
  enabled_token_count: number
  disabled_token_count: number
  total_token_count: number
}

/**
 * One entry of the organization's quota ledger, as `POST
 * /api/admin/organizations/:id/quota-adjustments` answers it.
 *
 * `quota_delta` is signed: the same endpoint grants quota and takes it back.
 */
export interface OrganizationQuotaAdjustment {
  id: number
  organization_id: number
  operator_user_id: number
  quota_delta: number
  quota_before: number
  quota_after: number
  used_quota: number
  reason: string
  created_at: number
}

// ============================================================================
// Organization API Keys
// ============================================================================

/**
 * An organization key. The list endpoint returns the plaintext `key` as well —
 * the organization administrator is allowed to see it — while `key_preview` is
 * the masked form used in the table.
 */
export interface OrganizationTokenRow {
  id: number
  /** The responsible member's user id, not the owner of a personal key. */
  user_id: number
  key: string
  key_preview?: string
  status: number
  name: string
  created_time: number
  accessed_time: number
  expired_time: number
  remain_quota: number
  unlimited_quota: boolean
  model_limits_enabled: boolean
  model_limits: string
  allow_ips?: string | null
  used_quota: number
  group: string
  cross_group_retry: boolean
  scope_type?: string
  scope_id?: number
  /** `private` keys are only usable by their responsible member. */
  visibility: string
  organization_id?: number
  creator_user_id?: number
  responsible_user_id?: number
  transfer_reason?: string
  updated_at?: number
  disabled_by_systems?: boolean
  system_disabled_reason?: string
  system_disabled_ref_id?: number
  system_disabled_at?: number
  previous_status?: number
  responsible_username?: string
  responsible_display_name?: string
  /** Why the key is currently force-disabled, if anything disabled it. */
  unavailable_reasons?: string[]
}

/**
 * Batch creation is idempotent, so a replay returns the tokens the first call
 * created. `secret_available` says whether their plaintext keys came back with
 * them — the dialog only offers the full secret when it is true.
 */
export interface OrganizationTokenBatchCreateResult {
  tokens: OrganizationTokenRow[]
  token_count: number
  secret_available: boolean
}

// ============================================================================
// Logs
// ============================================================================

export interface OrganizationLogRow {
  id: number
  user_id: number
  created_at: number
  type: number
  /** Blanked by the backend for organization reads; use the other fields. */
  content?: string
  username?: string
  token_name?: string
  model_name?: string
  quota: number
  prompt_tokens: number
  completion_tokens: number
  use_time: number
  is_stream: boolean
  channel?: number
  channel_name?: string
  token_id?: number
  group?: string
  ip?: string
  request_id?: string
  upstream_request_id?: string
  other?: string
  creator_user_id?: number
  creator_name?: string
  responsible_user_id?: number
  responsible_name?: string
  responsible_username?: string
  responsible_display_name?: string
}

export interface OrganizationLogStats {
  quota: number
  rpm: number
  tpm: number
}

// ============================================================================
// Tasks
// ============================================================================

export interface OrganizationTaskRow {
  id: number
  created_at: number
  updated_at: number
  task_id: string
  platform: string
  user_id: number
  group?: string
  channel_id?: number
  quota: number
  token_id?: number
  token_name?: string
  token_unlimited?: boolean
  request_id?: string
  actor_user_id?: number
  creator_user_id?: number
  creator_name?: string
  responsible_user_id?: number
  responsible_name?: string
  action?: string
  status: string
  fail_reason?: string
  submit_time?: number
  start_time?: number
  finish_time?: number
  progress?: string
  properties?: {
    input?: string
    upstream_model_name?: string
    origin_model_name?: string
  }
  username?: string
  data?: unknown
}

export interface OrganizationMidjourneyTaskRow {
  id: number
  code?: number
  user_id: number
  action?: string
  mj_id?: string
  prompt?: string
  prompt_en?: string
  description?: string
  state?: string
  submit_time?: number
  start_time?: number
  finish_time?: number
  image_url?: string
  video_url?: string
  video_urls?: string
  status?: string
  progress?: string
  fail_reason?: string
  channel_id?: number
  quota: number
  buttons?: string
  properties?: string
  token_name?: string
  token_unlimited?: boolean
  group?: string
  request_id?: string
  responsible_user_id?: number
  responsible_name?: string
}

// ============================================================================
// Audit Logs
// ============================================================================

export interface OrganizationAuditLogRow {
  id: number
  organization_id: number
  organization_name?: string
  organization_slug?: string
  operator_user_id?: number
  operator_username?: string
  operator_display_name?: string
  operator_role?: string
  /** A dotted key such as `organization.member.remove`. */
  action_type: string
  /** `organization` | `member` | `token` | `invite` | `quota_adjustment` | … */
  target_type?: string
  target_id?: number
  target_name?: string
  /** JSON-encoded string, not an object. */
  target_metadata?: string
  /** JSON-encoded snapshots; both are strings, not objects. */
  before_data?: string
  after_data?: string
  reason?: string
  ip?: string
  user_agent?: string
  created_at: number
}

// ============================================================================
// Quota Dashboard
// ============================================================================

export interface OrganizationQuotaDataRow {
  id: number
  user_id: number
  username?: string
  model_name: string
  created_at: number
  use_group?: string
  token_id?: number
  channel_id?: number
  token_used?: number
  count: number
  quota: number
  responsible_user_id?: number
}

// ============================================================================
// Billing
// ============================================================================

export interface OrganizationMemberBillingItem {
  responsible_user_id: number
  quota: number
  request_count: number
  token_count: number
}

export interface OrganizationBillingSummary {
  quota: number
  used_quota: number
  available_quota: number
  current_month_quota: number
  organization_keys: number
  members: OrganizationMemberBillingItem[]
}

export interface OrganizationMemberBillingSummary {
  responsible_user_id: number
  responsible_key_count: number
  current_month_quota: number
  request_count: number
}

export interface OrganizationBillingUserSummaryItem {
  responsible_user_id: number
  responsible_username?: string
  responsible_display_name?: string
  role?: string
  status?: string
  quota: number
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  token_count: number
}

export interface OrganizationBillingUserSummaryResponse {
  month: string
  month_start: number
  month_end: number
  items: OrganizationBillingUserSummaryItem[]
}

export interface OrganizationBillingMonthlySummaryItem {
  month: string
  month_start: number
  month_end: number
  quota: number
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  token_count: number
  responsible_user_count: number
}

export interface OrganizationBillingMonthlySummaryResponse {
  items: OrganizationBillingMonthlySummaryItem[]
}

export interface OrganizationBillingRecord {
  id: number
  organization_id: number
  session_id: number
  record_key: string
  request_id?: string
  task_id?: string
  record_type?: string
  quota_delta: number
  used_quota_delta: number
  usage_quota: number
  quota_before: number
  quota_after: number
  used_quota_before: number
  used_quota_after: number
  token_id?: number
  token_name?: string
  responsible_user_id?: number
  creator_user_id?: number
  model_name?: string
  group?: string
  prompt_tokens?: number
  completion_tokens?: number
  token_count?: number
  created_at: number
  responsible_username?: string
  responsible_display_name?: string
  ledger_quota_delta: number
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
