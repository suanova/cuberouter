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
import type { QuotaDataItem } from '@/features/dashboard/types'

import type { SelfUsageChartPoint } from '../types'

/**
 * Map the backend's day/model buckets onto the dashboard chart's input shape.
 * The date is treated as local midnight so the chart labels the same calendar
 * day the usage table shows.
 */
export function usageChartItems(
  points: SelfUsageChartPoint[]
): QuotaDataItem[] {
  return points.map((point) => {
    const [year, month, day] = point.date.split('-').map(Number)
    return {
      created_at: Math.floor(new Date(year, month - 1, day).getTime() / 1000),
      model_name: point.model_name,
      count: point.request_count,
      quota: point.quota,
    }
  })
}
