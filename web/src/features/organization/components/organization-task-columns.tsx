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
import type { ColumnDef } from '@tanstack/react-table'

import { formatQuota } from '@/lib/format'

import {
  organizationTaskResponsibleName,
  organizationTaskTokenName,
  type Translate,
} from '../lib'

/**
 * A row these columns can describe.
 *
 * Stated structurally rather than as one of the two task rows, because both task
 * tables add the same three columns over two different records. Every field here
 * exists on both.
 */
export type OrganizationTaskColumnRow = {
  responsible_name?: string
  creator_name?: string
  responsible_user_id?: number
  creator_user_id?: number
  user_id?: number
  token_name?: string
  token_id?: number
  quota?: number
}

/**
 * The columns the organization adds to a task table.
 *
 * The shared task columns come from the usage-logs feature and describe the task
 * itself — when, how long, in what state, on which artifacts. None of them can
 * say *whose* work it was or what it cost, because on a personal page both
 * answers are already implied by who is looking. Here they are not.
 *
 * They are returned as one group so the two tables put them in the same place:
 * directly after the column that identifies the task, before the columns that
 * describe it. A reader scanning a row should meet the row's identity, then who
 * it belongs to, then what it did.
 */
export function buildOrganizationTaskColumns<
  T extends OrganizationTaskColumnRow,
>(options: {
  /**
   * The caller reads every member's tasks, not just their own.
   *
   * Without it the backend pins both lists to the caller, so this column would
   * repeat one name down the table. The key and the cost stay: they still say
   * which of the caller's own keys was used and what it was charged.
   */
  showMember: boolean
  translate: Translate
}): ColumnDef<T>[] {
  const { translate } = options
  const columns: ColumnDef<T>[] = []

  if (options.showMember) {
    columns.push({
      id: 'member',
      header: translate('Member'),
      size: 150,
      cell: ({ row }) => (
        <span className='text-sm whitespace-nowrap'>
          {organizationTaskResponsibleName(row.original)}
        </span>
      ),
    })
  }

  columns.push(
    {
      id: 'token_name',
      header: translate('Key'),
      size: 150,
      cell: ({ row }) => (
        <span className='font-mono text-xs'>
          {organizationTaskTokenName(row.original)}
        </span>
      ),
    },
    {
      id: 'cost',
      header: translate('Cost'),
      size: 110,
      cell: ({ row }) => (
        // Absent on a row the organization never charged for — a task that was
        // submitted and failed before it cost anything reads as zero, not as a
        // missing figure.
        <span className='tabular-nums'>
          {formatQuota(row.original.quota ?? 0)}
        </span>
      ),
    }
  )

  return columns
}
