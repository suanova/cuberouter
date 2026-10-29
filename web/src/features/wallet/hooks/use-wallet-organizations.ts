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

import { getOrganizations } from '@/features/organization/api'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

import type { WalletOrganization } from '../types'

/**
 * The organizations whose balances the wallet can show.
 *
 * `GET /api/organizations` already carries both the organization pool and the
 * signed-in member's own quota figures, so one request covers the whole card.
 * The query key includes the account context because the backend resolves the
 * rows against it.
 */
export function useWalletOrganizations() {
  return useQuery({
    queryKey: ['wallet-organizations', getAccountContextCacheKey()] as const,
    queryFn: async (): Promise<WalletOrganization[]> => {
      const result = await getOrganizations()
      if (!result.success) {
        return []
      }
      return (result.data ?? []).map((organization) => ({
        id: organization.id,
        name: organization.name,
        quota: organization.quota,
        member_quota_limit: organization.member_quota_limit,
        member_used_quota: organization.member_used_quota,
      }))
    },
  })
}
