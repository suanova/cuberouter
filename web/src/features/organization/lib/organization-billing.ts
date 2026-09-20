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
import type { StatusVariant } from '@/components/status-badge'

import type {
  OrganizationBillingMonthlySummaryItem,
  OrganizationBillingRecord,
  OrganizationBillingUserSummaryItem,
} from '../types'
import type { Translate } from './organization-audit'

// ============================================================================
// Panels
// ============================================================================

/** The three panels behind the Usage tab, in the order they are offered. */
export const ORGANIZATION_BILLING_PANELS = [
  'user',
  'monthly',
  'details',
] as const

export type OrganizationBillingPanel = (typeof ORGANIZATION_BILLING_PANELS)[number]

export const DEFAULT_ORGANIZATION_BILLING_PANEL: OrganizationBillingPanel = 'user'

/**
 * A panel name read back from a link, or the default.
 *
 * The URL is hand-editable, so an unknown name selects the first panel rather
 * than rendering nothing.
 */
export function normalizeOrganizationBillingPanel(
  value: unknown
): OrganizationBillingPanel {
  return ORGANIZATION_BILLING_PANELS.includes(value as OrganizationBillingPanel)
    ? (value as OrganizationBillingPanel)
    : DEFAULT_ORGANIZATION_BILLING_PANEL
}

// ============================================================================
// Months
// ============================================================================

/**
 * `YYYY-MM`, the only spelling the month endpoints accept
 * (`parseOrganizationBillingMonth` in service/organization_billing_summary.go
 * parses with `2006-01` and rejects anything else).
 */
const MONTH_PATTERN = /^\d{4}-(0[1-9]|1[0-2])$/

export function isOrganizationBillingMonth(value: unknown): value is string {
  return typeof value === 'string' && MONTH_PATTERN.test(value)
}

/** The month `now` falls in, in the browser's own timezone. */
export function currentOrganizationBillingMonth(now = new Date()): string {
  return formatOrganizationBillingMonth(now)
}

function formatOrganizationBillingMonth(date: Date): string {
  return `${date.getFullYear()}-${String(date.getMonth() + 1).padStart(2, '0')}`
}

/**
 * `count` months ending with the current one, newest first — the order the
 * select shows them in.
 *
 * Built from the first of each month rather than by subtracting days: a month
 * that is shorter than its predecessor would otherwise be skipped.
 */
export function recentOrganizationBillingMonths(
  count = 12,
  now = new Date()
): string[] {
  const months: string[] = []
  for (let index = 0; index < count; index += 1) {
    months.push(
      formatOrganizationBillingMonth(
        new Date(now.getFullYear(), now.getMonth() - index, 1)
      )
    )
  }
  return months
}

/** A month from a link, or `fallback` when it does not name one. */
export function normalizeOrganizationBillingMonth(
  value: unknown,
  fallback: string
): string {
  return isOrganizationBillingMonth(value) ? value : fallback
}

/**
 * A month range the user-summaries endpoint will accept.
 *
 * The endpoint rejects a range whose start is later than its end
 * (`resolveOrganizationBillingUserSummaryRange` compares the two month starts
 * and errors when `start >= end`), and equal months are a legitimate
 * single-month range because the end it computes is the *next* month's start.
 * A range entered backwards is therefore swapped rather than refused: the user
 * has said which two months they want and the order of the two boxes is not
 * part of that.
 *
 * One bound without the other reads as that same month on both sides, which is
 * the range a reader means when they pick one month out of a pair of selects.
 */
export function organizationBillingMonthRange(
  start: unknown,
  end: unknown,
  now = new Date()
): { start: string; end: string } {
  const fallback = currentOrganizationBillingMonth(now)
  const from = isOrganizationBillingMonth(start) ? start : undefined
  const to = isOrganizationBillingMonth(end) ? end : undefined
  const resolvedFrom = from ?? to ?? fallback
  const resolvedTo = to ?? from ?? fallback
  return resolvedFrom <= resolvedTo
    ? { start: resolvedFrom, end: resolvedTo }
    : { start: resolvedTo, end: resolvedFrom }
}

/** How many months the monthly overview may look back. */
export const ORGANIZATION_BILLING_MONTH_COUNTS = [6, 12, 24, 36] as const

export const DEFAULT_ORGANIZATION_BILLING_MONTHS = 12

/**
 * A month count from the UI or a link.
 *
 * 36 is the endpoint's own ceiling (`normalizeOrganizationBillingMonths`), so a
 * hand-edited value above it is clamped here rather than silently answered with
 * less data than the label promises.
 */
export function normalizeOrganizationBillingMonths(value: unknown): number {
  const months = Number(value)
  if (!Number.isFinite(months) || months <= 0) {
    return DEFAULT_ORGANIZATION_BILLING_MONTHS
  }
  return Math.min(Math.floor(months), 36)
}

/** The entry for one month, or null when that month has no rows. */
export function findOrganizationBillingMonth(
  items: OrganizationBillingMonthlySummaryItem[],
  month: string
): OrganizationBillingMonthlySummaryItem | null {
  return items.find((item) => item?.month === month) ?? null
}

// ============================================================================
// Records
// ============================================================================

/**
 * The four record types the ledger writes
 * (`model.OrganizationBillingRecordType*`). The colours separate a reservation
 * from the settlement that replaces it, which is the distinction a reader has
 * to make to understand why one request appears twice.
 */
const RECORD_TYPE_META: Record<
  string,
  { labelKey: string; variant: StatusVariant }
> = {
  pre_consume: { labelKey: 'Pre-consumed', variant: 'orange' },
  settle: { labelKey: 'Settled', variant: 'blue' },
  refund: { labelKey: 'Refunded', variant: 'red' },
  adjustment: { labelKey: 'Adjusted', variant: 'purple' },
}

/**
 * The label and colour for a record type.
 *
 * A type this build does not know about — a newer backend, a value typed into a
 * link — is shown as the raw string with a neutral colour rather than hidden.
 */
export function organizationBillingRecordTypeMeta(
  recordType: string | undefined,
  translate: Translate
): { label: string; variant: StatusVariant } {
  const meta = RECORD_TYPE_META[recordType ?? '']
  if (!meta) {
    return { label: recordType || '-', variant: 'grey' }
  }
  return { label: translate(meta.labelKey), variant: meta.variant }
}

/**
 * How much this record moved the organization's quota.
 *
 * The endpoint computes this per type and returns it as `ledger_quota_delta`
 * (`hydrateOrganizationBillingRecordLedgerQuotaDeltas`): an adjustment moves by
 * its own delta, a settlement by the settlement delta when the request was
 * pre-consumed and by the full usage otherwise, everything else by its used
 * delta. Recomputing it here would mean a second copy of that rule, so this
 * reads the field the backend already resolved.
 */
export function organizationBillingLedgerDelta(
  record: OrganizationBillingRecord
): number {
  return record.ledger_quota_delta ?? 0
}

/**
 * Who was responsible for a charge: the resolved display name, then the
 * username, then the id.
 *
 * The last two fallbacks exist because the two fields are hydrated from the
 * user table, and a user deleted since the charge leaves nothing to hydrate —
 * the id is still the honest answer to "who was this".
 */
export function organizationBillingResponsibleName(record: {
  responsible_display_name?: string
  responsible_username?: string
  responsible_user_id?: number
}): string {
  const name =
    record.responsible_display_name?.trim() ||
    record.responsible_username?.trim()
  if (name) return name
  return record.responsible_user_id ? `#${record.responsible_user_id}` : '-'
}

function sumField<T>(items: T[], read: (item: T) => number | undefined): number {
  return items.reduce((sum, item) => sum + (Number(read(item)) || 0), 0)
}

/** What the per-member overview says above the table. */
export function organizationBillingUserOverviewStats(
  items: OrganizationBillingUserSummaryItem[]
) {
  const totalQuota = sumField(items, (item) => item.quota)
  const totalRequests = sumField(items, (item) => item.request_count)
  return {
    totalQuota,
    totalRequests,
    // A member with neither cost nor traffic is a member the organization did
    // not spend anything on in this range, which is not the same as a member
    // who is gone — the list already covers everyone who may be charged.
    activeUsers: items.filter(
      (item) => (Number(item.quota) || 0) > 0 || (Number(item.request_count) || 0) > 0
    ).length,
    topUser:
      items.reduce<OrganizationBillingUserSummaryItem | null>(
        (best, item) => ((item.quota || 0) > (best?.quota || 0) ? item : best),
        null
      ) ?? null,
  }
}

/** What the monthly overview says above the table. */
export function organizationBillingMonthlyOverviewStats(
  items: OrganizationBillingMonthlySummaryItem[],
  month = currentOrganizationBillingMonth()
) {
  const totalQuota = sumField(items, (item) => item.quota)
  return {
    totalQuota,
    // Averaged over the months that have rows, not over the window that was
    // asked for: the endpoint starts at the organization's creation month, so
    // dividing by the requested count would dilute every early month.
    averageQuota: items.length > 0 ? Math.round(totalQuota / items.length) : 0,
    peak:
      items.reduce<OrganizationBillingMonthlySummaryItem | null>(
        (best, item) => ((item.quota || 0) > (best?.quota || 0) ? item : best),
        null
      ) ?? null,
    current: findOrganizationBillingMonth(items, month),
  }
}

// ============================================================================
// Detail filters
// ============================================================================

/**
 * The fields the records endpoint filters on
 * (`OrganizationBillingDetailListRequest`).
 *
 * `month` and a timestamp window are alternatives, not a pair: the endpoint
 * ignores `month` as soon as either bound is set, so a window is deliberately
 * not offered here alongside the month select.
 */
export type OrganizationBillingDetailFilters = {
  month?: string
  token_name?: string
  model_name?: string
  group?: string
  request_id?: string
  responsible_name?: string
}

/**
 * Narrows form values to the request the endpoint accepts.
 *
 * Blank fields are dropped rather than sent empty: the endpoint treats an empty
 * `token_name` as a filter on the empty string, which matches nothing.
 * `responsible_name` is dropped without wide-data capability because the
 * endpoint pins such a caller to their own rows regardless, so sending it would
 * only make the request look more specific than the answer is.
 */
export function organizationBillingDetailFilters(
  values: {
    month?: string
    token_name?: string
    model_name?: string
    group?: string
    request_id?: string
    responsible_name?: string
  },
  options: { canViewWideData: boolean }
): OrganizationBillingDetailFilters {
  const filters: OrganizationBillingDetailFilters = {}
  const keywords = [
    'token_name',
    'model_name',
    'group',
    'request_id',
  ] as const
  for (const key of keywords) {
    const value = values[key]?.trim()
    if (value) filters[key] = value
  }
  if (isOrganizationBillingMonth(values.month)) {
    filters.month = values.month
  }
  if (options.canViewWideData) {
    const responsible = values.responsible_name?.trim()
    if (responsible) filters.responsible_name = responsible
  }
  return filters
}

// ============================================================================
// CSV export
// ============================================================================

/**
 * One CSV field.
 *
 * Quoted only when it has to be, so the file stays readable when opened as
 * text; a quote inside a quoted field is doubled, which is the one escape CSV
 * defines.
 */
export function csvCell(value: unknown): string {
  if (value === null || value === undefined) return ''
  const text = String(value)
  return /[",\n\r]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text
}

export type OrganizationCsvColumn<T> = {
  title: string
  value: (row: T) => unknown
}

/**
 * Rows as a CSV document, with a byte-order mark.
 *
 * The mark is what makes Excel read the file as UTF-8; without it a column of
 * Chinese names opens as mojibake.
 */
export function buildOrganizationBillingCsv<T>(
  columns: OrganizationCsvColumn<T>[],
  rows: T[]
): string {
  const header = columns.map((column) => csvCell(column.title)).join(',')
  const body = rows
    .map((row) => columns.map((column) => csvCell(column.value(row))).join(','))
    .join('\n')
  return `﻿${header}${body ? `\n${body}` : ''}`
}

/** `YYYY-MM-DD-HHmmss`, safe in a filename on every platform. */
export function organizationBillingExportTimestamp(date = new Date()): string {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}-${pad(date.getHours())}${pad(date.getMinutes())}${pad(date.getSeconds())}`
}

export function organizationBillingCsvFilename(
  slug: string,
  date = new Date()
): string {
  return `organization-billing-${slug}-${organizationBillingExportTimestamp(date)}.csv`
}

/**
 * A timestamp for a cell, in full rather than abbreviated.
 *
 * An exported row is read outside the app, where relative wording ("3 minutes
 * ago") is meaningless, so this is the local wall-clock time down to the second.
 */
export function formatOrganizationBillingExportTime(
  timestamp?: number
): string {
  const seconds = Number(timestamp)
  if (!Number.isFinite(seconds) || seconds <= 0) return ''
  const date = new Date(seconds * 1000)
  if (Number.isNaN(date.getTime())) return ''
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`
}

/**
 * Hands a generated file to the browser.
 *
 * The object URL is revoked as soon as the click is dispatched: the download
 * has already started by then, and holding the URL would keep the blob alive
 * for the lifetime of the document.
 */
export function downloadOrganizationBillingCsv(
  content: string,
  filename: string
): void {
  const url = URL.createObjectURL(
    new Blob([content], { type: 'text/csv;charset=utf-8' })
  )
  const link = document.createElement('a')
  link.href = url
  link.download = filename
  link.click()
  URL.revokeObjectURL(url)
}
