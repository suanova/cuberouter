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
import { createFileRoute, redirect } from '@tanstack/react-router'

import { PlatformOrganizationDetail } from '@/features/organization-admin'
import { normalizePlatformOrganizationTabKey } from '@/features/organization-admin/lib'
import { organizationDetailSearchSchema } from '@/features/organization/lib'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

/**
 * One organization, from the platform side.
 *
 * The sections read the same query parameters as the organization center's own
 * detail page — the search schema is the same module, because the endpoints
 * behind them take the same filters — but the page reads them through
 * `/api/admin/organizations`.
 *
 * There is deliberately no account-context alignment here. The member page has
 * to adopt the organization's context before its reads work; this one must not,
 * because an administrator has no context in an organization they do not belong
 * to and the admin endpoints resolve the caller from their own identity.
 */
export const Route = createFileRoute(
  '/_authenticated/admin/organizations/$organizationId/$section'
)({
  beforeLoad: ({ params }) => {
    const { auth } = useAuthStore.getState()

    // The backend gates this whole surface with AdminAuth; refusing the page
    // early keeps an unauthorized deep link from rendering a shell that would
    // only fill with refused requests.
    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }

    // An unknown section, or one of the legacy aliases, resolves to its
    // canonical tab rather than to a 404 — the URL is user-editable and older
    // links are still around.
    const canonical = normalizePlatformOrganizationTabKey(params.section)
    if (canonical !== params.section) {
      throw redirect({
        to: '/admin/organizations/$organizationId/$section',
        params: { organizationId: params.organizationId, section: canonical },
        replace: true,
      })
    }
  },
  validateSearch: organizationDetailSearchSchema,
  component: PlatformOrganizationDetail,
})
