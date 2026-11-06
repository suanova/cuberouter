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

  test('maps hourly buckets to the exact local hour', () => {
    expect(
      usageChartItems([
        {
          date: '2026-11-03 10:00',
          model_name: 'gpt-4o',
          request_count: 2,
          quota: 30,
        },
      ])
    ).toEqual([
      {
        created_at: Math.floor(new Date(2026, 10, 3, 10).getTime() / 1000),
        model_name: 'gpt-4o',
        count: 2,
        quota: 30,
      },
    ])
  })

  test('folds adjacent hours into two-hour buckets', () => {
    expect(
      usageChartItems(
        [
          {
            date: '2026-11-03 10:00',
            model_name: 'gpt-4o',
            request_count: 1,
            quota: 10,
          },
          {
            date: '2026-11-03 11:00',
            model_name: 'gpt-4o',
            request_count: 2,
            quota: 20,
          },
          {
            date: '2026-11-03 11:00',
            model_name: 'other',
            request_count: 1,
            quota: 5,
          },
        ],
        2
      )
    ).toEqual([
      {
        created_at: Math.floor(new Date(2026, 10, 3, 10).getTime() / 1000),
        model_name: 'gpt-4o',
        count: 3,
        quota: 30,
      },
      {
        created_at: Math.floor(new Date(2026, 10, 3, 10).getTime() / 1000),
        model_name: 'other',
        count: 1,
        quota: 5,
      },
    ])
  })

  test('returns an empty series for an empty response', () => {
    expect(usageChartItems([])).toEqual([])
  })
})
