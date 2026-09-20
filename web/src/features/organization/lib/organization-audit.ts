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
import type { OrganizationAuditLogRow } from '../types'

/** `t` as react-i18next exposes it, narrowed to what these helpers need. */
export type Translate = (
  key: string,
  options?: Record<string, unknown>
) => string

// ============================================================================
// Action Types
// ============================================================================

/**
 * Action type to a human label.
 *
 * Keys are the action types with their `organization.` prefix removed — see
 * {@link normalizeOrganizationAuditAction}. The backend writes the full dotted
 * constant (`organization.member.update`), so keeping the prefix out here means
 * one entry per operation rather than one per prefix, and a new prefix on an
 * existing operation does not silently lose its label.
 */
const ORGANIZATION_AUDIT_ACTION_LABELS: Record<string, string> = {
  create: 'Organization Created',
  update: 'Organization Updated',
  disable: 'Organization Disabled',
  enable: 'Organization Enabled',
  dissolve: 'Organization Dissolved',
  quota_adjust: 'Quota Adjusted',
  'member.add': 'Member Added',
  'member.update': 'Member Updated',
  'member.remove': 'Member Removed',
  'member.exit': 'Member Exited',
  'member.key_transfer': 'API Keys Transferred',
  'member.key_transfer_blocked': 'API Key Transfer Blocked',
  'owner.transfer': 'Ownership Transferred',
  'token.create': 'API Key Created',
  'token.update': 'API Key Updated',
  'token.delete': 'API Key Deleted',
  'token.responsibility_update': 'API Key Owner Changed',
  'invite.create': 'Invitation Sent',
  'invite.resend': 'Invitation Resent',
  'invite.reopen': 'Invitation Reopened',
  'invite.accept': 'Invitation Accepted',
  'invite.revoke': 'Invitation Revoked',
  'billing.repair_failed': 'Billing Repair Failed',
}

/** Which colour an action reads as: destructive, additive, corrective, other. */
export type OrganizationAuditTone =
  | 'red'
  | 'green'
  | 'orange'
  | 'blue'
  | 'purple'
  | 'cyan'
  | 'grey'

export interface OrganizationAuditFilterOption {
  /** The stored value, which is what the backend compares against. */
  value: string
  label: string
}

/**
 * The action filter's options.
 *
 * The backend compares `action_type` for equality, so the filter cannot be a
 * text box: it offers the action types this build knows about. The values carry
 * the full `organization.` prefix because that is the stored spelling.
 */
export function organizationAuditActionOptions(
  t: Translate
): OrganizationAuditFilterOption[] {
  return Object.keys(ORGANIZATION_AUDIT_ACTION_LABELS).map((normalized) => ({
    value: `organization.${normalized}`,
    label: t(ORGANIZATION_AUDIT_ACTION_LABELS[normalized]),
  }))
}

export function organizationAuditTargetTypeOptions(
  t: Translate
): OrganizationAuditFilterOption[] {
  return Object.keys(ORGANIZATION_AUDIT_TARGET_TYPE_LABELS).map((type) => ({
    value: type,
    label: t(ORGANIZATION_AUDIT_TARGET_TYPE_LABELS[type]),
  }))
}

/**
 * Drops the `organization.` prefix so a stored action type and one the backend
 * later re-prefixes both resolve to the same key.
 */
export function normalizeOrganizationAuditAction(actionType: unknown): string {
  let action = String(actionType ?? '').trim()
  if (action.startsWith('organization.audit.action.')) {
    action = action.slice('organization.audit.action.'.length)
  }
  if (action.startsWith('organization.')) {
    action = action.slice('organization.'.length)
  }
  return action || 'unknown'
}

/**
 * The i18n key for an action. An unrecognized action falls back to its raw
 * value, which is better than "Unknown": the audit trail is the one place where
 * seeing the backend's own vocabulary beats seeing nothing.
 */
export function organizationAuditActionLabelKey(actionType: unknown): string {
  const normalized = normalizeOrganizationAuditAction(actionType)
  return ORGANIZATION_AUDIT_ACTION_LABELS[normalized] ?? String(actionType ?? '-')
}

export function organizationAuditActionTone(
  actionType: unknown
): OrganizationAuditTone {
  const action = String(actionType ?? '').toLowerCase()
  if (
    ['delete', 'remove', 'revoke', 'dissolve', 'disable'].some((word) =>
      action.includes(word)
    )
  ) {
    return 'red'
  }
  if (
    ['create', 'invite', 'enable', 'accept'].some((word) =>
      action.includes(word)
    )
  ) {
    return 'green'
  }
  if (
    ['update', 'adjust', 'transfer'].some((word) => action.includes(word))
  ) {
    return 'orange'
  }
  return 'blue'
}

// ============================================================================
// Target Types
// ============================================================================

const ORGANIZATION_AUDIT_TARGET_TYPE_LABELS: Record<string, string> = {
  organization: 'Organization',
  member: 'Member',
  token: 'API Key',
  invite: 'Invitation',
  quota_adjustment: 'Quota Adjustment',
  billing_session: 'Billing Session',
}

const ORGANIZATION_AUDIT_TARGET_TONES: Record<string, OrganizationAuditTone> = {
  token: 'purple',
  member: 'cyan',
  invite: 'green',
  quota_adjustment: 'orange',
  organization: 'blue',
  billing_session: 'grey',
}

/** Unknown types are shown as the backend spelled them, never as "Unknown". */
export function organizationAuditTargetTypeLabelKey(targetType: unknown): string {
  const type = String(targetType ?? '')
  return ORGANIZATION_AUDIT_TARGET_TYPE_LABELS[type] ?? (type || '-')
}

export function organizationAuditTargetTone(
  targetType: unknown
): OrganizationAuditTone {
  return ORGANIZATION_AUDIT_TARGET_TONES[String(targetType ?? '')] ?? 'grey'
}

// ============================================================================
// Target Rendering
// ============================================================================

const ORGANIZATION_AUDIT_TOKEN_PREVIEW_PATTERN =
  /^(?:\*{3}|[^*]{6}\*{10}[^*]{6})$/

/**
 * Shows enough of a key to identify it and not enough to use it. A value that
 * is already masked server-side is passed through rather than masked twice,
 * which would hide the first and last six characters behind asterisks.
 */
export function maskOrganizationAuditApiKey(value: unknown): string {
  const text = String(value ?? '').trim()
  if (!text) return '-'
  const prefix = text.startsWith('sk-') ? 'sk-' : ''
  const body = prefix ? text.slice(prefix.length) : text
  if (ORGANIZATION_AUDIT_TOKEN_PREVIEW_PATTERN.test(body)) return text
  if (body.length <= 12) return `${prefix}***`
  return `${prefix}${body.slice(0, 6)}**********${body.slice(-6)}`
}

function parseAuditJson(value: unknown): unknown {
  if (!value || typeof value !== 'string') return value ?? null
  try {
    return JSON.parse(value)
  } catch {
    // A snapshot column that does not parse is still not worth failing a page
    // over; the row simply renders with whatever else it carries.
    return null
  }
}

/**
 * The audit row's three snapshots, flattened.
 *
 * `before_data` is written first and `after_data` last on purpose: where the two
 * overlap, the value the operator produced is the one worth showing.
 */
export function organizationAuditData(
  row: Partial<OrganizationAuditLogRow>
): Record<string, unknown> {
  const merged: Record<string, unknown> = {}
  for (const raw of [row.before_data, row.after_data, row.target_metadata]) {
    const value = parseAuditJson(raw)
    if (value && typeof value === 'object' && !Array.isArray(value)) {
      Object.assign(merged, value as Record<string, unknown>)
    }
  }
  return merged
}

function compactText(...values: unknown[]): string {
  return values
    .map((value) =>
      value === undefined || value === null ? '' : String(value).trim()
    )
    .filter(Boolean)
    .join(' · ')
}

export interface OrganizationAuditField {
  label: string
  value: string
  /** Keep the value on one line; ratios and ids should never wrap mid-token. */
  nowrap?: boolean
}

export interface OrganizationAuditTargetView {
  /** What the target cell shows. */
  main: string
  /** One line per property, rendered as the hover card. */
  details: OrganizationAuditField[]
  /** A key, masked, shown in an input so it can be read but not copied by accident. */
  apiKeyPreview: string
}

function field(
  label: string,
  value: unknown,
  options: { nowrap?: boolean } = {}
): OrganizationAuditField | null {
  if (value === undefined || value === null || value === '') return null
  return { label, value: String(value), nowrap: options.nowrap }
}

/** Turns one audit row into the target cell's content. */
export function buildOrganizationAuditTargetView(
  t: Translate,
  row: Partial<OrganizationAuditLogRow>
): OrganizationAuditTargetView {
  const targetType = row.target_type ?? ''
  const targetName = row.target_name ?? ''
  const data = organizationAuditData(row)
  const targetId = row.target_id

  if (targetType === 'organization') {
    const main = targetName || String(data.name ?? row.organization_name ?? '-')
    const slug = data.slug ?? row.organization_slug
    return {
      main,
      apiKeyPreview: '',
      details: [
        field(t('Name'), main, { nowrap: true }),
        field(t('Slug'), slug, { nowrap: true }),
      ].filter((line): line is OrganizationAuditField => line !== null),
    }
  }

  if (targetType === 'invite') {
    const main = String(data.target_email ?? targetName ?? '-')
    return {
      main,
      apiKeyPreview: '',
      details: [
        field(t('Email'), data.target_email, { nowrap: true }),
        field(t('Role'), data.role ? t(capitalizeRole(String(data.role))) : null),
        field(t('Username'), data.target_username ?? data.target_display_name),
      ].filter((line): line is OrganizationAuditField => line !== null),
    }
  }

  if (targetType === 'member') {
    const main = String(
      data.username ?? data.display_name ?? data.email ?? targetName ?? '-'
    )
    return {
      main,
      apiKeyPreview: '',
      details: [
        field(t('User ID'), data.user_id ?? targetId),
        field(t('Username'), data.username ?? data.display_name),
        field(t('Email'), data.email, { nowrap: true }),
        field(t('Role'), data.role ? t(capitalizeRole(String(data.role))) : null),
        field(t('Status'), data.status ? t(capitalizeStatus(String(data.status))) : null),
      ].filter((line): line is OrganizationAuditField => line !== null),
    }
  }

  if (targetType === 'token') {
    return buildOrganizationAuditTokenView(t, data, targetId)
  }

  if (targetType === 'quota_adjustment') {
    const main =
      compactText(
        data.quota_delta !== undefined
          ? t('Quota change: {{value}}', { value: data.quota_delta })
          : '',
        data.quota_before !== undefined && data.quota_after !== undefined
          ? t('Quota: {{before}} to {{after}}', {
              before: data.quota_before,
              after: data.quota_after,
            })
          : ''
      ) ||
      compactText(targetName, targetId ? `#${targetId}` : '') ||
      '-'
    return { main, apiKeyPreview: '', details: [] }
  }

  const main =
    compactText(targetName, targetId ? `#${targetId}` : '') || '-'
  return { main, apiKeyPreview: '', details: [] }
}

function capitalizeRole(role: string): string {
  const normalized = role.toLowerCase()
  if (normalized === 'owner') return 'Owner'
  if (normalized === 'admin') return 'Admin'
  if (normalized === 'member') return 'Member'
  return role
}

function capitalizeStatus(status: string): string {
  return status.charAt(0).toUpperCase() + status.slice(1)
}

/**
 * Token rows collapse into one of three shapes: a single key, a masked key, or
 * a batch that moved several keys between two members.
 */
function buildOrganizationAuditTokenView(
  t: Translate,
  data: Record<string, unknown>,
  targetId: number | undefined
): OrganizationAuditTargetView {
  const apiKey = String(data.api_key ?? '').trim()
  const tokenCountValue = data.token_count
  const isAggregate =
    !apiKey && tokenCountValue !== undefined && tokenCountValue !== null

  const apiKeyPreview = apiKey ? maskOrganizationAuditApiKey(apiKey) : ''
  const name = String(data.name ?? '')
  const main = isAggregate
    ? t('API keys transferred: {{count}}', { count: tokenCountValue })
    : name || '-'

  const details = [
    field(t('Target ID'), targetId, { nowrap: true }),
    apiKeyPreview ? field(t('API Key'), apiKeyPreview, { nowrap: true }) : null,
    field(t('API Key Name'), name, { nowrap: true }),
    field(
      t('Responsible User'),
      data.responsible_username ?? data.responsible_display_name,
      { nowrap: true }
    ),
    field(
      t('Creator'),
      data.creator_username ?? data.creator_display_name,
      { nowrap: true }
    ),
    isAggregate ? field(t('Key Count'), tokenCountValue) : null,
    isAggregate ? field(t('From User ID'), data.from_user_id) : null,
    isAggregate ? field(t('To User ID'), data.to_user_id) : null,
  ].filter((line): line is OrganizationAuditField => line !== null)

  return { main, apiKeyPreview, details }
}

// ============================================================================
// Reason
// ============================================================================

/**
 * A reason is free text, except for the one the backend writes itself
 * (`reopen from <status>`), which is a sentence fragment the user should not
 * have to read in English.
 */
export function formatOrganizationAuditReason(
  t: Translate,
  reason: unknown
): string {
  const text = String(reason ?? '').trim()
  if (!text) return '-'

  const reopenPrefix = 'reopen from '
  if (text.startsWith(reopenPrefix)) {
    const status = text.slice(reopenPrefix.length).trim() || 'unknown'
    return t('Reopened from {{status}}', { status })
  }
  return text
}
