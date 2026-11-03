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
import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { formatNumber } from '@/lib/format'

import type { SelfUsageData } from '../../types'
import { UsageTable } from '../usage-table'

vi.mock('@/components/data-table', () => ({
  useDataTable: () => ({ table: {} }),
  DataTablePage: (props: { afterTable?: React.ReactNode }) => (
    <div data-testid='data-table'>{props.afterTable}</div>
  ),
}))

function usageData(): SelfUsageData {
  return {
    backfill_complete: true,
    range: {
      start: '2026-11-03',
      end: '2026-11-03',
      start_timestamp: 0,
      end_timestamp: 0,
    },
    overview: {
      personal: {
        request_count: 0,
        prompt_tokens: 0,
        completion_tokens: 0,
        cache_tokens: 0,
        quota: 0,
        remaining_quota: 0,
      },
      organization: null,
    },
    rows: { page: 1, page_size: 20, total: 0, items: [] },
    totals: {
      request_count: 7,
      success_count: 6,
      failure_count: 1,
      prompt_tokens: 1234,
      completion_tokens: 766,
      cache_tokens: 500,
      quota: 0,
    },
    filter_options: { models: [], tokens: [] },
  }
}

describe('UsageTable totals', () => {
  test('shows one total token count instead of input/output/cache', () => {
    render(
      <UsageTable
        data={usageData()}
        loading={false}
        filtering={false}
        organizations={[]}
        modelOptions={[]}
        tokenOptions={[]}
        initialFilters={{}}
        page={1}
        pageSize={20}
        onFiltersChange={() => undefined}
        onPageChange={() => undefined}
      />
    )

    expect(screen.getByText('Tokens')).toBeInTheDocument()
    expect(screen.getByText(formatNumber(2000))).toBeInTheDocument()
    expect(screen.queryByText('Input Tokens')).not.toBeInTheDocument()
    expect(screen.queryByText('Output Tokens')).not.toBeInTheDocument()
    expect(screen.queryByText('Cache Tokens')).not.toBeInTheDocument()
  })
})
