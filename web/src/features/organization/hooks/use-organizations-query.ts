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
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { getAccountContextCacheKey } from '@/stores/account-context-store'

import { getOrganizations } from '../api'
import { ERROR_MESSAGES } from '../constants'
import type { UserOrganization } from '../types'

/**
 * The caller's organizations, shared by the list page and its table so both read
 * one cache entry. `GET /api/organizations` returns every organization the user
 * belongs to — including the ones they did not create — carrying the
 * capabilities the backend already resolved for them.
 */
export function useOrganizationsQuery(refreshTrigger: number) {
  const { t } = useTranslation()

  return useQuery({
    // The rows are resolved from the account context, and React Query cannot
    // tell one context from another, so the context belongs in the key.
    queryKey: [
      'organizations',
      getAccountContextCacheKey(),
      refreshTrigger,
    ] as const,
    queryFn: async (): Promise<UserOrganization[]> => {
      const result = await getOrganizations()
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.LOAD_FAILED))
        return []
      }
      return result.data ?? []
    },
    placeholderData: (previousData) => previousData,
  })
}
