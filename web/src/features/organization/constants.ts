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
import type { StatusVariant } from '@/components/status-badge'

import type { OrganizationRole, OrganizationStatus } from './types'

// ============================================================================
// Organization Statuses
// ============================================================================

export const ORGANIZATION_STATUSES = {
  active: {
    labelKey: 'Active',
    variant: 'success' as StatusVariant,
    value: 'active',
  },
  disabled: {
    labelKey: 'Disabled',
    variant: 'danger' as StatusVariant,
    value: 'disabled',
  },
  dissolved: {
    labelKey: 'Dissolved',
    variant: 'neutral' as StatusVariant,
    value: 'dissolved',
  },
} as const satisfies Record<
  OrganizationStatus,
  { labelKey: string; variant: StatusVariant; value: OrganizationStatus }
>

export const organizationStatusLabelKey = (status?: string): string =>
  ORGANIZATION_STATUSES[status as OrganizationStatus]?.labelKey ?? 'Unknown'

/**
 * Statuses the caller's own organization list can actually contain.
 *
 * `GET /api/organizations` excludes dissolved organizations server-side, so
 * offering them as a filter would only ever produce an empty list. The dissolved
 * status still exists for the detail page, which stays readable after a
 * dissolve.
 */
export const ORGANIZATION_LIST_STATUSES: OrganizationStatus[] = [
  'active',
  'disabled',
]

// ============================================================================
// Organization Roles
// ============================================================================

export const ORGANIZATION_ROLES = {
  owner: {
    labelKey: 'Owner',
    variant: 'purple' as StatusVariant,
    value: 'owner',
  },
  admin: {
    labelKey: 'Admin',
    variant: 'blue' as StatusVariant,
    value: 'admin',
  },
  member: {
    labelKey: 'Member',
    variant: 'neutral' as StatusVariant,
    value: 'member',
  },
} as const satisfies Record<
  OrganizationRole,
  { labelKey: string; variant: StatusVariant; value: OrganizationRole }
>

export const organizationRoleLabelKey = (role?: string): string =>
  ORGANIZATION_ROLES[role as OrganizationRole]?.labelKey ?? 'Unknown'

/**
 * The role's badge, or `null` when the caller holds no role here.
 *
 * The role arrives as a plain string and is empty for a platform administrator
 * browsing an organization they do not belong to; that is a real state, not a
 * missing value to paper over with "Unknown".
 */
export const organizationRoleMeta = (
  role?: string
): { labelKey: string; variant: StatusVariant } | null =>
  ORGANIZATION_ROLES[role as OrganizationRole] ?? null

/** Roles an organization administrator may assign to another member. */
export const ORGANIZATION_ASSIGNABLE_ROLES: OrganizationRole[] = [
  'admin',
  'member',
]

/** The caller may browse the organization but not write to it. */
export const ORGANIZATION_ACCESS_MODE_READ_ONLY = 'read_only'

// ============================================================================
// Membership Statuses
// ============================================================================

/**
 * `organization_members.status`.
 *
 * `exited` and `removed` both end a membership, but they are not the same
 * event — one is the member's own decision — so they keep separate labels.
 */
export const ORGANIZATION_MEMBER_STATUSES = {
  active: { labelKey: 'Active', variant: 'success' as StatusVariant },
  disabled: { labelKey: 'Disabled', variant: 'warning' as StatusVariant },
  exited: { labelKey: 'Exited', variant: 'neutral' as StatusVariant },
  removed: { labelKey: 'Removed', variant: 'neutral' as StatusVariant },
} as const

export type OrganizationMemberStatus = keyof typeof ORGANIZATION_MEMBER_STATUSES

export const organizationMemberStatusMeta = (
  status?: string
): { labelKey: string; variant: StatusVariant } => {
  const meta =
    ORGANIZATION_MEMBER_STATUSES[status as OrganizationMemberStatus] ?? null
  return meta ?? { labelKey: 'Unknown status', variant: 'neutral' }
}

// ============================================================================
// Organization Detail Sections
// ============================================================================

export const ORGANIZATION_DETAIL_TAB_KEYS = [
  'overview',
  'members',
  'invitations',
  'tokens',
  'logs',
  'usage',
  'tasks',
  'audit-logs',
  'settings',
] as const

export type OrganizationDetailTabKey =
  (typeof ORGANIZATION_DETAIL_TAB_KEYS)[number]

export const ORGANIZATION_DEFAULT_TAB: OrganizationDetailTabKey = 'overview'

/** Older links used shorter section names; keep them resolving. */
export const ORGANIZATION_LEGACY_TAB_ALIASES: Record<
  string,
  OrganizationDetailTabKey
> = {
  invites: 'invitations',
  billing: 'usage',
  audit: 'audit-logs',
}

export const ORGANIZATION_TAB_LABEL_KEYS: Record<
  OrganizationDetailTabKey,
  string
> = {
  overview: 'Overview',
  members: 'Members',
  invitations: 'Invitations',
  tokens: 'API Keys',
  logs: 'Logs',
  usage: 'Usage',
  tasks: 'Tasks',
  'audit-logs': 'Audit Logs',
  settings: 'Settings',
}

// ============================================================================
// Messages (i18n keys: use t(ERROR_MESSAGES.xxx) when displaying)
// ============================================================================

export const ERROR_MESSAGES = {
  LOAD_FAILED: 'Failed to load organizations',
  CREATE_FAILED: 'Failed to create the organization',
  UPDATE_FAILED: 'Failed to update the organization',
  STATUS_FAILED: 'Failed to change the organization status',
  DISSOLVE_FAILED: 'Failed to dissolve the organization',
  NO_ORGANIZATION: 'No organization selected',
  UNEXPECTED: 'An unexpected error occurred',
} as const

export const SUCCESS_MESSAGES = {
  CREATED: 'Organization created',
  UPDATED: 'Organization updated',
  ENABLED: 'Organization enabled',
  DISABLED: 'Organization disabled',
  DISSOLVED: 'Organization dissolved',
} as const

/**
 * Shown as a banner when the organization is browsable but not writable. Each
 * state gets its own wording because the recovery differs: a disabled
 * organization may be re-enabled by an administrator, a dissolved one can
 * never come back.
 */
export const READ_ONLY_MESSAGE_KEYS = {
  disabled: 'This organization is disabled. Platform support is required.',
  dissolved: 'This organization has been dissolved. Its data is read-only.',
  read_only: 'You have read-only access to this organization.',
} as const

export type OrganizationReadOnlyReason = keyof typeof READ_ONLY_MESSAGE_KEYS
