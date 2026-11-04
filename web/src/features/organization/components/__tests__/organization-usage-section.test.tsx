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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { exportOrganizationUsage, getOrganizationUsage } from '../../api'
import type { OrganizationUsageData } from '../../types'
import { OrganizationUsageSection } from '../organization-usage-section'

// The URL is the page's state, not this section's; the harness supplies the
// search params instead of standing up a route.
const pageRoute = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
}))

vi.mock('../organization-page-provider', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../organization-page-provider')>()),
  useOrganizationSectionRoute: () => pageRoute,
  useOrganizationSurface: () => 'member',
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  getOrganizationUsage: vi.fn(),
  exportOrganizationUsage: vi.fn(),
}))

const exportMock = vi.hoisted(() => ({
  buildSelfUsageExportSheet: vi.fn(() => ({
    rows: [],
    columns: [],
    filename: 'organization-usage-day.xlsx',
  })),
  writeSelfUsageExportFile: vi.fn(),
}))

vi.mock('@/features/usage/lib/usage-export', () => exportMock)

const getUsageMock = vi.mocked(getOrganizationUsage)
const exportUsageMock = vi.mocked(exportOrganizationUsage)

const ORGANIZATION_ID = 7

function usageData(
  overrides: Partial<OrganizationUsageData> = {}
): OrganizationUsageData {
  return {
    backfill_complete: true,
    range: {
      start: '2026-11-03',
      end: '2026-11-03',
      start_timestamp: 0,
      end_timestamp: 0,
    },
    rows: {
      page: 1,
      page_size: 20,
      total: 1,
      items: [
        {
          date: '2026-11-03',
          type: 'text',
          billing_account_type: 'organization',
          organization_id: ORGANIZATION_ID,
          organization_name: 'Acme',
          responsible_user_id: 42,
          responsible_name: 'Alice',
          token_id: 1,
          token_name: 'team-key',
          model_name: 'gpt-4o',
          request_count: 2,
          success_count: 2,
          failure_count: 0,
          prompt_tokens: 100,
          completion_tokens: 50,
          cache_tokens: 10,
          quota: 800_000,
        },
      ],
    },
    totals: {
      request_count: 2,
      success_count: 2,
      failure_count: 0,
      prompt_tokens: 100,
      completion_tokens: 50,
      cache_tokens: 10,
      quota: 800_000,
    },
    filter_options: {
      models: ['gpt-4o'],
      tokens: [
        {
          id: 1,
          name: 'team-key',
          organization_id: ORGANIZATION_ID,
          organization_name: 'Acme',
        },
      ],
      users: [
        { id: 42, name: 'Alice' },
        { id: 43, name: 'Bob' },
      ],
    },
    ...overrides,
  }
}

function renderSection(): void {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <OrganizationUsageSection
        organizationId={ORGANIZATION_ID}
        canView
        onForbidden={() => undefined}
      />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  pageRoute.search = {
    usageStart: '2026-11-03',
    usageEnd: '2026-11-03',
  }
  pageRoute.navigate.mockReset()
  getUsageMock.mockReset().mockResolvedValue({
    success: true,
    data: usageData(),
  })
  exportUsageMock.mockReset().mockResolvedValue({
    success: true,
    data: {
      granularity: 'day',
      rows: usageData().rows.items,
      total: 1,
      truncated: false,
    },
  })
  exportMock.buildSelfUsageExportSheet.mockClear()
  exportMock.writeSelfUsageExportFile.mockClear()
})

describe('organization usage section', () => {
  test('renders the same daily table as personal usage with a member column', async () => {
    renderSection()

    await waitFor(
      () => {
        expect(screen.getByText('Alice')).toBeInTheDocument()
      },
      { timeout: 5_000 }
    )

    for (const header of [
      'Date',
      'User',
      'Type',
      'API Key',
      'Model',
      'Requests',
      'Success',
      'Failed',
      'Input Tokens',
      'Output Tokens',
      'Cache Tokens',
      'Cost',
    ]) {
      expect(
        screen.getByRole('columnheader', { name: header })
      ).toBeInTheDocument()
    }

    for (const oldHeader of [
      'Upstream',
      'Billing Mode',
      'Original price',
      'Billing Account',
    ]) {
      expect(
        screen.queryByRole('columnheader', { name: oldHeader })
      ).not.toBeInTheDocument()
    }
    expect(screen.getByText('team-key')).toBeInTheDocument()
    expect(screen.getByText('gpt-4o')).toBeInTheDocument()
  })

  test('reads the day range and the selected filters from the URL', async () => {
    pageRoute.search = {
      usageStart: '2026-11-01',
      usageEnd: '2026-11-03',
      usageModel: 'gpt-4o',
      usageTokenId: '2',
      usageUserId: '42',
    }
    renderSection()

    await waitFor(
      () => {
        expect(getUsageMock).toHaveBeenCalled()
      },
      { timeout: 5_000 }
    )

    const params = getUsageMock.mock.calls[0]?.[2]
    expect(params?.start).toBe('2026-11-01')
    expect(params?.end).toBe('2026-11-03')
    expect(params?.model_name).toBe('gpt-4o')
    expect(params?.token_id).toBe(2)
    expect(params?.responsible_user_id).toBe(42)
    expect(params?.p).toBe(1)
  })

  test('exports the filtered table by day or by hour', async () => {
    renderSection()

    await waitFor(
      () => {
        expect(screen.getByText('Alice')).toBeInTheDocument()
      },
      { timeout: 5_000 }
    )

    await userEvent.click(screen.getByRole('button', { name: /Export/ }))
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Daily' })
    )

    await waitFor(() => {
      expect(exportUsageMock).toHaveBeenCalledWith(
        'member',
        ORGANIZATION_ID,
        expect.objectContaining({
          start: '2026-11-03',
          end: '2026-11-03',
          granularity: 'day',
        })
      )
      expect(exportMock.buildSelfUsageExportSheet).toHaveBeenCalledWith(
        expect.objectContaining({ granularity: 'day' }),
        expect.any(Function),
        { variant: 'organization' }
      )
      expect(exportMock.writeSelfUsageExportFile).toHaveBeenCalled()
    })

    await userEvent.click(screen.getByRole('button', { name: /Export/ }))
    await userEvent.click(
      await screen.findByRole('menuitem', { name: 'Hourly' })
    )

    await waitFor(() => {
      expect(exportUsageMock).toHaveBeenLastCalledWith(
        'member',
        ORGANIZATION_ID,
        expect.objectContaining({ granularity: 'hour' })
      )
    })
  })

  test('waits for the history backfill before showing numbers', async () => {
    getUsageMock.mockResolvedValue({
      success: true,
      data: usageData({ backfill_complete: false }),
    })
    renderSection()

    expect(
      await screen.findByText('History is still being prepared.')
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Requests' })
    ).not.toBeInTheDocument()
  })
})
