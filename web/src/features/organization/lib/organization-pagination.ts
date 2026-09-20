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
