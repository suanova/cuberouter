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

import { UsageCharts } from '../usage-charts'

vi.mock(
  '@/features/dashboard/components/models/consumption-distribution-chart',
  () => ({
    ConsumptionDistributionChart: (props: {
      data: Array<{ count?: number }>
      loading?: boolean
      timeGranularity?: string
      timePoints?: string[]
      minTrendPoints?: number
      formatQuota?: (rawQuota: number) => string
      showChartTypeToggle?: boolean
    }) => (
      <div
        data-testid='consumption-chart'
        data-count={props.data[0]?.count}
        data-loading={props.loading}
        data-granularity={props.timeGranularity}
        data-time-points={props.timePoints?.length ?? 0}
        data-window={props.minTrendPoints}
        data-has-formatter={Boolean(props.formatQuota)}
        data-show-toggle={props.showChartTypeToggle}
      />
    ),
  })
)

describe('UsageCharts', () => {
  test('draws a single day as twelve two-hour buckets', () => {
    render(
      <UsageCharts
        points={[
          {
            date: '2026-11-03 10:00',
            model_name: 'gpt-4o',
            request_count: 3,
            quota: 37,
          },
        ]}
        loading={false}
        singleDay
      />
    )

    const chart = screen.getByTestId('consumption-chart')
    expect(chart).toHaveAttribute('data-granularity', 'hour-of-day')
    expect(chart).toHaveAttribute('data-time-points', '12')
    expect(chart).toHaveAttribute('data-count', '3')
  })

  test('renders the cost bar chart from the shared series without extra padding', () => {
    render(
      <UsageCharts
        points={[
          {
            date: '2026-11-03',
            model_name: 'gpt-4o',
            request_count: 3,
            quota: 37,
          },
        ]}
        loading={false}
      />
    )

    const chart = screen.getByTestId('consumption-chart')
    expect(chart).toHaveAttribute('data-count', '3')
    expect(chart).toHaveAttribute('data-loading', 'false')
    expect(chart).toHaveAttribute('data-granularity', 'day')
    expect(chart).toHaveAttribute('data-time-points', '0')
    expect(chart).toHaveAttribute('data-window', '0')
    expect(chart).toHaveAttribute('data-has-formatter', 'true')
    expect(chart).toHaveAttribute('data-show-toggle', 'false')
  })
})
