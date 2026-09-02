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

import type { OffPeakWindow } from '@/features/pricing/types'

import {
  draftsToWindow,
  hourDraftRegex,
  isOffPeakDisabled,
  isOffPeakWindowJson,
  isValidHourDraft,
  parseOffPeakWindowJson,
  windowToDrafts,
} from '../off-peak-window-drafts'

const defaultWindow: OffPeakWindow = {
  start_hour: 22,
  end_hour: 8,
  timezone: 'Asia/Shanghai',
}

describe('off-peak window drafts', () => {
  test('window to drafts preserves values as strings', () => {
    expect(windowToDrafts(defaultWindow)).toEqual({
      startHour: '22',
      endHour: '8',
      timezone: 'Asia/Shanghai',
    })
  })

  test('drafts to window round-trips the default window', () => {
    const window = draftsToWindow(windowToDrafts(defaultWindow))
    expect(window).toEqual(defaultWindow)
  })

  test('drafts to window returns null for an empty or out-of-range hour', () => {
    expect(draftsToWindow({ startHour: '', endHour: '8', timezone: 'x' })).toBe(null)
    expect(draftsToWindow({ startHour: '24', endHour: '8', timezone: 'x' })).toBe(null)
    expect(draftsToWindow({ startHour: '-1', endHour: '8', timezone: 'x' })).toBe(null)
    expect(draftsToWindow({ startHour: '22', endHour: '1a', timezone: 'x' })).toBe(null)
  })

  test('drafts to window trims the timezone', () => {
    const window = draftsToWindow({
      startHour: '22',
      endHour: '8',
      timezone: '  Asia/Shanghai  ',
    })
    expect(window?.timezone).toBe('Asia/Shanghai')
  })

  test('hour draft regex only admits up to two digits', () => {
    expect(hourDraftRegex.test('')).toBeTruthy()
    expect(hourDraftRegex.test('0')).toBeTruthy()
    expect(hourDraftRegex.test('23')).toBeTruthy()
    expect(hourDraftRegex.test('234')).toBe(false)
    expect(hourDraftRegex.test('2a')).toBe(false)
    expect(hourDraftRegex.test(' 2')).toBe(false)
  })

  test('hour validation accepts 0-23 and rejects empty, negative or out-of-range', () => {
    expect(isValidHourDraft('0')).toBe(true)
    expect(isValidHourDraft('23')).toBe(true)
    expect(isValidHourDraft('')).toBe(false)
    expect(isValidHourDraft('24')).toBe(false)
    expect(isValidHourDraft('-1')).toBe(false)
    expect(isValidHourDraft('1.5')).toBe(false)
  })

  test('equal start and end hours disable off-peak pricing', () => {
    expect(isOffPeakDisabled({ start_hour: 22, end_hour: 22, timezone: 'UTC' })).toBe(true)
    expect(isOffPeakDisabled(defaultWindow)).toBe(false)
  })

  test('parse window JSON accepts valid and lenient numbers', () => {
    expect(parseOffPeakWindowJson(JSON.stringify(defaultWindow))).toEqual(defaultWindow)
    expect(
      parseOffPeakWindowJson('{"start_hour":24,"end_hour":8,"timezone":"UTC"}')
    ).toEqual({ start_hour: 24, end_hour: 8, timezone: 'UTC' })
  })

  test('parse window JSON rejects malformed input', () => {
    expect(parseOffPeakWindowJson('')).toBe(null)
    expect(parseOffPeakWindowJson('  ')).toBe(null)
    expect(parseOffPeakWindowJson('{nope')).toBe(null)
    expect(parseOffPeakWindowJson('[]')).toBe(null)
    expect(parseOffPeakWindowJson('{"start_hour":"22","end_hour":8,"timezone":"UTC"}')).toBe(null)
  })

  test('window JSON structure check requires integer hours in [0,23] and a timezone string', () => {
    expect(isOffPeakWindowJson(defaultWindow)).toBe(true)
    expect(isOffPeakWindowJson({ start_hour: 0, end_hour: 23, timezone: '' })).toBe(true)
    expect(isOffPeakWindowJson({ start_hour: 24, end_hour: 8, timezone: 'UTC' })).toBe(false)
    expect(isOffPeakWindowJson({ start_hour: 22, end_hour: 8 })).toBe(false)
    expect(isOffPeakWindowJson({ start_hour: 22.5, end_hour: 8, timezone: 'UTC' })).toBe(false)
    expect(isOffPeakWindowJson(null)).toBe(false)
    expect(isOffPeakWindowJson('not an object')).toBe(false)
  })
})
