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

/**
 * Which side of the organization API a page is reading.
 *
 * The backend publishes one organization twice. The member surface
 * (`/api/organizations/:id`) is gated by the caller's own membership and reads
 * the caller's account context; the platform admin surface
 * (`/api/admin/organizations/:id`) is gated by `AdminAuth` and lets an
 * administrator inspect an organization they need not belong to.
 *
 * Both resolve to the same handlers and answer the same shapes, so a page that
 * renders an organization is written once and told which surface it is on. The
 * prefix below is the whole of the difference; everything else — the routes,
 * the capability names, the payloads — is shared.
 */
export const ORGANIZATION_SURFACES = ['member', 'admin'] as const

export type OrganizationSurface = (typeof ORGANIZATION_SURFACES)[number]

/** What a page renders on when no provider names a surface. */
export const DEFAULT_ORGANIZATION_SURFACE: OrganizationSurface = 'member'

/**
 * The root one organization's resources hang off.
 *
 * Not every resource exists on both surfaces — an administrator cannot send an
 * invitation and a member cannot adjust the quota — but every resource that
 * does exist sits at the same path under whichever root applies.
 */
export function organizationResourceBase(
  surface: OrganizationSurface,
  organizationId: number
): string {
  const root =
    surface === 'admin' ? '/api/admin/organizations' : '/api/organizations'
  return `${root}/${organizationId}`
}
