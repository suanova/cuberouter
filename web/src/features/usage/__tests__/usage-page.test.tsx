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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { getOrganizations } from '@/features/organization/api'

import { getSelfUsage } from '../api'
import { UsagePage, type UsageSearch } from '../index'
import type { SelfUsageData } from '../types'

vi.mock('../api', () => ({ getSelfUsage: vi.fn() }))
vi.mock('@/features/organization/api', () => ({ getOrganizations: vi.fn() }))
vi.mock('../components/usage-date-range-picker', () => ({
  UsageDateRangePicker: () => <div data-testid='date-range-picker' />,
}))
vi.mock('../components/usage-overview', () => ({
  UsageOverview: () => <div data-testid='usage-overview' />,
}))
vi.mock('../components/usage-table', () => ({
  UsageTable: () => <div data-testid='usage-table' />,
}))

const getSelfUsageMock = vi.mocked(getSelfUsage)
const getOrganizationsMock = vi.mocked(getOrganizations)

function usageData(backfillComplete: boolean): SelfUsageData {
  const stat = {
    request_count: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    cache_tokens: 0,
    quota: 0,
    remaining_quota: 0,
  }
  return {
    backfill_complete: backfillComplete,
    range: {
      start: '2026-11-03',
      end: '2026-11-03',
      start_timestamp: 0,
      end_timestamp: 0,
    },
    overview: { personal: stat, organization: null },
    rows: { page: 1, page_size: 20, total: 0, items: [] },
    totals: {
      request_count: 0,
      success_count: 0,
      failure_count: 0,
      prompt_tokens: 0,
      completion_tokens: 0,
      cache_tokens: 0,
      quota: 0,
    },
    filter_options: { models: [], tokens: [] },
  }
}

function renderUsagePage(search: UsageSearch) {
  getOrganizationsMock.mockResolvedValue({ success: true, data: [] })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const ui = (nextSearch: UsageSearch) => (
    <QueryClientProvider client={client}>
      <UsagePage search={nextSearch} onSearchChange={() => undefined} />
    </QueryClientProvider>
  )
  const rendered = render(ui(search))
  return {
    rerender: (nextSearch: UsageSearch) => rendered.rerender(ui(nextSearch)),
  }
}

describe('UsagePage', () => {
  test('shows only the preparation notice until the history backfill completes', async () => {
    getSelfUsageMock.mockResolvedValue({
      success: true,
      data: usageData(false),
    })

    renderUsagePage({ start: '2026-11-03', end: '2026-11-03' })

    expect(
      await screen.findByText('History is still being prepared.')
    ).toBeInTheDocument()
    expect(screen.queryByTestId('usage-overview')).not.toBeInTheDocument()
    expect(screen.queryByTestId('usage-table')).not.toBeInTheDocument()
  })

  test('renders usage content accessibly once the history backfill completes', async () => {
    getSelfUsageMock.mockResolvedValue({
      success: true,
      data: usageData(true),
    })

    renderUsagePage({ start: '2026-11-03', end: '2026-11-03' })

    const table = await screen.findByTestId('usage-table')
    expect(
      screen.queryByText('History is still being prepared.')
    ).not.toBeInTheDocument()
    expect(table.closest('[aria-hidden="true"]')).toBeNull()
    expect(screen.getByTestId('usage-overview')).toBeInTheDocument()
  })

  test('keeps the previous rows visible while the next range loads', async () => {
    getSelfUsageMock.mockResolvedValue({
      success: true,
      data: usageData(true),
    })

    const page = renderUsagePage({ start: '2026-11-03', end: '2026-11-03' })
    await screen.findByTestId('usage-table')

    getSelfUsageMock.mockReturnValue(new Promise(() => undefined))
    page.rerender({ start: '2026-11-02', end: '2026-11-02' })

    await waitFor(() => {
      expect(getSelfUsageMock).toHaveBeenCalledTimes(2)
    })
    expect(screen.getByTestId('usage-table')).toBeInTheDocument()
  })
})
