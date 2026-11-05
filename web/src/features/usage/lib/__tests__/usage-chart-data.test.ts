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

import { usageChartItems } from '../usage-chart-data'

describe('usageChartItems', () => {
  test('maps day/model buckets to chart items at local midnight', () => {
    expect(
      usageChartItems([
        {
          date: '2026-11-03',
          model_name: 'gpt-4o',
          request_count: 3,
          quota: 37,
        },
      ])
    ).toEqual([
      {
        created_at: Math.floor(new Date(2026, 10, 3).getTime() / 1000),
        model_name: 'gpt-4o',
        count: 3,
        quota: 37,
      },
    ])
  })

  test('returns an empty series for an empty response', () => {
    expect(usageChartItems([])).toEqual([])
  })
})
