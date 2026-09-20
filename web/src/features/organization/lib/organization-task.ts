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

import {
  TASK_ACTION_MAPPINGS,
  TASK_STATUS_MAPPINGS,
} from '@/features/usage-logs/constants'
import type { MidjourneyLog, TaskLog } from '@/features/usage-logs/types'

import type {
  OrganizationMidjourneyTaskRow,
  OrganizationTaskRow,
} from '../types'

// ============================================================================
// Panels
// ============================================================================

/** The two task tables. Two records, two endpoints, two units of time. */
export const ORGANIZATION_TASK_PANELS = ['tasks', 'midjourney'] as const

export type OrganizationTaskPanel = (typeof ORGANIZATION_TASK_PANELS)[number]

export const DEFAULT_ORGANIZATION_TASK_PANEL: OrganizationTaskPanel = 'tasks'

/** Falls back rather than throwing, so a stale link opens the default panel. */
export function normalizeOrganizationTaskPanel(
  value: unknown
): OrganizationTaskPanel {
  return ORGANIZATION_TASK_PANELS.includes(value as OrganizationTaskPanel)
    ? (value as OrganizationTaskPanel)
    : DEFAULT_ORGANIZATION_TASK_PANEL
}

// ============================================================================
// Rows
// ============================================================================

/**
 * An organization task row, with the organization columns added.
 *
 * The endpoint returns `model.Task` rows, which is the same record the personal
 * task table reads — with four columns added to say whose work it was. Declaring
 * it as the personal shape plus those four is what lets the task table's own
 * columns render it: every cell reads a field both shapes have.
 */
export type OrganizationTaskLog = TaskLog & {
  creator_name?: string
  responsible_name?: string
  responsible_user_id?: number
  creator_user_id?: number
  token_name?: string
  request_id?: string
}

/**
 * A Midjourney row with the organization columns added.
 *
 * `quota` and `token_id` are spelled out because the personal drawing table has
 * no use for either: its `MidjourneyLog` stops at the task itself. The
 * Midjourney model does store both, so the organization view can charge the row
 * to a key and show what it cost.
 */
export type OrganizationMidjourneyTaskLog = MidjourneyLog & {
  quota?: number
  token_id?: number
  creator_name?: string
  responsible_name?: string
  responsible_user_id?: number
  token_name?: string
  request_id?: string
}

/**
 * The row as the task table reads it.
 *
 * Only four fields need filling in: the endpoint writes empty strings where the
 * personal shape promises a value, because the columns are newer than the rows
 * and a row written before them has nothing stored.
 */
export function organizationTaskToLog(
  row: OrganizationTaskRow
): OrganizationTaskLog {
  return {
    ...row,
    action: row.action ?? '',
    group: row.group ?? '',
    channel_id: row.channel_id ?? 0,
    submit_time: row.submit_time ?? 0,
  }
}

/** The same for the Midjourney table, which stores its times in milliseconds. */
export function organizationMidjourneyTaskToLog(
  row: OrganizationMidjourneyTaskRow
): OrganizationMidjourneyTaskLog {
  return {
    ...row,
    code: row.code ?? 0,
    mj_id: row.mj_id ?? '',
    action: row.action ?? '',
    channel_id: row.channel_id ?? 0,
    submit_time: row.submit_time ?? 0,
    progress: row.progress ?? '',
    prompt: row.prompt ?? '',
    status: row.status ?? '',
  }
}

/**
 * Whose work the row records.
 *
 * The names are snapshots written when the task was created, so they survive
 * the holder being renamed or deleted; the id is there for a row that predates
 * the snapshot, so the column never reads as empty.
 *
 * `username` is deliberately not consulted even though the type carries it: it
 * is a `gorm:"-"` field the personal dashboard fills in while reading, and this
 * endpoint returns the rows as they were found, so on an organization row it is
 * always blank.
 */
export function organizationTaskResponsibleName(row: {
  responsible_name?: string
  creator_name?: string
  responsible_user_id?: number
  creator_user_id?: number
  user_id?: number
}): string {
  const named = row.responsible_name || row.creator_name
  if (named) return named
  const id = row.responsible_user_id || row.creator_user_id || row.user_id
  if (id) return `#${id}`
  return '-'
}

/** How the row's key is labelled, matching how the keys tab names one. */
export function organizationTaskTokenName(row: {
  token_name?: string
  token_id?: number
}): string {
  if (row.token_name) return row.token_name
  if (row.token_id) return `#${row.token_id}`
  return '-'
}

// ============================================================================
// Filter options
// ============================================================================

/**
 * The states a task can be filtered to.
 *
 * Derived from the table's own status mapping rather than listed again, so a
 * state added to the badge cannot go missing from the filter that selects it.
 * The endpoint compares the value exactly, so there is no "all" entry: an
 * absent filter already means every state.
 */
export const ORGANIZATION_TASK_STATUS_OPTIONS = Object.entries(
  TASK_STATUS_MAPPINGS
).map(([value, meta]) => ({ value, label: meta.label }))

export const ORGANIZATION_TASK_ACTION_OPTIONS = Object.entries(
  TASK_ACTION_MAPPINGS
).map(([value, meta]) => ({ value, label: meta.label }))

// ============================================================================
// Time window
// ============================================================================

/**
 * The window as the endpoint wants it.
 *
 * The two task endpoints do not agree on the unit: `submit_time` is seconds in
 * the async task table and milliseconds in the Midjourney table, and both
 * endpoints compare the parameter against the stored column directly rather
 * than converting it. A window sent in the wrong unit does not fail — it
 * silently matches everything, or nothing.
 */
export function organizationTaskTimeRangeParams(
  start: Date | undefined,
  end: Date | undefined,
  unit: 'seconds' | 'milliseconds'
): { start_timestamp?: number; end_timestamp?: number } {
  const params: { start_timestamp?: number; end_timestamp?: number } = {}
  const startValue = toUnit(start, unit)
  const endValue = toUnit(end, unit)
  if (startValue !== undefined) params.start_timestamp = startValue
  if (endValue !== undefined) params.end_timestamp = endValue
  return params
}

function toUnit(
  value: Date | undefined,
  unit: 'seconds' | 'milliseconds'
): number | undefined {
  if (!(value instanceof Date)) return undefined
  const milliseconds = value.getTime()
  if (Number.isNaN(milliseconds)) return undefined
  return unit === 'seconds' ? Math.floor(milliseconds / 1000) : milliseconds
}

// ============================================================================
// Columns
// ============================================================================

/**
 * Puts the organization's own columns directly after one of the shared ones.
 *
 * The task tables' columns come from the usage-logs feature, so the place to
 * insert is named rather than numbered: a column added upstream would otherwise
 * move these without saying so. An unknown name appends, which is a worse
 * position for a column rather than a missing column.
 *
 * Two row types, not one. The shared columns were written against the narrower
 * row — the personal task table's — and the organization columns read the wider
 * one, so the row types cannot be the same. `TExtra extends TBase` is the
 * promise that makes mixing them sound: every field a shared column reads is
 * present on the wider row too, so a shared cell behaves identically whichever
 * table renders it. The cast below is that argument, stated once here rather
 * than at each call site.
 */
export function insertColumnsAfter<TBase, TExtra extends TBase>(
  columns: ColumnDef<TBase>[],
  afterId: string,
  extra: ColumnDef<TExtra>[]
): ColumnDef<TExtra>[] {
  const index = columns.findIndex((column) => {
    // `id` is what a column declares; `accessorKey` is the id TanStack derives
    // from the field it reads when none was declared. Both are consulted because
    // a group column carries only the first.
    const { id, accessorKey } = column as { id?: string; accessorKey?: string }
    return (id ?? accessorKey) === afterId
  })
  const shared = columns as unknown as ColumnDef<TExtra>[]
  if (index < 0) return [...shared, ...extra]
  return [...shared.slice(0, index + 1), ...extra, ...shared.slice(index + 1)]
}
