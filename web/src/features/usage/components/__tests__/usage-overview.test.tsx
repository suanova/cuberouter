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
import { describe, expect, test } from 'vitest'

import { formatNumber } from '@/lib/format'

import type { SelfUsageData } from '../../types'
import { UsageOverview } from '../usage-overview'

function usageData(
  stat: Partial<SelfUsageData['overview']['personal']>
): SelfUsageData {
  const personal = {
    request_count: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    cache_tokens: 0,
    quota: 0,
    remaining_quota: 0,
    ...stat,
  }
  return {
    backfill_complete: true,
    range: {
      start: '2026-11-03',
      end: '2026-11-03',
      start_timestamp: 0,
      end_timestamp: 0,
    },
    overview: { personal, organization: null },
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

describe('UsageOverview', () => {
  test('shows one total token count for the personal row', () => {
    render(
      <UsageOverview
        data={usageData({
          prompt_tokens: 1234,
          completion_tokens: 766,
          cache_tokens: 500,
        })}
        loading={false}
        organizations={[]}
        selectedOrganizationId={null}
        onSelectOrganization={() => undefined}
      />
    )

    expect(screen.getByText('Tokens')).toBeInTheDocument()
    expect(screen.getByText(formatNumber(2000))).toBeInTheDocument()
    expect(screen.getByText('Available quota')).toBeInTheDocument()
    expect(screen.queryByText('Remaining quota')).not.toBeInTheDocument()
    expect(screen.queryByText('Input Tokens')).not.toBeInTheDocument()
    expect(screen.queryByText('Output Tokens')).not.toBeInTheDocument()
    expect(screen.queryByText('Cache Tokens')).not.toBeInTheDocument()
    expect(screen.getByText('Cost').closest('.grid')?.className).toContain(
      'sm:grid-cols-4'
    )
  })

  test('shows one total token count for the organization row', () => {
    const data = usageData({})
    data.overview.organization = {
      id: 9,
      name: 'Acme',
      request_count: 0,
      prompt_tokens: 100,
      completion_tokens: 50,
      cache_tokens: 25,
      quota: 0,
      remaining_quota: 0,
      member_quota_limit: 0,
      member_used_quota: 0,
      organization_quota: 0,
      organization_used_quota: 0,
    }

    render(
      <UsageOverview
        data={data}
        loading={false}
        organizations={[{ id: 9, name: 'Acme' }]}
        selectedOrganizationId={9}
        onSelectOrganization={() => undefined}
      />
    )

    expect(screen.getAllByText('Tokens')).toHaveLength(2)
    expect(screen.getByText(formatNumber(150))).toBeInTheDocument()
    expect(screen.getAllByText('Available quota')).toHaveLength(2)
    expect(screen.queryByText('My available quota')).not.toBeInTheDocument()
    expect(screen.queryByText('Remaining quota')).not.toBeInTheDocument()
    expect(screen.queryByText('Input Tokens')).not.toBeInTheDocument()
    expect(screen.queryByText('Output Tokens')).not.toBeInTheDocument()
    expect(screen.queryByText('Cache Tokens')).not.toBeInTheDocument()
  })
})
