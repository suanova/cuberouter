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

import {
  discountToPercent,
  isValidPercent,
  percentToDiscount,
  removeDiscount,
  upsertDiscount,
} from './model-discount'

describe('model discount conversion', () => {
  test('converts a percent to a 4-decimal discount', () => {
    expect(percentToDiscount(1)).toBe(0.01)
    expect(percentToDiscount(80)).toBe(0.8)
    expect(percentToDiscount(100)).toBe(1)
    expect(percentToDiscount(85.125)).toBe(0.8513)
    expect(percentToDiscount(66.6666)).toBe(0.6667)
  })

  test('converts a discount back to percent', () => {
    expect(discountToPercent(0.01)).toBe(1)
    expect(discountToPercent(0.8)).toBe(80)
    expect(discountToPercent(0.8513)).toBe(85.13)
    expect(discountToPercent(1)).toBe(100)
  })

  test('round-trips a percent through the discount representation', () => {
    for (const percent of [1, 25, 50, 80, 99.99, 100]) {
      expect(discountToPercent(percentToDiscount(percent))).toBeCloseTo(
        percent,
        10
      )
    }
  })

  test('accepts only finite percents within 1..100', () => {
    expect(isValidPercent(1)).toBe(true)
    expect(isValidPercent(80)).toBe(true)
    expect(isValidPercent(100)).toBe(true)
    expect(isValidPercent(0)).toBe(false)
    expect(isValidPercent(-1)).toBe(false)
    expect(isValidPercent(100.5)).toBe(false)
    expect(isValidPercent(Number.NaN)).toBe(false)
    expect(isValidPercent(Number.POSITIVE_INFINITY)).toBe(false)
  })
})

describe('model discount map edits', () => {
  test('upsert adds a model without mutating the source map', () => {
    const value = { m: 0.8 }

    const next = upsertDiscount(value, 'm2', 50)

    expect(next).toEqual({ m: 0.8, m2: 0.5 })
    expect(value).toEqual({ m: 0.8 })
    expect(next).not.toBe(value)
  })

  test('upsert overwrites an existing model with 4-decimal rounding', () => {
    expect(upsertDiscount({ m: 0.8 }, 'm', 90)).toEqual({ m: 0.9 })
    expect(upsertDiscount({}, 'm', 85.125)).toEqual({ m: 0.8513 })
  })

  test('upsert rejects an out-of-range or non-finite percent', () => {
    expect(() => upsertDiscount({}, 'm', 0)).toThrow(RangeError)
    expect(() => upsertDiscount({}, 'm', 100.5)).toThrow(RangeError)
    expect(() => upsertDiscount({}, 'm', Number.NaN)).toThrow(RangeError)
  })

  test('remove drops the model and keeps the other entries', () => {
    const value = { a: 0.5, b: 0.9 }

    const next = removeDiscount(value, 'a')

    expect(next).toEqual({ b: 0.9 })
    expect(value).toEqual({ a: 0.5, b: 0.9 })
    expect(next).not.toBe(value)
  })

  test('remove of an absent model returns an equivalent copy', () => {
    const value = { a: 0.5 }

    const next = removeDiscount(value, 'missing')

    expect(next).toEqual({ a: 0.5 })
    expect(next).not.toBe(value)
  })
})
