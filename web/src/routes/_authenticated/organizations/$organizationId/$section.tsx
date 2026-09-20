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

import { OrganizationDetail } from '@/features/organization'
import {
  alignOrganizationAccountContext,
  normalizeOrganizationTabKey,
  organizationDetailSearchSchema,
} from '@/features/organization/lib'

export const Route = createFileRoute(
  '/_authenticated/organizations/$organizationId/$section'
)({
  validateSearch: organizationDetailSearchSchema,
  beforeLoad: async ({ params, context }) => {
    // An unknown section, or one of the legacy aliases, resolves to its
    // canonical tab rather than to a 404 — the URL is user-editable and older
    // links are still around.
    const canonical = normalizeOrganizationTabKey(params.section)
    if (canonical !== params.section) {
      throw redirect({
        to: '/organizations/$organizationId/$section',
        params: { organizationId: params.organizationId, section: canonical },
        replace: true,
      })
    }

    // The organization's own read endpoints require the request to carry its
    // account context, so a deep link has to adopt it before the page loads.
    await alignOrganizationAccountContext(
      Number(params.organizationId),
      context.queryClient
    )
  },
  component: OrganizationDetail,
})
