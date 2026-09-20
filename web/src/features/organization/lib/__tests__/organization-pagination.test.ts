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
import { describe, expect, test } from 'vitest'

import {
  collectOrganizationRows,
  emptyPagedResult,
  ORGANIZATION_EXPORT_MAX_ROWS,
  ORGANIZATION_MAX_PAGE_SIZE,
  organizationPageParams,
  type PagedResult,
} from '../organization-pagination'

/**
 * A stand-in endpoint holding `total` rows, answering at most `cap` per page
 * the way the backend does — silently, without saying it capped anything.
 */
function fakeEndpoint(total: number, cap = ORGANIZATION_MAX_PAGE_SIZE) {
  const calls: Array<{ page: number; pageSize: number }> = []
  const fetchPage = async (
    page: number,
    pageSize: number
  ): Promise<PagedResult<number>> => {
    calls.push({ page, pageSize })
    const size = Math.min(pageSize, cap)
    const start = (page - 1) * size
    const items: number[] = []
    for (let index = start; index < Math.min(start + size, total); index += 1) {
      items.push(index)
    }
    return { page, page_size: size, total, items }
  }
  return { fetchPage, calls }
}

describe('page parameters', () => {
  test('the backend spells the page p, not page', () => {
    expect(organizationPageParams(3, 20)).toEqual({ p: 3, page_size: 20 })
  })

  test('a missing envelope reads as an empty page', () => {
    expect(emptyPagedResult<number>(50)).toEqual({
      page: 1,
      page_size: 50,
      total: 0,
      items: [],
    })
  })
})

describe('collectOrganizationRows', () => {
  test('an empty set is one request and no rows', async () => {
    const endpoint = fakeEndpoint(0)
    const result = await collectOrganizationRows(endpoint.fetchPage)

    expect(result).toEqual({ items: [], total: 0, truncated: false })
    expect(endpoint.calls).toHaveLength(1)
  })

  test('a set smaller than one page is one request', async () => {
    const endpoint = fakeEndpoint(3)
    const result = await collectOrganizationRows(endpoint.fetchPage)

    expect(result.items).toEqual([0, 1, 2])
    expect(result.truncated).toBe(false)
    expect(endpoint.calls).toHaveLength(1)
  })

  test('a set that exactly fills a page is not fetched twice', async () => {
    const endpoint = fakeEndpoint(100)
    const result = await collectOrganizationRows(endpoint.fetchPage)

    expect(result.items).toHaveLength(100)
    expect(result.truncated).toBe(false)
    expect(endpoint.calls).toHaveLength(1)
  })

  test('a larger set is assembled in order across pages', async () => {
    const endpoint = fakeEndpoint(250)
    const result = await collectOrganizationRows(endpoint.fetchPage)

    expect(result.items).toHaveLength(250)
    expect(result.items.at(0)).toBe(0)
    expect(result.items.at(-1)).toBe(249)
    expect(result.truncated).toBe(false)
    expect(endpoint.calls.map((call) => call.page)).toEqual([1, 2, 3])
  })

  test('the walk stops at the row cap and says it did', async () => {
    const endpoint = fakeEndpoint(ORGANIZATION_EXPORT_MAX_ROWS + 500)
    const result = await collectOrganizationRows(endpoint.fetchPage)

    expect(result.items).toHaveLength(ORGANIZATION_EXPORT_MAX_ROWS)
    expect(result.total).toBe(ORGANIZATION_EXPORT_MAX_ROWS + 500)
    expect(result.truncated).toBe(true)
    expect(endpoint.calls).toHaveLength(ORGANIZATION_EXPORT_MAX_ROWS / 100)
  })

  test('a page short of the requested size ends the walk', async () => {
    // The endpoint caps at 5 per page whatever is asked for, so the walk ends on
    // the first short page rather than asking for pages that cannot exist.
    const endpoint = fakeEndpoint(12, 5)
    const result = await collectOrganizationRows(endpoint.fetchPage, {
      pageSize: 5,
      maxRows: 100,
    })

    expect(result.items).toHaveLength(12)
    expect(result.truncated).toBe(false)
    expect(endpoint.calls).toHaveLength(3)
  })

  test('a wrongly capped page does not become an endless walk', async () => {
    // The backend clamps page_size to 100, so asking for more is answered with
    // 100 rows. The walk has to keep going, and it must still terminate.
    const endpoint = fakeEndpoint(250)
    const result = await collectOrganizationRows(endpoint.fetchPage, {
      pageSize: 1000,
      maxRows: 1000,
    })

    expect(result.items).toHaveLength(250)
    expect(result.truncated).toBe(false)
    expect(endpoint.calls.map((call) => call.page)).toEqual([1, 2, 3])
  })

  test('an endpoint that reports no total still terminates', async () => {
    // No count to compare against, so only the short page ends the walk — which
    // it does, on the page that comes back empty.
    const fetchPage = async (page: number, pageSize: number) => {
      const size = Math.min(pageSize, 100)
      const items = page === 1 ? Array.from({ length: size }, (_, i) => i) : []
      return { page, page_size: size, total: 0, items }
    }
    const result = await collectOrganizationRows(fetchPage, { maxRows: 1000 })

    expect(result.items).toHaveLength(100)
    expect(result.truncated).toBe(false)
  })
})
