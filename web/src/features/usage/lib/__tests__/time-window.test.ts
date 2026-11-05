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

import { getDefaultUsageWindow, usageDateParams } from '../time-window'

describe('usage time window', () => {
  test('defaults to today, local calendar day', () => {
    const window = getDefaultUsageWindow(new Date(2026, 10, 3, 15, 30))
    expect(window.start.getFullYear()).toBe(2026)
    expect(window.start.getMonth()).toBe(10)
    expect(window.start.getDate()).toBe(3)
    expect(window.start.getHours()).toBe(0)
    expect(window.end.getDate()).toBe(3)
  })

  test('serializes day boundaries as YYYY-MM-DD', () => {
    expect(
      usageDateParams({
        start: new Date(2026, 10, 1),
        end: new Date(2026, 10, 3),
      })
    ).toEqual({
      start: '2026-11-01',
      end: '2026-11-03',
    })
  })
})
