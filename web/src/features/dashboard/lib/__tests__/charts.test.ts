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

import type { QuotaDataItem } from '../../types'
import { processChartData } from '../charts'

function singleDay(): QuotaDataItem[] {
  return [
    {
      created_at: Math.floor(new Date(2026, 10, 3, 10).getTime() / 1000),
      model_name: 'gpt-4o',
      count: 2,
      quota: 10,
    },
  ]
}

function uniqueTrendTimes(chart: ReturnType<typeof processChartData>): number {
  const values = chart.spec_line.data[0].values as Array<{ Time: string }>
  return new Set(values.map((value) => value.Time)).size
}

describe('processChartData options', () => {
  test('pads a single bucket to the dashboard trend window by default', () => {
    expect(uniqueTrendTimes(processChartData(singleDay(), 'day'))).toBe(7)
  })

  test('keeps only the data buckets when the caller disables padding', () => {
    expect(
      uniqueTrendTimes(
        processChartData(singleDay(), 'day', undefined, undefined, {
          minTrendPoints: 0,
        })
      )
    ).toBe(1)
  })

  test('formats the quota total with the caller formatter', () => {
    const chart = processChartData(singleDay(), 'day', undefined, undefined, {
      formatQuota: (quota) => `¥${quota.toFixed(2)}`,
    })
    expect(chart.totalQuotaDisplay).toBe('¥10.00')
  })

  test('formats the value axis with the caller formatter', () => {
    const chart = processChartData(singleDay(), 'day', undefined, undefined, {
      formatQuota: (quota) => `¥${quota.toFixed(2)}`,
    })
    const axes = chart.spec_line.axes as Array<{
      orient: string
      label?: { formatMethod?: (value: number) => string }
    }>
    const left = axes.find((axis) => axis.orient === 'left')
    expect(left?.label?.formatMethod?.(10)).toBe('¥10.00')
  })

  test('leaves the value axis to the dashboard default without a formatter', () => {
    expect(processChartData(singleDay(), 'day').spec_line.axes).toBeUndefined()
  })
})
