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
import {
  ORGANIZATION_ACCESS_MODE_READ_ONLY,
  ORGANIZATION_DEFAULT_TAB,
  ORGANIZATION_DETAIL_TAB_KEYS,
  ORGANIZATION_LEGACY_TAB_ALIASES,
  ORGANIZATION_TAB_LABEL_KEYS,
  type OrganizationDetailTabKey,
  type OrganizationReadOnlyReason,
} from '../constants'
import type { OrganizationMember, UserOrganization } from '../types'

type Capabilities = UserOrganization['capabilities']

// ============================================================================
// Paths
// ============================================================================

function lastPathSegment(value: unknown): string {
  const raw = String(value ?? '').trim()
  if (!raw) return ''
  const segments = raw.split('/').filter(Boolean)
  return segments.at(-1) ?? ''
}

/**
 * Resolve an arbitrary section value to a real tab. Unknown values fall back to
 * the overview rather than 404-ing, because the section arrives from the URL.
 */
export function normalizeOrganizationTabKey(
  value: unknown
): OrganizationDetailTabKey {
  const segment = lastPathSegment(value)
  const canonical = ORGANIZATION_LEGACY_TAB_ALIASES[segment] ?? segment
  return (ORGANIZATION_DETAIL_TAB_KEYS as readonly string[]).includes(canonical)
    ? (canonical as OrganizationDetailTabKey)
    : ORGANIZATION_DEFAULT_TAB
}

export function getOrganizationDetailPath(
  organizationId: number | string,
  tabKey: unknown = ORGANIZATION_DEFAULT_TAB
): string {
  return `/organizations/${organizationId}/${normalizeOrganizationTabKey(tabKey)}`
}

// ============================================================================
// Permissions
// ============================================================================

export interface OrganizationAccessInput {
  capabilities: Capabilities
  /** Organization status; a disabled or dissolved organization is read-only. */
  status?: string
  /** How the caller reaches the organization; `read_only` disables all writes. */
  accessMode?: string
  /** Explicit override, e.g. from the detail payload's actor. */
  readOnly?: boolean
}

export interface OrganizationReadOnlyState {
  readOnly: boolean
  reason: OrganizationReadOnlyReason | null
}

/**
 * Organization-wide read-only mode.
 *
 * Note this is about the organization, not about the caller's role: a dissolved
 * organization stays browsable so members can still read their history.
 */
export function getOrganizationReadOnlyState(
  input: OrganizationAccessInput
): OrganizationReadOnlyState {
  const status = String(input.status ?? '').toLowerCase()
  if (status === 'dissolved') return { readOnly: true, reason: 'dissolved' }
  if (status === 'disabled') return { readOnly: true, reason: 'disabled' }
  if (input.readOnly || input.accessMode === ORGANIZATION_ACCESS_MODE_READ_ONLY) {
    return { readOnly: true, reason: 'read_only' }
  }
  return { readOnly: false, reason: null }
}

/** Row actions for the organization center list. */
export function getOrganizationListActionFlags(
  organization: Pick<UserOrganization, 'capabilities'>
) {
  const capabilities = organization.capabilities
  return {
    enter: capabilities.can_view_organization,
    edit: capabilities.can_update_organization,
    disable: capabilities.can_disable_organization,
    enable: capabilities.can_enable_organization,
    dissolve: capabilities.can_dissolve_organization,
  }
}

/**
 * Whether opening this organization should also select it as the account
 * context.
 *
 * Only an active organization can be a context — the server refuses anything
 * else (middleware/account_context.go) — and a read-only caller has nothing to
 * write, so switching would only narrow what they can read. Everything else is
 * opened without touching the context, which works because the read endpoints
 * skip the context check for a non-active organization.
 */
export function isOrganizationEnterable(
  organization: Pick<UserOrganization, 'status' | 'access_mode'>
): boolean {
  return (
    String(organization.status ?? '').toLowerCase() === 'active' &&
    organization.access_mode !== ORGANIZATION_ACCESS_MODE_READ_ONLY
  )
}

export interface OrganizationTabSpec {
  key: OrganizationDetailTabKey
  labelKey: string
  /** The tab is visible but its contents are narrowed to the caller's own rows. */
  limitedView: boolean
  /** Write actions inside the tab are unavailable; the tab itself is readable. */
  readOnly: boolean
}

/**
 * Which sections to show for the current caller, in section order.
 *
 * Every flag comes straight from the backend capability set; the frontend never
 * re-derives permissions from the role. A section is dropped entirely rather
 * than shown disabled, because the backend would reject its data requests
 * anyway.
 */
export function getOrganizationTabs(
  input: OrganizationAccessInput
): OrganizationTabSpec[] {
  const { capabilities } = input
  const { readOnly } = getOrganizationReadOnlyState(input)
  const limitedView = !capabilities.can_view_organization_wide_data

  // readOnly is uniform per session, so it is filled in at the end rather than
  // repeated on every entry.
  const specs: Array<
    Omit<OrganizationTabSpec, 'labelKey' | 'readOnly'> & { visible: boolean }
  > = [
    {
      key: 'overview',
      visible: capabilities.can_view_organization,
      limitedView: false,
    },
    {
      key: 'members',
      visible:
        capabilities.can_manage_members ||
        capabilities.can_view_members_limited ||
        capabilities.can_view_organization_wide_data,
      // Members are listed, but the caller only sees their own usage.
      limitedView: !capabilities.can_manage_members,
    },
    {
      key: 'invitations',
      visible: capabilities.can_view_invites,
      limitedView: false,
    },
    {
      key: 'tokens',
      visible: capabilities.can_view_organization_tokens,
      limitedView: !capabilities.can_manage_all_tokens,
    },
    {
      key: 'logs',
      visible: capabilities.can_view_organization_logs,
      limitedView,
    },
    {
      key: 'usage',
      visible: capabilities.can_view_organization_usage,
      limitedView,
    },
    {
      key: 'tasks',
      visible: capabilities.can_view_organization_logs,
      limitedView,
    },
    {
      key: 'audit-logs',
      visible: capabilities.can_view_audit,
      limitedView: false,
    },
    {
      key: 'settings',
      visible:
        !readOnly &&
        (capabilities.can_manage_members ||
          capabilities.can_dissolve_organization ||
          capabilities.can_update_organization),
      limitedView: false,
    },
  ]

  return specs
    .filter((spec) => spec.visible)
    .map(({ key, limitedView: limited }) => ({
      key,
      labelKey: ORGANIZATION_TAB_LABEL_KEYS[key],
      limitedView: limited,
      readOnly,
    }))
}

// ============================================================================
// Member Actions
// ============================================================================

export interface OrganizationMemberActionFlags {
  canChangeRole: boolean
  canChangeStatus: boolean
  canRemove: boolean
}

export interface OrganizationMemberActionInput {
  actorRole?: string
  targetRole?: string
  canManageMembers?: boolean
  readOnly?: boolean
  isCurrentUser?: boolean
  /**
   * The caller administers this organization from the platform rather than
   * belonging to it.
   *
   * Such a caller has no organization role at all — the backend reports an empty
   * `organization_role` for a platform administrator who is not a member — so
   * the role hierarchy below cannot speak for them and the owner/admin/member
   * comparison would deny every row. What actually limits them is the backend:
   * `ensureOrganizationMemberTargetAllowed` refuses any operation against the
   * owner's own membership, for every role, so the owner's row stays untouchable
   * here too.
   */
  platformAdministrator?: boolean
}

function normalizeRole(role: unknown): string {
  return String(role ?? '').toLowerCase()
}

/**
 * What an organization administrator may do to one member row.
 *
 * Owners are never a valid target: transferring ownership is a separate
 * operation (PUT /api/organizations/:id/owner), so the member list must not
 * offer to demote or remove the owner.
 */
export function getOrganizationMemberActionFlags({
  actorRole,
  targetRole,
  canManageMembers = false,
  readOnly = false,
  isCurrentUser = false,
  platformAdministrator = false,
}: OrganizationMemberActionInput): OrganizationMemberActionFlags {
  const denied: OrganizationMemberActionFlags = {
    canChangeRole: false,
    canChangeStatus: false,
    canRemove: false,
  }
  if (!canManageMembers || readOnly || isCurrentUser) return denied

  const target = normalizeRole(targetRole)
  if (platformAdministrator) {
    return target === 'owner'
      ? denied
      : { canChangeRole: true, canChangeStatus: true, canRemove: true }
  }

  const actor = normalizeRole(actorRole)
  const canManageTarget =
    (actor === 'owner' && (target === 'admin' || target === 'member')) ||
    (actor === 'admin' && target === 'member')
  if (!canManageTarget) return denied

  return { canChangeRole: true, canChangeStatus: true, canRemove: true }
}

/**
 * Demoting an admin to member is only allowed together with handing ownership
 * over; the backend enforces this and the UI must ask for the new owner up
 * front rather than letting the request fail.
 */
export function isOrganizationMemberDemotion(
  member: Pick<OrganizationMember, 'role'> | null | undefined,
  nextRole: unknown
): boolean {
  return (
    normalizeRole(member?.role) === 'admin' &&
    normalizeRole(nextRole) === 'member'
  )
}

export interface OrganizationMemberRoleUpdatePayload {
  role: string
  transfer_to_user_id?: number
  reason?: string
}

export function buildOrganizationMemberRoleUpdatePayload(
  member: Pick<OrganizationMember, 'role'>,
  nextRole: string,
  { transferToUserId, reason }: { transferToUserId?: number; reason?: string } = {}
): OrganizationMemberRoleUpdatePayload {
  const payload: OrganizationMemberRoleUpdatePayload = { role: nextRole }
  if (!isOrganizationMemberDemotion(member, nextRole)) return payload

  const targetUserId = Number(transferToUserId ?? 0)
  if (targetUserId > 0) payload.transfer_to_user_id = targetUserId
  payload.reason = String(reason ?? '').trim()
  return payload
}

export interface OrganizationMemberOption {
  label: string
  value: number
}

/** The member fields the pickers below read. The list endpoint joins the user
 * record in, so the display fields are present on a member row. */
type MemberOptionSource = Partial<OrganizationMember> & {
  username?: string
  display_name?: string
  email?: string
  user_status?: number
}

/**
 * An active membership held by a user whose platform account is enabled.
 *
 * Both halves are required by the backend wherever a member is named as the
 * holder of something — `activeOrganizationMemberWithEnabledUserQuery` joins the
 * user row on the same two conditions — so a picker that offered only one of
 * them would produce refusals the operator cannot act on.
 */
function hasEnabledActiveAccount(member: MemberOptionSource): boolean {
  return (
    Number(member?.user_id ?? 0) > 0 &&
    String(member?.status ?? '').toLowerCase() === 'active' &&
    Number(member?.user_status ?? 1) === 1
  )
}

function toMemberOption(member: MemberOptionSource): OrganizationMemberOption {
  return {
    label:
      member.username ||
      member.display_name ||
      member.email ||
      String(member.user_id),
    value: Number(member.user_id),
  }
}

/**
 * Members who could take over ownership: active admins or owners other than the
 * one stepping down. A user whose platform account is disabled cannot be an
 * owner, so they are excluded too.
 */
export function getOrganizationTransferMemberOptions(
  members: MemberOptionSource[] = [],
  excludedUserId?: number
): OrganizationMemberOption[] {
  const excluded = Number(excludedUserId ?? 0)
  return members
    .filter(
      (member) =>
        hasEnabledActiveAccount(member) &&
        Number(member.user_id) !== excluded &&
        ['owner', 'admin'].includes(normalizeRole(member?.role))
    )
    .map(toMemberOption)
}

/**
 * Members who could hold an organization key.
 *
 * Any member of standing may hold a private key, but a published one has to be
 * held by the owner or an administrator: a key every member can use must still
 * have someone able to disable it, and its holder is the only member who could
 * otherwise do so.
 */
export function getOrganizationTokenResponsibleOptions(
  members: MemberOptionSource[] = [],
  visibility: string
): OrganizationMemberOption[] {
  const publicKey = visibility === 'public'
  return members
    .filter(
      (member) =>
        hasEnabledActiveAccount(member) &&
        (!publicKey || ['owner', 'admin'].includes(normalizeRole(member?.role)))
    )
    .map(toMemberOption)
}

// ============================================================================
// Token Actions
// ============================================================================

export interface OrganizationTokenBatchDeletePlan {
  selectedCount: number
  unauthorizedCount: number
  request: { ids: number[] } | null
}

/**
 * Batch deletion is all-or-nothing: if any selected key is outside the caller's
 * authority the whole request is withheld, so the UI can ask the user to drop
 * the offending rows instead of half-deleting.
 *
 * Generic in the row type so a caller holding full key rows can pass its own
 * predicate — a predicate that reads only `id` would otherwise force the caller
 * to widen its safety check down to the id.
 */
export function getOrganizationTokenBatchDeletePlan<T extends { id: number }>(
  selectedKeys: T[] = [],
  canDeleteToken: (token: T) => boolean = () => true
): OrganizationTokenBatchDeletePlan {
  const ids = selectedKeys.map((token) => token.id)
  const unauthorizedCount = selectedKeys.filter(
    (token) => !canDeleteToken(token)
  ).length

  return {
    selectedCount: ids.length,
    unauthorizedCount,
    request: ids.length > 0 && unauthorizedCount === 0 ? { ids } : null,
  }
}
