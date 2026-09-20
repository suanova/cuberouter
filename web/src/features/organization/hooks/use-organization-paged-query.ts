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

import type { PagedResult } from '../lib'
import type { OrganizationApiResponse } from '../types'

/** The HTTP status the backend uses when the caller may not read this section. */
const FORBIDDEN = 403

function statusOf(error: unknown): number | undefined {
  return (error as { response?: { status?: number } })?.response?.status
}

type SectionQueryOptions<T> = {
  organizationId: number
  /** Which list this is, so two sections never share a cache entry. */
  resource: string
  /** Everything that changes the answer, besides the organization and context. */
  params: Record<string, unknown>
  query: () => Promise<OrganizationApiResponse<T>>
  enabled?: boolean
  /**
   * Called when the backend answers 403.
   *
   * A section the backend refuses mid-session means the caller's access changed
   * while the page was open — they were removed, or the organization was
   * dissolved. The page can no longer show anything true, so it hands control
   * back to the caller of this hook to leave.
   */
  onForbidden?: () => void
}

type SectionQueryResult<T> = {
  data: T | undefined
  isLoading: boolean
  isFetching: boolean
  refetch: () => void
}

/**
 * Reads one organization section.
 *
 * Failures resolve to `undefined` rather than throwing: every one of them has
 * already been reported by the HTTP interceptor, and a section that cannot load
 * should show its empty state instead of tearing down the whole page. The one
 * that needs a decision here is the 403, which is not an error to display but a
 * signal that the page is stale.
 */
function useOrganizationSectionQuery<T>(
  options: SectionQueryOptions<T>
): SectionQueryResult<T> {
  const { data, isLoading, isFetching, refetch } = useQuery({
    // The payload is resolved against the account context, and React Query
    // cannot tell one context from another, so the context is part of the key.
    queryKey: [
      'organization-section',
      options.resource,
      options.organizationId,
      getAccountContextCacheKey(),
      options.params,
    ],
    queryFn: async (): Promise<T | undefined> => {
      try {
        const result = await options.query()
        return result.success ? result.data : undefined
      } catch (error) {
        if (statusOf(error) === FORBIDDEN) {
          options.onForbidden?.()
          return undefined
        }
        // Not forbidden, so the interceptor has already shown the reason and the
        // section degrades to its empty state.
        return undefined
      }
    },
    enabled: options.enabled ?? true,
    placeholderData: (previousData) => previousData,
  })

  return {
    data,
    isLoading,
    isFetching,
    refetch: () => {
      void refetch()
    },
  }
}

type PagedSectionOptions<T> = Omit<
  SectionQueryOptions<PagedResult<T>>,
  'query'
> & {
  query: () => Promise<OrganizationApiResponse<PagedResult<T>>>
}

/**
 * A paged section, with the envelope already unwrapped.
 *
 * An absent or malformed envelope reads as an empty page rather than as a
 * loading state forever — a list that answers without `items` has nothing to
 * show, and saying so is more useful than spinning.
 */
export function useOrganizationPagedSection<T>(
  options: PagedSectionOptions<T>
): {
  items: T[]
  total: number
  isLoading: boolean
  isFetching: boolean
  refetch: () => void
} {
  const result = useOrganizationSectionQuery<PagedResult<T>>(options)
  const page = result.data

  return {
    items: page?.items ?? [],
    total: page?.total ?? 0,
    isLoading: result.isLoading,
    isFetching: result.isFetching,
    refetch: result.refetch,
  }
}

/** A section endpoint that answers with one object instead of a page. */
export function useOrganizationResourceSection<T>(
  options: SectionQueryOptions<T>
): SectionQueryResult<T> {
  return useOrganizationSectionQuery(options)
}
