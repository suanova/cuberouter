/*
Copyright (C) 2023-2026 QuantumNous

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
import { expect } from 'vitest'
import { describe, test } from 'vitest'

import {
  formatOffPeakHour,
  formatVideoPrice,
  getOffPeakWindowLabel,
} from '../video-price'

describe('formatVideoPrice', () => {
  test('renders configured values verbatim without trailing zeros', () => {
    expect(formatVideoPrice(0.75)).toBe('0.75')
    expect(formatVideoPrice(0.375)).toBe('0.375')
    expect(formatVideoPrice(0.625)).toBe('0.625')
    expect(formatVideoPrice(0.3125)).toBe('0.3125')
    expect(formatVideoPrice(1)).toBe('1')
    expect(formatVideoPrice(0)).toBe('0')
  })

  test('falls back to a placeholder for non-finite values', () => {
    expect(formatVideoPrice(Number.NaN)).toBe('—')
    expect(formatVideoPrice(Number.POSITIVE_INFINITY)).toBe('—')
  })
})

describe('formatOffPeakHour', () => {
  test('formats hours with zero padding', () => {
    expect(formatOffPeakHour(22)).toBe('22:00')
    expect(formatOffPeakHour(8)).toBe('08:00')
    expect(formatOffPeakHour(0)).toBe('00:00')
    expect(formatOffPeakHour(23)).toBe('23:00')
  })

  test('rejects out-of-range or fractional hours', () => {
    expect(formatOffPeakHour(-1)).toBe('—')
    expect(formatOffPeakHour(24)).toBe('—')
    expect(formatOffPeakHour(12.5)).toBe('—')
    expect(formatOffPeakHour(Number.NaN)).toBe('—')
  })
})

describe('getOffPeakWindowLabel', () => {
  test('flags a window crossing midnight', () => {
    expect(
      getOffPeakWindowLabel({
        start_hour: 22,
        end_hour: 8,
        timezone: 'Asia/Shanghai',
      })
    ).toEqual({ start: '22:00', end: '08:00', crossesMidnight: true })
  })

  test('does not flag a same-day window', () => {
    expect(
      getOffPeakWindowLabel({
        start_hour: 9,
        end_hour: 17,
        timezone: 'Asia/Shanghai',
      })
    ).toEqual({ start: '09:00', end: '17:00', crossesMidnight: false })
  })

  test('equal start and end hours mean off-peak is disabled and no label is shown', () => {
    const label = getOffPeakWindowLabel({
      start_hour: 22,
      end_hour: 22,
      timezone: 'Asia/Shanghai',
    })
    expect(label).toBe(null)
  })

  test('returns null when the window is missing or invalid', () => {
    expect(getOffPeakWindowLabel(undefined)).toBe(null)
    expect(
      getOffPeakWindowLabel({
        start_hour: 24,
        end_hour: 8,
        timezone: 'Asia/Shanghai',
      })
    ).toBe(null)
  })
})
