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
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback } from 'react'

import { switchAccountContext } from '@/lib/account-context'

/**
 * Opens an organization, selecting it as the account context first when the
 * caller may use it as one.
 *
 * The switch is not cosmetic: the read endpoints compare the request's context
 * against the organization id in the path and reject a mismatch, so opening
 * another organization without switching would 403 its very first request. When
 * the organization cannot be a context, the read endpoints skip that check, so
 * browsing it without switching is exactly right.
 */
export function useEnterOrganization() {
  const navigate = useNavigate()
  const queryClient = useQueryClient()

  return useCallback(
    async (organizationId: number, switchContext: boolean) => {
      if (switchContext) {
        await switchAccountContext('organization', organizationId)
        // Everything cached was fetched under the previous context. React Query
        // has no way to tell the two apart, so the whole cache is refetched.
        await queryClient.invalidateQueries()
      }

      await navigate({
        to: '/organizations/$organizationId/$section',
        params: { organizationId: String(organizationId), section: 'overview' },
      })
    },
    [navigate, queryClient]
  )
}
