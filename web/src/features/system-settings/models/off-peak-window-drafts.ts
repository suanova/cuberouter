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
import type { OffPeakWindow } from '@/features/pricing/types'

/** Editable text drafts of an off-peak window; hours stay strings while being typed. */
export type OffPeakWindowDraft = {
  startHour: string
  endHour: string
  timezone: string
}

/** Hour fields only accept digits (max 2), so drafts never hold non-integer text. */
export const hourDraftRegex = /^\d{0,2}$/

export function isValidHourDraft(draft: string): boolean {
  if (!/^\d{1,2}$/.test(draft)) return false
  const hour = Number(draft)
  return hour >= 0 && hour <= 23
}

export function windowToDrafts(window: OffPeakWindow): OffPeakWindowDraft {
  return {
    startHour: String(window.start_hour),
    endHour: String(window.end_hour),
    timezone: window.timezone,
  }
}

/**
 * Converts drafts to a window. Returns null while any hour is empty or out of
 * [0,23] — the caller then keeps the last valid value. Timezone is trimmed.
 */
export function draftsToWindow(
  drafts: OffPeakWindowDraft
): OffPeakWindow | null {
  if (!isValidHourDraft(drafts.startHour) || !isValidHourDraft(drafts.endHour)) {
    return null
  }
  return {
    start_hour: Number(drafts.startHour),
    end_hour: Number(drafts.endHour),
    timezone: drafts.timezone.trim(),
  }
}

/** start == end disables off-peak pricing (matches backend `IsOffPeakHour`). */
export function isOffPeakDisabled(window: OffPeakWindow): boolean {
  return window.start_hour === window.end_hour
}

/** Lenient parse for rendering the editor from the form's JSON string value. */
export function parseOffPeakWindowJson(value: string): OffPeakWindow | null {
  if (!value.trim()) return null
  try {
    const parsed: unknown = JSON.parse(value)
    if (typeof parsed !== 'object' || parsed === null) return null
    const window = parsed as Record<string, unknown>
    if (
      typeof window.start_hour !== 'number' ||
      typeof window.end_hour !== 'number' ||
      typeof window.timezone !== 'string'
    ) {
      return null
    }
    return {
      start_hour: window.start_hour,
      end_hour: window.end_hour,
      timezone: window.timezone,
    }
  } catch {
    return null
  }
}

/**
 * Strict structure check for the form schema: integer hours in [0,23] and a
 * string timezone. An empty timezone passes — the backend defaults it.
 */
export function isOffPeakWindowJson(value: unknown): boolean {
  if (typeof value !== 'object' || value === null) return false
  const window = value as Record<string, unknown>
  return (
    typeof window.start_hour === 'number' &&
    Number.isInteger(window.start_hour) &&
    window.start_hour >= 0 &&
    window.start_hour <= 23 &&
    typeof window.end_hour === 'number' &&
    Number.isInteger(window.end_hour) &&
    window.end_hour >= 0 &&
    window.end_hour <= 23 &&
    typeof window.timezone === 'string'
  )
}
