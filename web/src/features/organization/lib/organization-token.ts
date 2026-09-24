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

import type { OrganizationTokenRow } from '../types'

// ============================================================================
// Status
// ============================================================================

/** `common.TokenStatus*`. Expired and exhausted are written by the relay, never
 * by an operator, so they are filterable but not offerable. */
export const ORGANIZATION_TOKEN_STATUS = {
  ENABLED: 1,
  DISABLED: 2,
  EXPIRED: 3,
  EXHAUSTED: 4,
} as const

const ORGANIZATION_TOKEN_STATUS_META: Record<
  number,
  { labelKey: string; variant: StatusVariant }
> = {
  [ORGANIZATION_TOKEN_STATUS.ENABLED]: {
    labelKey: 'Enabled',
    variant: 'success',
  },
  [ORGANIZATION_TOKEN_STATUS.DISABLED]: {
    labelKey: 'Disabled',
    variant: 'warning',
  },
  [ORGANIZATION_TOKEN_STATUS.EXPIRED]: {
    labelKey: 'Expired',
    variant: 'neutral',
  },
  [ORGANIZATION_TOKEN_STATUS.EXHAUSTED]: {
    labelKey: 'Exhausted',
    variant: 'danger',
  },
}

export function organizationTokenStatusMeta(status?: number): {
  labelKey: string
  variant: StatusVariant
} {
  return (
    ORGANIZATION_TOKEN_STATUS_META[status ?? 0] ?? {
      labelKey: 'Unknown status',
      variant: 'neutral',
    }
  )
}

/** The four values the status filter offers, as the strings a URL carries. */
export const ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS = [
  ORGANIZATION_TOKEN_STATUS.ENABLED,
  ORGANIZATION_TOKEN_STATUS.DISABLED,
  ORGANIZATION_TOKEN_STATUS.EXPIRED,
  ORGANIZATION_TOKEN_STATUS.EXHAUSTED,
].map((value) => ({ value: String(value), labelKey: organizationTokenStatusMeta(value).labelKey }))

// ============================================================================
// Visibility
// ============================================================================

/**
 * Who may use a key.
 *
 * The distinction is load-bearing rather than cosmetic: a public key is usable
 * by every member but editable by none of them, so a member who publishes one
 * hands over its use without keeping control of it.
 */
export const ORGANIZATION_TOKEN_VISIBILITIES = {
  private: {
    labelKey: 'Private',
    descriptionKey: 'Only the responsible member can use this key.',
  },
  public: {
    labelKey: 'Public',
    descriptionKey: 'Any active member can use this key.',
  },
} as const

export type OrganizationTokenVisibility =
  keyof typeof ORGANIZATION_TOKEN_VISIBILITIES

/** An absent value is private, matching the column default on the backend. */
export function organizationTokenVisibilityMeta(visibility?: string): {
  labelKey: string
  descriptionKey: string
} {
  return (
    ORGANIZATION_TOKEN_VISIBILITIES[
      visibility as OrganizationTokenVisibility
    ] ?? ORGANIZATION_TOKEN_VISIBILITIES.private
  )
}

// ============================================================================
// Key masking
// ============================================================================

/** Every key carries this prefix; the mask keeps it so a key stays recognisable. */
const KEY_PREFIX = 'sk-'

/** Asterisks between the visible head and tail of a masked key. */
const KEY_MASK_DOTS = '**********'

/**
 * Shortens a key to `sk-abcd**********wxyz`.
 *
 * A key short enough that masking would leave nothing hidden is returned whole
 * rather than cut in half — there is no useful secret to protect there, and a
 * half-masked stub is harder to compare against than the value itself.
 */
export function maskOrganizationTokenKey(key?: string): string {
  const full = (key ?? '').trim()
  if (!full) return ''

  const prefixed = full.startsWith(KEY_PREFIX) ? full : `${KEY_PREFIX}${full}`
  const body = prefixed.slice(KEY_PREFIX.length)
  if (body.length <= 8) return prefixed

  return `${KEY_PREFIX}${body.slice(0, 4)}${KEY_MASK_DOTS}${body.slice(-4)}`
}

/**
 * The masked form of a key, preferring the one the backend computed.
 *
 * Falling back to `key` matters because the preview is only filled in on the
 * list and detail reads; a key that arrived from anywhere else still has to be
 * displayable without a round trip.
 */
export function organizationTokenKeyPreview(
  token: Pick<OrganizationTokenRow, 'key' | 'key_preview'>
): string {
  return token.key_preview?.trim() || maskOrganizationTokenKey(token.key)
}

/**
 * The key as a client has to send it, with the `sk-` prefix restored.
 *
 * The stored value is the bare key body, which is why every surface that hands
 * a key to a caller — the mask above, the bulk copy, the create dialog — puts
 * the prefix back on. Copying the bare body produces a string that matches
 * neither what the table shows nor what a relay is normally given, so this is
 * the one place that decides what a caller receives.
 */
export function organizationTokenFullKey(
  token: Pick<OrganizationTokenRow, 'key'>
): string {
  const key = (token.key ?? '').trim()
  if (!key) return ''
  return key.startsWith(KEY_PREFIX) ? key : `${KEY_PREFIX}${key}`
}

// ============================================================================
// Per-row authority
// ============================================================================

export interface OrganizationTokenActionFlags {
  /** Rename, re-scope, enable or disable the key. */
  canEdit: boolean
  canDelete: boolean
  /** Read the secret back out of the list. */
  canCopy: boolean
}

export interface OrganizationTokenActionContext {
  /** The caller may act on every key here, not only their own. */
  canManageAllTokens: boolean
  currentUserId: number
  /** The caller may not write to this organization. */
  readOnly: boolean
}

/**
 * What the caller may do with one key, mirroring the checks in
 * `service/organization_token.go`.
 *
 * A plain member holds no organization-wide authority, so they reach only the
 * keys they are responsible for — and a key they published is excluded even
 * from that, because publishing it was the act of giving up control. Copying is
 * a read: it survives `readOnly`, and a public key is copyable by anyone who can
 * see it.
 *
 * A responsible id of zero is treated as "nobody", never as a match: the field
 * is absent on a row that never went through the organization token path, and
 * reading that as ownership would hand out authority over it.
 */
export function getOrganizationTokenActionFlags(
  token: Pick<
    OrganizationTokenRow,
    'responsible_user_id' | 'user_id' | 'visibility' | 'key'
  >,
  context: OrganizationTokenActionContext
): OrganizationTokenActionFlags {
  const hasKey = Boolean((token.key ?? '').trim())
  const responsibleUserId = token.responsible_user_id || token.user_id || 0
  const isResponsible =
    responsibleUserId > 0 && responsibleUserId === context.currentUserId
  const isPublic = token.visibility === 'public'

  if (context.canManageAllTokens) {
    return {
      canEdit: !context.readOnly,
      canDelete: !context.readOnly,
      canCopy: hasKey,
    }
  }

  return {
    canEdit: !context.readOnly && isResponsible && !isPublic,
    canDelete: !context.readOnly && isResponsible && !isPublic,
    canCopy: hasKey && (isResponsible || isPublic),
  }
}

// ============================================================================
// Expiry shortcuts
// ============================================================================

const HOUR = 3600
const DAY = 24 * HOUR

/** Offered under the expiry picker. `null` means the key never expires. */
export const ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS: Array<{
  labelKey: string
  seconds: number | null
}> = [
  { labelKey: '1 hour', seconds: HOUR },
  { labelKey: '6 hours', seconds: 6 * HOUR },
  { labelKey: '1 Day', seconds: DAY },
  { labelKey: '1 Month', seconds: 30 * DAY },
  { labelKey: '1 year', seconds: 365 * DAY },
  { labelKey: 'Never expire', seconds: null },
]

/** Closed presets for the per-key quota. Amounts, not quota units, because the
 * deployment decides what a unit is worth; see `parseQuotaFromDollars`. */
export const ORGANIZATION_TOKEN_QUOTA_PRESET_AMOUNTS = [
  1, 10, 50, 100, 500, 1000,
]

// ============================================================================
// Error mapping
// ============================================================================

export interface OrganizationTokenErrorBody {
  code?: string
  message?: string
}

export type OrganizationTokenApiError = {
  response?: { data?: OrganizationTokenErrorBody }
}

/** The parameter is `unknown` because that is what a caught error is. */
function tokenErrorBody(error: unknown): OrganizationTokenErrorBody | undefined {
  return (error as OrganizationTokenApiError | null | undefined)?.response?.data
}

/**
 * The refusal codes that need a sentence of their own.
 *
 * Each one says the same thing — the key stays disabled — but for a different
 * reason, and the operator's next move differs: fix the member, wait for the
 * platform, or accept that this is above their authority.
 */
const ORGANIZATION_TOKEN_ERROR_MESSAGE_KEYS: Record<string, string> = {
  organization_token_responsible_member_disabled:
    'The responsible member for this key is disabled. Enable the member before enabling the key.',
  organization_token_responsible_user_disabled:
    'The responsible user for this key is disabled by the platform. Enable the user before enabling the key.',
  organization_token_enable_forbidden:
    'This key was disabled by a higher-privilege administrator. Your account cannot enable it.',
}

/** `null` when the refusal is not one this mapping knows. */
export function organizationTokenErrorMessageKey(error: unknown): string | null {
  const code = tokenErrorBody(error)?.code
  if (!code) return null
  return ORGANIZATION_TOKEN_ERROR_MESSAGE_KEYS[code] ?? null
}

/**
 * What to show for a failed write: the specific sentence when the code has one,
 * the backend's own message next, and the caller's fallback last.
 */
export function organizationTokenErrorText(
  error: unknown,
  fallback: string
): string {
  const specific = organizationTokenErrorMessageKey(error)
  if (specific) return specific
  return tokenErrorBody(error)?.message || fallback
}
