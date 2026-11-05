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
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { getOrganizations } from '@/features/organization/api'

import { exportSelfUsage, getSelfUsage } from '../api'
import { UsagePage, type UsageSearch } from '../index'
import type { SelfUsageData } from '../types'

vi.mock('../api', () => ({
  getSelfUsage: vi.fn(),
  exportSelfUsage: vi.fn(),
}))
vi.mock('@/features/organization/api', () => ({ getOrganizations: vi.fn() }))
vi.mock('../components/usage-date-range-picker', () => ({
  UsageDateRangePicker: () => <div data-testid='date-range-picker' />,
}))
vi.mock('../components/usage-overview', () => ({
  UsageOverview: () => <div data-testid='usage-overview' />,
}))
vi.mock('../components/usage-charts', () => ({
  UsageCharts: () => <div data-testid='usage-charts' />,
}))
vi.mock('../components/usage-table', () => ({
  UsageTable: (props: {
    data: SelfUsageData
    onExport: (granularity: 'day' | 'hour') => void
  }) => (
    <div data-testid='usage-table'>
      {props.data.rows.items[0]?.model_name}
      <button type='button' onClick={() => props.onExport('hour')}>
        export-hour
      </button>
    </div>
  ),
}))
const exportFileMock = vi.hoisted(() => ({
  buildSelfUsageExportSheet: vi.fn(() => ({
    rows: [],
    columns: [],
    filename: 'usage.xlsx',
  })),
  writeSelfUsageExportFile: vi.fn(),
}))
vi.mock('../lib/usage-export', () => exportFileMock)

const getSelfUsageMock = vi.mocked(getSelfUsage)
const exportSelfUsageMock = vi.mocked(exportSelfUsage)
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
    rows: {
      page: 1,
      page_size: 20,
      total: 1,
      items: [
        {
          date: '2026-11-03',
          type: 'text',
          billing_account_type: 'personal',
          organization_id: 0,
          organization_name: '',
          token_id: 1,
          token_name: 'k1',
          model_name: 'gpt-4o',
          request_count: 1,
          success_count: 1,
          failure_count: 0,
          prompt_tokens: 0,
          completion_tokens: 0,
          cache_tokens: 0,
          quota: 0,
        },
      ],
    },
    totals: {
      request_count: 0,
      success_count: 0,
      failure_count: 0,
      prompt_tokens: 0,
      completion_tokens: 0,
      cache_tokens: 0,
      quota: 0,
    },
    charts: [],
    filter_options: { models: [], tokens: [] },
  }
}

function renderUsagePage(
  search: UsageSearch,
  organizations: { id: number; name: string }[] = []
) {
  getOrganizationsMock.mockResolvedValue({
    success: true,
    data: organizations as never,
  })
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
    expect(screen.queryByTestId('usage-charts')).not.toBeInTheDocument()
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
    expect(await screen.findByTestId('usage-charts')).toBeInTheDocument()
  })

  test('keeps the previous rows visible while the next range loads', async () => {
    getSelfUsageMock.mockResolvedValue({
      success: true,
      data: usageData(true),
    })

    const page = renderUsagePage({ start: '2026-11-03', end: '2026-11-03' })
    const table = await screen.findByTestId('usage-table')
    expect(table).toHaveTextContent('gpt-4o')

    getSelfUsageMock.mockReturnValue(new Promise(() => undefined))
    page.rerender({ start: '2026-11-02', end: '2026-11-02' })

    await waitFor(() => {
      expect(getSelfUsageMock).toHaveBeenCalledTimes(2)
    })
    expect(screen.getByTestId('usage-table')).toHaveTextContent('gpt-4o')
  })

  test('shows an error state instead of zero figures when the request fails', async () => {
    getSelfUsageMock.mockRejectedValue(new Error('boom'))

    renderUsagePage({ start: '2026-11-03', end: '2026-11-03' })

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Retry' })).toBeInTheDocument()
    expect(screen.queryByTestId('usage-overview')).not.toBeInTheDocument()
    expect(screen.queryByTestId('usage-charts')).not.toBeInTheDocument()
    expect(screen.queryByTestId('usage-table')).not.toBeInTheDocument()
  })

  test('falls back to a member organization when the URL id is unknown', async () => {
    getSelfUsageMock.mockResolvedValue({
      success: true,
      data: usageData(true),
    })

    renderUsagePage(
      {
        start: '2026-11-03',
        end: '2026-11-03',
        overviewOrganizationId: '999',
      },
      [{ id: 9, name: 'Acme' }]
    )

    await waitFor(() => {
      const calls = getSelfUsageMock.mock.calls
      const last = calls.at(-1)?.[0]
      expect(last?.overview_organization_id).toBe(9)
    })
    expect(
      getSelfUsageMock.mock.calls.some(
        (call) => call[0].overview_organization_id === 999
      )
    ).toBe(false)
  })

  test('exports the applied filters at the chosen granularity', async () => {
    getSelfUsageMock.mockResolvedValue({
      success: true,
      data: usageData(true),
    })
    exportSelfUsageMock.mockResolvedValue({
      success: true,
      data: {
        granularity: 'hour',
        rows: usageData(true).rows.items,
        total: 1,
        truncated: false,
      },
    })

    renderUsagePage(
      {
        start: '2026-11-03',
        end: '2026-11-03',
        model: 'gpt-4o',
        token: 'key-1',
        tokenId: '1',
        billingAccountType: 'organization',
        organizationId: '9',
      },
      [{ id: 9, name: 'Acme' }]
    )

    await userEvent.click(
      await screen.findByRole('button', { name: 'export-hour' })
    )
    await waitFor(() => expect(exportSelfUsageMock).toHaveBeenCalled())
    expect(exportSelfUsageMock).toHaveBeenCalledWith({
      start: '2026-11-03',
      end: '2026-11-03',
      granularity: 'hour',
      model_name: 'gpt-4o',
      token_name: 'key-1',
      token_id: 1,
      billing_account_type: 'organization',
      organization_id: 9,
    })
    await waitFor(() =>
      expect(exportFileMock.writeSelfUsageExportFile).toHaveBeenCalled()
    )
  })
})
