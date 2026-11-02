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
import type { UsageLog } from '@/features/usage-logs/data/schema'
import { parseLogOther } from '@/features/usage-logs/lib/format'
import type { LogOtherData } from '@/features/usage-logs/types'

import type { OrganizationLogRow } from '../types'

// ============================================================================
// Log Types
// ============================================================================

/**
 * `model.LogType*` (model/log.go:159). The organization endpoints filter on the
 * same column as the personal ones, so the numbering is the backend's, not a
 * choice made here.
 */
export const ORGANIZATION_LOG_TYPE = {
  TOPUP: 1,
  CONSUME: 2,
  MANAGE: 3,
  SYSTEM: 4,
  ERROR: 5,
  REFUND: 6,
  LOGIN: 7,
} as const

const ORGANIZATION_LOG_TYPE_META: Record<
  number,
  { labelKey: string; variant: StatusVariant }
> = {
  [ORGANIZATION_LOG_TYPE.TOPUP]: { labelKey: 'Top-up', variant: 'cyan' },
  [ORGANIZATION_LOG_TYPE.CONSUME]: { labelKey: 'Consume', variant: 'lime' },
  [ORGANIZATION_LOG_TYPE.MANAGE]: { labelKey: 'Manage', variant: 'orange' },
  [ORGANIZATION_LOG_TYPE.SYSTEM]: { labelKey: 'System', variant: 'purple' },
  [ORGANIZATION_LOG_TYPE.ERROR]: { labelKey: 'Error', variant: 'red' },
  [ORGANIZATION_LOG_TYPE.REFUND]: { labelKey: 'Refund', variant: 'blue' },
  [ORGANIZATION_LOG_TYPE.LOGIN]: { labelKey: 'Login', variant: 'teal' },
}

export function organizationLogTypeMeta(type?: number): {
  labelKey: string
  variant: StatusVariant
} {
  return (
    ORGANIZATION_LOG_TYPE_META[type ?? 0] ?? {
      labelKey: 'Unknown',
      variant: 'grey',
    }
  )
}

export type OrganizationLogBillingMode = 'ratio' | 'per_request' | 'tiered_expr'

export const ORGANIZATION_LOG_BILLING_MODE_LABELS: Record<
  OrganizationLogBillingMode,
  string
> = {
  ratio: 'Ratio',
  per_request: 'Per Request',
  tiered_expr: 'Expression',
}

/**
 * How the request was billed.
 *
 * Tiered and per-request billing can write their mode into the log. Historical
 * rows imported from the legacy gateway record `'token'` without a model ratio,
 * so that mode marks ratio billing on its own. Otherwise the mode is inferred
 * from the pricing fields; a positive per-request price wins over a ratio
 * because that is the charge the request actually used.
 */
export function organizationLogBillingMode(
  other: LogOtherData | null
): OrganizationLogBillingMode | undefined {
  if (!other) return undefined
  if (other.billing_mode === 'tiered_expr') return 'tiered_expr'
  if (other.billing_mode === 'per_request') return 'per_request'
  if (typeof other.model_price === 'number' && other.model_price > 0) {
    return 'per_request'
  }
  if (other.billing_mode === 'token') return 'ratio'
  if (typeof other.model_ratio === 'number') return 'ratio'
  return undefined
}

/**
 * The charge before the effective group ratio was applied.
 *
 * `group_ratio` in the log is the multiplier actually used, with the user-model
 * discount already folded in (see relay/helper/price.go). Dividing it back out
 * restores the list price the token or per-request price would have produced.
 * A row without a usable ratio keeps its charged amount rather than inventing
 * one.
 */
export function organizationLogOriginalQuota(row: {
  quota?: number
  other?: string
}): number {
  const quota = row.quota ?? 0
  const groupRatio = parseLogOther(row.other ?? '')?.group_ratio
  if (
    typeof groupRatio !== 'number' ||
    !Number.isFinite(groupRatio) ||
    groupRatio <= 0
  ) {
    return quota
  }
  return Math.round(quota / groupRatio)
}

// ============================================================================
// Time Window
// ============================================================================

export interface OrganizationLogTimeWindow {
  start: Date
  end: Date
  /**
   * The URL named at least one bound, so this is the operator's own choice and
   * not the default. Used to decide whether to show a reset affordance.
   */
  fromUrl: boolean
}

/**
 * The window to read, from what the URL carries.
 *
 * A link that names only one bound gets the default for the other rather than
 * an open-ended range: the endpoint treats a missing bound as no bound at all,
 * which for the end would pull in everything ever recorded — a much larger
 * answer than the one the operator asked for.
 *
 * `fallback` is passed in rather than computed here so this stays pure; the
 * caller supplies the same default the personal usage-logs page uses, so the
 * two surfaces open on the same window.
 */
export function organizationLogTimeWindow(
  search: { startTime?: number; endTime?: number },
  fallback: { start: Date; end: Date }
): OrganizationLogTimeWindow {
  const urlStart = toDate(search.startTime)
  const urlEnd = toDate(search.endTime)

  return {
    start: urlStart ?? fallback.start,
    end: urlEnd ?? fallback.end,
    fromUrl: urlStart !== undefined || urlEnd !== undefined,
  }
}

function toDate(milliseconds?: number): Date | undefined {
  if (typeof milliseconds !== 'number' || !Number.isFinite(milliseconds)) {
    return undefined
  }
  return new Date(milliseconds)
}

export interface OrganizationLogTimeRangeParams {
  start_timestamp?: number
  end_timestamp?: number
}

/**
 * A window as the endpoint's second-resolution timestamps.
 *
 * An unparsable bound is left out rather than sent as zero, which the endpoint
 * would read as no bound at all.
 */
export function organizationLogTimeRangeParams(
  start?: Date,
  end?: Date
): OrganizationLogTimeRangeParams {
  const params: OrganizationLogTimeRangeParams = {}
  const startSeconds = toSeconds(start)
  const endSeconds = toSeconds(end)
  if (startSeconds !== undefined) params.start_timestamp = startSeconds
  if (endSeconds !== undefined) params.end_timestamp = endSeconds
  return params
}

function toSeconds(value?: Date): number | undefined {
  if (!(value instanceof Date)) return undefined
  const milliseconds = value.getTime()
  if (Number.isNaN(milliseconds)) return undefined
  return Math.floor(milliseconds / 1000)
}

// ============================================================================
// Row Values
// ============================================================================

/**
 * The model to show, and the upstream model behind it when a mapping applied.
 *
 * Both come from the row: `model_name` is what was requested, and `other`
 * records the substitution the relay made. A row whose `other` is unparsable
 * still shows its model — the mapping is extra, not a precondition.
 */
export function organizationLogModelInfo(row: {
  model_name?: string
  other?: string
}): { name: string; actualModel?: string } {
  const other = parseLogOther(row.other ?? '')
  const isMapped = Boolean(
    other?.is_model_mapped &&
    other.upstream_model_name &&
    other.upstream_model_name !== ''
  )

  return {
    name: row.model_name || '',
    actualModel: isMapped ? other?.upstream_model_name : undefined,
  }
}

/**
 * Who the entry belongs to.
 *
 * The backend hydrates the names for the rows it returns; a row whose holder
 * has since been deleted has only the stored snapshot, and one with none at all
 * falls back to the id so the column never reads as empty.
 */
export function organizationLogResponsibleName(
  row: OrganizationLogRow
): string {
  const named =
    row.responsible_display_name ||
    row.responsible_username ||
    row.responsible_name
  if (named) return named
  if (row.responsible_user_id) return `#${row.responsible_user_id}`
  return '-'
}

/** How the row's token is labelled, matching how the keys tab names one. */
export function organizationLogTokenName(row: OrganizationLogRow): string {
  if (row.token_name) return row.token_name
  if (row.token_id) return `#${row.token_id}`
  return '-'
}

/**
 * Adapt one organization log row to the shared usage-log row shape.
 *
 * The two endpoints return the same `model.Log`, but the organization view's
 * type omits fields it never displays and blanks `content` on the server. The
 * shared details dialog expects the full personal-row shape, so fill the
 * organization-owned defaults explicitly rather than casting.
 */
export function organizationLogToUsageLog(row: OrganizationLogRow): UsageLog {
  const model = organizationLogModelInfo(row)
  const organizationId = row.organization_id ?? row.billing_account_id ?? 0
  return {
    id: row.id,
    user_id: row.user_id,
    created_at: row.created_at,
    type: row.type,
    content: row.content ?? '',
    username: row.username ?? '',
    token_name: organizationLogTokenName(row),
    model_name: model.name,
    quota: row.quota,
    prompt_tokens: row.prompt_tokens,
    completion_tokens: row.completion_tokens,
    use_time: row.use_time,
    is_stream: row.is_stream,
    channel: row.channel ?? 0,
    channel_name: row.channel_name ?? '',
    token_id: row.token_id ?? 0,
    group: row.group ?? '',
    ip: row.ip ?? '',
    other: row.other ?? '',
    request_id: row.request_id ?? '',
    upstream_request_id: row.upstream_request_id ?? '',
    scope_type: row.scope_type ?? 'organization',
    scope_id: row.scope_id ?? organizationId,
    billing_account_type: row.billing_account_type ?? 'organization',
    billing_account_id: row.billing_account_id ?? organizationId,
    organization_id: organizationId,
    organization_name: row.organization_name ?? '',
    actor_user_id: row.actor_user_id ?? 0,
    creator_user_id: row.creator_user_id ?? 0,
    creator_name: row.creator_name ?? '',
    responsible_user_id: row.responsible_user_id ?? 0,
    responsible_name: row.responsible_name ?? '',
    responsible_username: row.responsible_username ?? '',
    responsible_display_name: row.responsible_display_name ?? '',
  }
}
