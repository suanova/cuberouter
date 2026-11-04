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
import { enUS, fr, ja, ru, vi, zhCN, zhTW } from 'react-day-picker/locale'

import dayjs from '@/lib/dayjs'

const calendarLocales = {
  en: enUS,
  zh: zhCN,
  zhCN,
  zhTW,
  fr,
  ru,
  ja,
  vi,
} as const

export type UsageCalendarLocale =
  (typeof calendarLocales)[keyof typeof calendarLocales]

// 后端按「含首尾」计算最多 31 天，因此两个日期最多相差 30 天。
export const MAX_USAGE_RANGE_DAYS = 31

export function usageRangeWithinLimit(from: Date, to: Date): boolean {
  return dayjs(to).diff(from, 'day') < MAX_USAGE_RANGE_DAYS
}

export function usageCalendarLocale(language: string): UsageCalendarLocale {
  return calendarLocales[language as keyof typeof calendarLocales] ?? enUS
}
