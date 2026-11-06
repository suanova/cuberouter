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
  points: SelfUsageChartPoint[],
  bucketHours = 1
): QuotaDataItem[] {
  const size = Number.isInteger(bucketHours) && bucketHours > 0 ? bucketHours : 1
  // Hourly points are keyed by their bucket start; a size above one folds
  // adjacent hours into one bar, e.g. the single-day chart's two-hour view.
  const byBucket = new Map<
    string,
    { createdAt: number; modelName: string; count: number; quota: number }
  >()
  for (const point of points) {
    const [datePart, timePart] = point.date.split(' ')
    const [year, month, day] = datePart.split('-').map(Number)
    const parsedHour = timePart ? Number.parseInt(timePart, 10) : 0
    const hour = Number.isFinite(parsedHour) ? parsedHour : 0
    const bucketHour = timePart ? Math.floor(hour / size) * size : 0
    const createdAt = Math.floor(
      new Date(year, month - 1, day, bucketHour).getTime() / 1000
    )
    const key = `${createdAt}|${point.model_name}`
    const existing = byBucket.get(key)
    if (existing) {
      existing.count += point.request_count
      existing.quota += point.quota
    } else {
      byBucket.set(key, {
        createdAt,
        modelName: point.model_name,
        count: point.request_count,
        quota: point.quota,
      })
    }
  }
  return [...byBucket.values()]
    .sort(
      (left, right) =>
        left.createdAt - right.createdAt ||
        left.modelName.localeCompare(right.modelName)
    )
    .map((item) => ({
      created_at: item.createdAt,
      model_name: item.modelName,
      count: item.count,
      quota: item.quota,
    }))
}
