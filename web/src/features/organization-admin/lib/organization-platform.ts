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
import type {
  OrganizationManagementView,
  OrganizationStatus,
} from '@/features/organization/types'

/**
 * An organization as one of the platform actions sees it.
 *
 * Narrowed rather than typed as the whole `OrganizationManagementView` because
 * the list and the detail page carry different payloads for the same
 * organization: a list row adds the owner's name and the member and key counts,
 * while the detail page answers with the organization record itself. Both
 * satisfy this, so the dialogs work on either page without either one having to
 * invent fields it does not have.
 */
export interface PlatformOrganizationTarget {
  id: number
  name: string
  slug: string
  status: OrganizationStatus
}

/** The same organization, plus the fields the platform edit form reads. */
export interface PlatformOrganizationEditTarget
  extends PlatformOrganizationTarget {
  description?: string
  group?: string
  quota: number
  used_quota: number
}

/**
 * The statuses the platform list filters on.
 *
 * All three, including `dissolved`. The organization center's own list leaves
 * that status out because the backend never returns a dissolved organization
 * there, but the platform view is the only place a dissolved organization is
 * still reachable — a filter that omitted it would hide them from the one
 * surface that can still see them.
 */
export const PLATFORM_ORGANIZATION_STATUSES = [
  'active',
  'disabled',
  'dissolved',
] as const

/**
 * The `status` parameter the platform list endpoint reads.
 *
 * The endpoint takes a comma-joined selection and treats an empty one as every
 * status, which is what makes "no filter chosen" and "all three chosen" the same
 * request — and they should be, since they mean the same thing.
 */
export function platformOrganizationStatusParam(
  statuses: readonly string[] | undefined
): string {
  return (statuses ?? []).filter(Boolean).join(',')
}

/**
 * What the organization has left to spend.
 *
 * `quota` is the grant an operator has made and `used_quota` the part already
 * spent against it. The subtraction is clamped at zero because an organization
 * whose grant was lowered below what it already spent has spent everything it
 * has, not a negative amount — and a negative remaining quota would be offered
 * back as a top-up target.
 */
export function platformOrganizationRemainingQuota(
  organization: Pick<OrganizationManagementView, 'quota' | 'used_quota'>
): number {
  const grant = Number(organization.quota ?? 0)
  const used = Number(organization.used_quota ?? 0)
  return Math.max(grant - used, 0)
}

/**
 * The signed adjustment that would leave the organization at `nextRemaining`.
 *
 * The edit form edits the remaining quota, which is what an operator reasons
 * about ("give them another $50"), while the endpoint takes a signed delta
 * against the grant. The conversion has to account for what has already been
 * spent: raising a grant that is half spent by the amount requested would leave
 * the organization with that amount *plus* what it spent.
 *
 * Truncated to an integer because quota is counted in whole units on the
 * backend and a fractional delta would be rejected there.
 */
export function platformOrganizationQuotaDelta(
  organization: Pick<OrganizationManagementView, 'quota' | 'used_quota'>,
  nextRemainingQuota: number
): number {
  const grant = Number(organization.quota ?? 0)
  const used = Number(organization.used_quota ?? 0)
  const nextRemaining = Math.max(Math.trunc(Number(nextRemainingQuota) || 0), 0)
  return nextRemaining + used - grant
}

export interface PlatformOrganizationActionFlags {
  /** The organization is dissolved; nothing about it can change any more. */
  readOnly: boolean
  canEdit: boolean
  canDisable: boolean
  canEnable: boolean
  canAdjustQuota: boolean
  /**
   * Dissolving is restricted to the platform's root role on the backend
   * (`capabilitiesForPlatformRole` grants `dissolve_organization` only to
   * `platform_root`), so an ordinary administrator is not offered a control
   * that would be refused.
   */
  canDissolve: boolean
}

/**
 * Which platform actions this organization may still receive.
 *
 * Derived from the organization's status and the caller's platform role rather
 * than from a capability set: the platform list endpoint is gated by
 * `AdminAuth` alone and answers no per-organization capabilities, because the
 * caller is not a member of the organizations it lists.
 */
export function getPlatformOrganizationActionFlags(
  organization: Pick<OrganizationManagementView, 'status'>,
  options: { isRoot: boolean }
): PlatformOrganizationActionFlags {
  const readOnly = organization.status === 'dissolved'

  return {
    readOnly,
    canEdit: !readOnly,
    canDisable: !readOnly && organization.status === 'active',
    canEnable: !readOnly && organization.status === 'disabled',
    canAdjustQuota: !readOnly,
    canDissolve: Boolean(options.isRoot) && !readOnly,
  }
}

/**
 * Whether the platform list row is a torn-down organization.
 *
 * The row is styled as inactive rather than hidden: a dissolved organization is
 * still the record of what happened to its members and keys, and the audit trail
 * behind it has to stay reachable.
 */
export function isPlatformOrganizationInactive(
  organization: Pick<OrganizationManagementView, 'status'>
): boolean {
  return organization.status !== 'active'
}
