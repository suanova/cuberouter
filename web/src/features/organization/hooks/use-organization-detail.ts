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
import { useQuery } from '@tanstack/react-query'

import { getAccountContextCacheKey } from '@/stores/account-context-store'

import { getOrganization } from '../api'
import type { OrganizationDetail } from '../types'

/**
 * The organization as this caller sees it, or `null` when it cannot be loaded —
 * the API error interceptor has already reported why.
 *
 * The payload is resolved against the account context, so the context is part of
 * the identity of the cached value.
 */
export function useOrganizationDetail(
  organizationId: string | number | undefined
): {
  detail: OrganizationDetail | null | undefined
  /** Re-read the payload, for a write that changed something the page shows. */
  refetch: () => Promise<unknown>
} {
  const id = Number(organizationId)
  const validId = Number.isFinite(id) && id > 0

  const { data, refetch } = useQuery({
    queryKey: ['organization', id, getAccountContextCacheKey()],
    queryFn: async (): Promise<OrganizationDetail | null> => {
      const result = await getOrganization(id)
      return result.success && result.data ? result.data : null
    },
    enabled: validId,
  })

  // A disabled query never resolves, so an unparseable id reports as
  // unavailable instead of loading forever.
  return { detail: validId ? data : null, refetch }
}
