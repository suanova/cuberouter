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
import { z } from 'zod'

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import type { OrganizationTokenPayload } from '../api'
import type { OrganizationTokenRow } from '../types'

/** The backend refuses a batch larger than this; see `organization_token.go`. */
export const ORGANIZATION_TOKEN_MAX_BATCH = 100

export const organizationTokenFormSchema = z
  .object({
    name: z.string().trim().min(1, 'Key name is required'),
    /** How many keys to create at once. Ignored when editing. */
    token_count: z
      .number()
      .int()
      .min(1, 'Create at least one key')
      .max(
        ORGANIZATION_TOKEN_MAX_BATCH,
        `Create at most ${ORGANIZATION_TOKEN_MAX_BATCH} keys at once`
      ),
    visibility: z.enum(['private', 'public']),
    responsible_user_id: z.number().optional(),
    unlimited_quota: z.boolean(),
    remain_quota_dollars: z.number().optional(),
    /** Absent means the key never expires. */
    expired_time: z.date().optional(),
    model_limits: z.array(z.string()),
    allow_ips: z.string().optional(),
    group: z.string().optional(),
    cross_group_retry: z.boolean(),
  })
  .superRefine((values, ctx) => {
    if (
      !values.unlimited_quota &&
      (values.remain_quota_dollars === undefined ||
        values.remain_quota_dollars < 0)
    ) {
      ctx.addIssue({
        code: 'custom',
        path: ['remain_quota_dollars'],
        message: 'Quota must be zero or greater',
      })
    }

    // A key that expires in the past is dead the moment it is created, and the
    // picker defaults its time to midnight, so this is easy to do by accident.
    if (values.expired_time && values.expired_time.getTime() <= Date.now()) {
      ctx.addIssue({
        code: 'custom',
        path: ['expired_time'],
        message: 'Expiration time must be later than now',
      })
    }
  })

export type OrganizationTokenFormValues = z.infer<
  typeof organizationTokenFormSchema
>

export const ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES: OrganizationTokenFormValues =
  {
    name: '',
    token_count: 1,
    visibility: 'private',
    responsible_user_id: undefined,
    unlimited_quota: true,
    remain_quota_dollars: 0,
    expired_time: undefined,
    model_limits: [],
    allow_ips: '',
    group: '',
    cross_group_retry: false,
  }

/**
 * A key, as form values.
 *
 * `expired_time` of `-1` means never and becomes an empty picker rather than the
 * last second of 1969; every other value is the key's real deadline.
 */
export function transformOrganizationTokenToFormDefaults(
  token: OrganizationTokenRow
): OrganizationTokenFormValues {
  return {
    name: token.name,
    token_count: 1,
    visibility: token.visibility === 'public' ? 'public' : 'private',
    responsible_user_id: token.responsible_user_id ?? token.user_id,
    unlimited_quota: token.unlimited_quota,
    remain_quota_dollars: token.unlimited_quota
      ? 0
      : quotaUnitsToDollars(token.remain_quota),
    expired_time:
      token.expired_time > 0 ? new Date(token.expired_time * 1000) : undefined,
    model_limits: token.model_limits
      ? token.model_limits.split(',').filter(Boolean)
      : [],
    allow_ips: token.allow_ips ?? '',
    group: token.group ?? '',
    cross_group_retry: Boolean(token.cross_group_retry),
  }
}

export interface OrganizationTokenPayloadContext {
  /** The caller may set any responsible member and publish a key. */
  canManageAllTokens: boolean
  /** Present when editing an existing key. */
  editing?: OrganizationTokenRow
}

/**
 * Form values, as the body the create and update endpoints take.
 *
 * Two fields are not the caller's to choose unless they hold organization-wide
 * authority: a member can only be responsible for their own keys and cannot
 * publish one. The backend refuses both outright, so this repeats the rule
 * rather than relying on the form having hidden the controls — and on edit the
 * stored values are carried back unchanged for a member, so a save that only
 * touched the quota does not read as an attempted hand-over.
 *
 * `responsible_user_id` is omitted rather than zeroed when there is nothing to
 * say: the backend reads a zero as "work it out" and falls back to the operator
 * on create and to the current holder on update, which is what both callers
 * want.
 */
export function transformOrganizationTokenFormToPayload(
  values: OrganizationTokenFormValues,
  context: OrganizationTokenPayloadContext
): OrganizationTokenPayload {
  const editing = context.editing
  // A manager says who holds the key. Anyone else says nothing and lets the
  // backend keep the holder: it ignores the field for them anyway, and on an
  // edit the stored value is what must survive.
  let responsibleUserId: number | undefined
  if (context.canManageAllTokens) {
    responsibleUserId = values.responsible_user_id
  } else if (editing) {
    responsibleUserId = responsibleUserIdOf(editing)
  }

  return {
    name: values.name.trim(),
    status: editing ? editing.status : 1,
    expired_time: values.expired_time
      ? Math.floor(values.expired_time.getTime() / 1000)
      : -1,
    remain_quota: values.unlimited_quota
      ? 0
      : parseQuotaFromDollars(values.remain_quota_dollars ?? 0),
    unlimited_quota: values.unlimited_quota,
    model_limits_enabled: values.model_limits.length > 0,
    model_limits: values.model_limits.join(','),
    allow_ips: values.allow_ips ?? '',
    group: values.group ?? '',
    cross_group_retry:
      values.group === 'auto' ? Boolean(values.cross_group_retry) : false,
    visibility: context.canManageAllTokens
      ? values.visibility
      : (editing?.visibility ?? 'private'),
    responsible_user_id: responsibleUserId,
  }
}

/** The member a key currently belongs to. Zero is "nobody", not a match. */
function responsibleUserIdOf(token: OrganizationTokenRow): number {
  return token.responsible_user_id || token.user_id || 0
}

/**
 * The body for a `PATCH` that only changes the status.
 *
 * Updating replaces every field it names, so a bare `{ status }` would blank the
 * key's quota, expiry and allowlist. The row's stored values are sent back with
 * it, which is also what makes the enable path work: the backend re-checks the
 * responsible member while enabling.
 */
export function buildOrganizationTokenStatusPayload(
  token: OrganizationTokenRow,
  status: number
): OrganizationTokenPayload {
  return {
    name: token.name,
    status,
    expired_time: token.expired_time,
    remain_quota: token.remain_quota,
    unlimited_quota: token.unlimited_quota,
    model_limits_enabled: token.model_limits_enabled,
    model_limits: token.model_limits,
    allow_ips: token.allow_ips ?? '',
    group: token.group,
    cross_group_retry: token.cross_group_retry,
    visibility: token.visibility,
    responsible_user_id: responsibleUserIdOf(token),
  }
}

/** Whether the form's responsible member differs from the key's current one. */
export function isOrganizationTokenHandover(
  token: OrganizationTokenRow,
  responsibleUserId: number | undefined
): boolean {
  if (!responsibleUserId) return false
  return responsibleUserId !== responsibleUserIdOf(token)
}
