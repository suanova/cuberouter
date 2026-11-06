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
import { useMemo } from 'react'

import { ConsumptionDistributionChart } from '@/features/dashboard/components/models/consumption-distribution-chart'
import { formatLogQuota } from '@/lib/format'

import { usageChartItems } from '../lib/usage-chart-data'
import type { SelfUsageChartPoint } from '../types'

interface UsageChartsProps {
  points: SelfUsageChartPoint[]
  loading: boolean
  /** A one-day range is drawn as 24 hourly buckets instead of one bar. */
  singleDay?: boolean
}

const HOURS_OF_DAY = Array.from(
  { length: 12 },
  (_, index) => `${String(index * 2).padStart(2, '0')}:00`
)

export function UsageCharts(props: UsageChartsProps) {
  const data = useMemo(
    () => usageChartItems(props.points, props.singleDay ? 2 : 1),
    [props.points, props.singleDay]
  )

  return (
    <ConsumptionDistributionChart
      data={data}
      loading={props.loading}
      timeGranularity={props.singleDay ? 'hour-of-day' : 'day'}
      timePoints={props.singleDay ? HOURS_OF_DAY : undefined}
      minTrendPoints={0}
      formatQuota={formatLogQuota}
      showChartTypeToggle={false}
    />
  )
}
