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

import { listOrganizationMembers } from '../api'
import {
  getOrganizationTokenResponsibleOptions,
  getOrganizationTransferMemberOptions,
  type OrganizationMemberOption,
} from '../lib'
import type { OrganizationMemberRow } from '../types'

/** The page size the exhaustive walk uses; the endpoint caps what it returns. */
const PAGE_SIZE = 100

/** A hard stop, so a server that keeps answering cannot loop forever. */
const MAX_PAGES = 20

/**
 * Every member of the organization, however many pages that takes.
 *
 * The transfer pickers need the whole roster, not the page on screen: offering
 * only the members currently visible would silently hide a valid target, and
 * the choice is about who takes over keys, which has nothing to do with where
 * they happen to sort.
 */
async function listAllMembers(organizationId: number): Promise<OrganizationMemberRow[]> {
  const collected: OrganizationMemberRow[] = []
  let total = Number.POSITIVE_INFINITY

  for (let page = 1; page <= MAX_PAGES && collected.length < total; page += 1) {
    const result = await listOrganizationMembers(organizationId, {
      p: page,
      page_size: PAGE_SIZE,
    })
    if (!result.success) break
    const items = result.data?.items ?? []
    collected.push(...items)
    total = result.data?.total ?? collected.length
    // An empty page means the server has nothing more, whatever it claimed.
    if (items.length === 0) break
  }

  return collected
}

/**
 * The whole roster, for the pickers that need to offer members.
 *
 * Fetched lazily — `enabled` is driven by whether a dialog is open — because
 * most visits to a list never open one. Shared between the pickers so opening
 * two of them does not walk the member list twice.
 */
function useOrganizationRoster(
  organizationId: number,
  enabled: boolean
): { members: OrganizationMemberRow[]; isLoading: boolean } {
  const { data, isLoading } = useQuery({
    queryKey: [
      'organization-member-options',
      organizationId,
      getAccountContextCacheKey(),
    ],
    queryFn: async () => {
      try {
        return await listAllMembers(organizationId)
      } catch {
        // The interceptor has already reported it; an empty picker is the right
        // degradation, and the dialog stays usable without a target.
        return []
      }
    },
    enabled,
  })

  return { members: data ?? [], isLoading }
}

/** The members who could take over an administrator's keys. */
export function useOrganizationMemberOptions(
  organizationId: number,
  excludedUserId: number | undefined,
  enabled: boolean
): { options: OrganizationMemberOption[]; isLoading: boolean } {
  const { members, isLoading } = useOrganizationRoster(organizationId, enabled)

  return {
    options: getOrganizationTransferMemberOptions(members, excludedUserId),
    isLoading,
  }
}

/**
 * The members who could hold an organization key.
 *
 * Narrowed by the key's visibility, so switching a key to public drops the
 * members who cannot hold one before they can be chosen.
 */
export function useOrganizationTokenResponsibleOptions(
  organizationId: number,
  visibility: string,
  enabled: boolean
): { options: OrganizationMemberOption[]; isLoading: boolean } {
  const { members, isLoading } = useOrganizationRoster(organizationId, enabled)

  return {
    options: getOrganizationTokenResponsibleOptions(members, visibility),
    isLoading,
  }
}
