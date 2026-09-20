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

import type { OrganizationCapabilities } from '@/lib/account-context'

import type { OrganizationMemberRow } from '@/features/organization/types'

/** Where one organization's platform page lives. Spelled out rather than
 * derived, because the router needs the literal for its own route types. */
const PLATFORM_ORGANIZATION_DETAIL_ROUTE = '/admin/organizations'

// ============================================================================
// Sections
// ============================================================================

/**
 * The sections an administrator sees on one organization.
 *
 * The organization center's own tab set does not apply here in either
 * direction. Two of its tabs are member business — settings, which is where an
 * organization renames itself and dissolves of its own accord, and invitations,
 * which only a member may send — and the platform page wants one the member
 * page has no reason to show: handing ownership to another member, which exists
 * precisely because the current owner may be the thing that is broken.
 */
export const PLATFORM_ORGANIZATION_TAB_KEYS = [
  'overview',
  'members',
  'tokens',
  'logs',
  'tasks',
  'usage',
  'audit-logs',
  'owner-repair',
] as const

export type PlatformOrganizationTabKey =
  (typeof PLATFORM_ORGANIZATION_TAB_KEYS)[number]

export const PLATFORM_ORGANIZATION_DEFAULT_TAB: PlatformOrganizationTabKey =
  'overview'

/** Older links used other names for these sections; keep them resolving. */
export const PLATFORM_ORGANIZATION_LEGACY_TAB_ALIASES: Record<
  string,
  PlatformOrganizationTabKey
> = {
  billing: 'usage',
  audit: 'audit-logs',
  owner: 'owner-repair',
}

export const PLATFORM_ORGANIZATION_TAB_LABEL_KEYS: Record<
  PlatformOrganizationTabKey,
  string
> = {
  overview: 'Overview',
  members: 'Members',
  tokens: 'API Keys',
  logs: 'Logs',
  tasks: 'Tasks',
  usage: 'Usage',
  'audit-logs': 'Audit Logs',
  'owner-repair': 'Transfer Ownership',
}

/** Resolve an arbitrary section value to a real tab, falling back to the
 * overview rather than 404-ing, because the section arrives from the URL. */
export function normalizePlatformOrganizationTabKey(
  value: unknown
): PlatformOrganizationTabKey {
  const raw = String(value ?? '').trim()
  const segments = raw.split('/').filter(Boolean)
  // The section is the last segment, so a link copied from a nested path still
  // resolves to the tab it names.
  const segment = segments.at(-1) ?? ''
  const canonical = PLATFORM_ORGANIZATION_LEGACY_TAB_ALIASES[segment] ?? segment
  return (PLATFORM_ORGANIZATION_TAB_KEYS as readonly string[]).includes(canonical)
    ? (canonical as PlatformOrganizationTabKey)
    : PLATFORM_ORGANIZATION_DEFAULT_TAB
}

export function getPlatformOrganizationDetailPath(
  organizationId: number | string,
  tabKey: unknown = PLATFORM_ORGANIZATION_DEFAULT_TAB
): string {
  return `${PLATFORM_ORGANIZATION_DETAIL_ROUTE}/${organizationId}/${normalizePlatformOrganizationTabKey(tabKey)}`
}

// ============================================================================
// Permissions
// ============================================================================

/**
 * Whether the whole page is read-only for its visitor.
 *
 * Only a dissolved organization is read-only here. The organization center
 * treats a disabled organization as read-only too, because its members have to
 * wait for platform support; an administrator *is* that support, so disabling
 * must not take away the buttons that would undo it.
 */
export function isPlatformOrganizationReadOnly(status?: string): boolean {
  return String(status ?? '').toLowerCase() === 'dissolved'
}

export interface PlatformOrganizationDetailActions {
  readOnly: boolean
  canEdit: boolean
  canDisable: boolean
  canEnable: boolean
  canAdjustQuota: boolean
  canDissolve: boolean
  canTransferOwner: boolean
}

/**
 * What the page's header offers.
 *
 * Every flag is the backend's own answer for this caller, read from the
 * capabilities the detail payload carries. The list page cannot do this — the
 * list endpoint answers no capability set — so it derives what it shows from the
 * administrator's role instead, which is why the two have separate helpers.
 */
export function getPlatformOrganizationDetailActions(input: {
  capabilities: OrganizationCapabilities
  status?: string
}): PlatformOrganizationDetailActions {
  const { capabilities } = input
  const readOnly = isPlatformOrganizationReadOnly(input.status)
  const status = String(input.status ?? '').toLowerCase()
  const writesAllowed = !readOnly

  return {
    readOnly,
    canEdit: writesAllowed && capabilities.can_update_organization,
    canDisable: writesAllowed && status === 'active' &&
      capabilities.can_disable_organization,
    canEnable: writesAllowed && status === 'disabled' &&
      capabilities.can_enable_organization,
    // Adjusting the quota is the second half of an edit the administrator
    // already has: the drawer sends it only when the remaining quota moved.
    canAdjustQuota: writesAllowed && capabilities.can_update_organization,
    canDissolve: writesAllowed && capabilities.can_dissolve_organization,
    canTransferOwner: writesAllowed && capabilities.can_transfer_owner,
  }
}

export interface PlatformOrganizationTabSpec {
  key: PlatformOrganizationTabKey
  labelKey: string
  readOnly: boolean
}

/**
 * Which sections to show, in order.
 *
 * A section whose read the backend would refuse is dropped rather than shown
 * disabled: its data requests would be rejected, and an empty tab reads as "there
 * is nothing here" when it means "you may not look".
 */
export function getPlatformOrganizationTabs(input: {
  capabilities: OrganizationCapabilities
  status?: string
}): PlatformOrganizationTabSpec[] {
  const { capabilities } = input
  const readOnly = isPlatformOrganizationReadOnly(input.status)

  const specs: Array<{ key: PlatformOrganizationTabKey; visible: boolean }> = [
    { key: 'overview', visible: capabilities.can_view_organization },
    {
      key: 'members',
      visible:
        capabilities.can_manage_members ||
        capabilities.can_view_organization_wide_data,
    },
    { key: 'tokens', visible: capabilities.can_view_organization_tokens },
    { key: 'logs', visible: capabilities.can_view_organization_logs },
    { key: 'tasks', visible: capabilities.can_view_organization_logs },
    { key: 'usage', visible: capabilities.can_view_organization_usage },
    { key: 'audit-logs', visible: capabilities.can_view_audit },
    // Offered whenever the backend would allow the transfer, not only on an
    // active organization: repointing an organization whose owner is gone is
    // what this tab is for, and that owner is just as gone while the
    // organization's traffic is switched off.
    {
      key: 'owner-repair',
      visible: !readOnly && capabilities.can_transfer_owner,
    },
  ]

  return specs
    .filter((spec) => spec.visible)
    .map(({ key }) => ({
      key,
      labelKey: PLATFORM_ORGANIZATION_TAB_LABEL_KEYS[key],
      readOnly,
    }))
}

// ============================================================================
// Owner Transfer
// ============================================================================

/**
 * Handing ownership of an organization to another of its members.
 *
 * The reason is required and the slug has to be retyped, as on every other
 * platform action against an organization the administrator does not belong to:
 * this changes who controls the organization's keys and its quota, and its own
 * members read the audit entry afterwards.
 */
export const platformOrganizationOwnerTransferSchema = z.object({
  owner_user_id: z.coerce
    .number()
    .refine(Number.isFinite, { message: 'Choose a member' })
    .refine((value) => value > 0, { message: 'Choose a member' }),
  reason: z.string().trim().min(1, 'Reason is required'),
})

export type PlatformOrganizationOwnerTransferValues = z.infer<
  typeof platformOrganizationOwnerTransferSchema
>

export function transformPlatformOrganizationOwnerTransfer(
  values: PlatformOrganizationOwnerTransferValues
): { owner_user_id: number; reason: string } {
  return {
    owner_user_id: Math.trunc(values.owner_user_id),
    reason: values.reason.trim(),
  }
}

/**
 * The members who could become the organization's owner.
 *
 * Any active member will do — the backend's owner repair locks the target as an
 * active membership and nothing more — so this must not be narrowed to the
 * administrators, which is what the member-side transfer picker does. A member
 * whose platform account is switched off is left out: the transfer would leave
 * the organization owned by someone who cannot sign in, which is the state this
 * panel exists to repair.
 *
 * The current owner is left out as well. Naming them is not a repair, and the
 * backend would happily accept it and write an audit entry saying nothing
 * happened.
 */
export function getPlatformOrganizationOwnerOptions(
  members: OrganizationMemberRow[] = [],
  currentOwnerUserId?: number
): Array<{ label: string; value: number }> {
  const currentOwner = Number(currentOwnerUserId ?? 0)

  return members
    .filter((member) => {
      const userId = Number(member?.user_id ?? 0)
      return (
        userId > 0 &&
        userId !== currentOwner &&
        String(member?.status ?? '').toLowerCase() === 'active' &&
        Number(member?.user_status ?? 1) === 1
      )
    })
    .map((member) => ({
      label:
        member.display_name ||
        member.username ||
        member.email ||
        String(member.user_id),
      value: Number(member.user_id),
    }))
}
