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
import dayjs from '@/lib/dayjs'

export function getDefaultUsageWindow(now = new Date()): {
  start: Date
  end: Date
} {
  return {
    start: dayjs(now).startOf('day').toDate(),
    end: now,
  }
}

export function usageDateParams(range: { start: Date; end: Date }): {
  start: string
  end: string
} {
  return {
    start: dayjs(range.start).format('YYYY-MM-DD'),
    end: dayjs(range.end).format('YYYY-MM-DD'),
  }
}

/** Parses the URL's day strings; missing or invalid values fall back to today. */
export function usageWindowFromSearch(
  search: { start?: string; end?: string },
  now = new Date()
): { start: Date; end: Date } {
  const fallback = getDefaultUsageWindow(now)
  const parse = (value: string | undefined) => {
    if (!value) return undefined
    const parsed = new Date(`${value}T00:00:00`)
    return Number.isNaN(parsed.getTime()) ? undefined : parsed
  }
  const start = parse(search.start) ?? fallback.start
  const end = parse(search.end) ?? fallback.end
  if (end < start) return fallback
  return { start, end }
}
