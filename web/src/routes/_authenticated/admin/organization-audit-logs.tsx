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

import { PlatformOrganizationAuditLog } from '@/features/organization-admin'
import { platformOrganizationAuditSearchSchema } from '@/features/organization-admin/lib'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

/**
 * The platform's cross-organization audit trail.
 *
 * The read behind it is an administrator's, and the backend enforces the same
 * thing, but the page is guarded here too: a page whose every request would be
 * refused should not be reachable in the first place.
 */
export const Route = createFileRoute(
  '/_authenticated/admin/organization-audit-logs'
)({
  beforeLoad: () => {
    const { auth } = useAuthStore.getState()

    if (!auth.user || auth.user.role < ROLE.ADMIN) {
      throw redirect({
        to: '/403',
      })
    }
  },
  validateSearch: platformOrganizationAuditSearchSchema,
  component: PlatformOrganizationAuditLog,
})
