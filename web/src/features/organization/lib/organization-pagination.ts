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

/** Rows per page for the organization tables. Mirrors the backend default. */
export const ORGANIZATION_DEFAULT_PAGE_SIZE = 10

export const ORGANIZATION_PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const

/**
 * The pagination envelope every organization list endpoint returns.
 *
 * The backend fills it through `common.PageInfo.SetItems`, so the keys are the
 * same across members, invitations, tokens, logs, tasks, audit logs and billing
 * records — only `items` differs.
 */
export interface PagedResult<T> {
  page: number
  page_size: number
  total: number
  items: T[]
}

/**
 * Query parameters for a paged endpoint.
 *
 * The page number is `p`, not `page`, and the size is `page_size` — that is
 * what `common.GetPageQuery` reads, and it is not the same spelling the
 * frontend uses internally.
 */
export function organizationPageParams(
  page: number,
  pageSize: number
): { p: number; page_size: number } {
  return { p: page, page_size: pageSize }
}

/** An endpoint that answered without a page envelope is treated as empty. */
export function emptyPagedResult<T>(
  pageSize: number = ORGANIZATION_DEFAULT_PAGE_SIZE
): PagedResult<T> {
  return { page: 1, page_size: pageSize, total: 0, items: [] }
}

/**
 * The largest page the backend will answer with.
 *
 * `common.GetPageQuery` clamps `page_size` to 100 and does so silently, so a
 * request for more is answered with 100 rows and no indication that the rest
 * were left out.
 */
export const ORGANIZATION_MAX_PAGE_SIZE = 100

/**
 * How many rows an export may assemble, and how many are asked for at a time.
 *
 * The cap is a bound on requests rather than on the data: an export that needs
 * more than this many pages reports what it managed instead of holding the tab
 * open indefinitely.
 */
export const ORGANIZATION_EXPORT_MAX_ROWS = 5000

export interface CollectedOrganizationRows<T> {
  items: T[]
  /** The endpoint's own count of the whole filtered set. */
  total: number
  /** The set was larger than the cap, so `items` is a leading slice of it. */
  truncated: boolean
}

/**
 * Reads a whole filtered set, one page at a time.
 *
 * A single request cannot do this: `page_size` is clamped to 100 by the backend,
 * so an export of a busy month has to be assembled. The walk stops on the first
 * page that comes back shorter than the page it was asked for — measured against
 * the size the response reports rather than the size requested, because the
 * backend answers a request above its cap with a shorter page rather than an
 * error, and reading that as the end of the data would export a hundred rows of
 * a set that holds thousands.
 */
export async function collectOrganizationRows<T>(
  fetchPage: (page: number, pageSize: number) => Promise<PagedResult<T>>,
  options: { pageSize?: number; maxRows?: number } = {}
): Promise<CollectedOrganizationRows<T>> {
  const pageSize = Math.min(
    options.pageSize ?? ORGANIZATION_MAX_PAGE_SIZE,
    ORGANIZATION_MAX_PAGE_SIZE
  )
  const maxRows = options.maxRows ?? ORGANIZATION_EXPORT_MAX_ROWS
  const items: T[] = []
  let total = 0
  let page = 1

  while (items.length < maxRows) {
    const result = await fetchPage(page, pageSize)
    total = result.total ?? 0
    const batch = result.items ?? []
    items.push(...batch.slice(0, maxRows - items.length))
    const answered = result.page_size > 0 ? result.page_size : pageSize
    if (batch.length < answered) break
    // A set that the last page happened to fill completely; the count is what
    // says so, and it is only trusted when the endpoint actually reported one.
    if (total > 0 && items.length >= total) break
    page += 1
  }

  return { items, total, truncated: total > items.length }
}
