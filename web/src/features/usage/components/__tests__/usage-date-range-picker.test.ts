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
import { enUS, zhCN, zhTW } from 'react-day-picker/locale'
import { describe, expect, test } from 'vitest'

import {
  usageCalendarLocale,
  usageRangeWithinLimit,
} from '../../lib/date-range'

describe('usage date range limits', () => {
  test('accepts 31 inclusive dates and rejects one more', () => {
    expect(
      usageRangeWithinLimit(new Date(2026, 10, 1), new Date(2026, 11, 1))
    ).toBe(true)
    expect(
      usageRangeWithinLimit(new Date(2026, 10, 1), new Date(2026, 11, 2))
    ).toBe(false)
  })
})

describe('usage calendar locales', () => {
  test('maps the app language codes onto calendar locales', () => {
    expect(usageCalendarLocale('zhCN')).toBe(zhCN)
    expect(usageCalendarLocale('zhTW')).toBe(zhTW)
    expect(usageCalendarLocale('en')).toBe(enUS)
    expect(usageCalendarLocale('de')).toBe(enUS)
  })
})
