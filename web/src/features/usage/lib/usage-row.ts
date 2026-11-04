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
import type { SelfUsageRow } from '../types'

type UsageCountRow = Pick<
  SelfUsageRow,
  'request_count' | 'success_count' | 'failure_count'
>

/**
 * The number the Success column shows.
 *
 * A row without failures reports every request as successful — including a
 * task that is still running and has not been counted as a success yet. Only
 * once a failure exists does the stored success count become the truthful
 * answer, because then request_count would hide the failed attempt.
 */
export function usageSuccessValue(row: UsageCountRow): number {
  return row.failure_count > 0 ? row.success_count : row.request_count
}

/** The number the Failed column shows; zero is rendered as a dash. */
export function usageFailureValue(row: UsageCountRow): number {
  return row.failure_count > 0 ? row.failure_count : 0
}
