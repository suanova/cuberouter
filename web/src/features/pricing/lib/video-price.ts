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
import type { OffPeakWindow } from '../types'

// ----------------------------------------------------------------------------
// Video price table display helpers (pure logic, unit tested)
// ----------------------------------------------------------------------------

const MISSING_VALUE = '—'

/**
 * Format an hour-of-day (0-23) as "HH:00" for the off-peak window note.
 * Returns a placeholder for out-of-range or fractional hours.
 */
export function formatOffPeakHour(hour: number): string {
  if (!Number.isInteger(hour) || hour < 0 || hour > 23) return MISSING_VALUE
  return `${String(hour).padStart(2, '0')}:00`
}

/**
 * Format a per-second video price for display. Admin-configured ¥/s values are
 * shown verbatim (no coefficients, no rounding): JSON numbers round-trip to
 * their shortest decimal form, so String() reproduces the stored value.
 */
export function formatVideoPrice(value: number): string {
  if (!Number.isFinite(value)) return MISSING_VALUE
  return String(value)
}

export type OffPeakWindowLabel = {
  start: string
  end: string
  /** True when the window crosses midnight (start hour > end hour). */
  crossesMidnight: boolean
}

/**
 * Build the display parts of the global off-peak window. Returns null when the
 * window is absent or its hours are out of range.
 */
export function getOffPeakWindowLabel(
  window: OffPeakWindow | null | undefined
): OffPeakWindowLabel | null {
  if (!window) return null
  // start == end 表示错峰已禁用,不展示「22:00 - 22:00」这类无意义窗口
  if (window.start_hour === window.end_hour) return null
  const start = formatOffPeakHour(window.start_hour)
  const end = formatOffPeakHour(window.end_hour)
  if (start === MISSING_VALUE || end === MISSING_VALUE) return null
  return {
    start,
    end,
    crossesMidnight: window.start_hour > window.end_hour,
  }
}
