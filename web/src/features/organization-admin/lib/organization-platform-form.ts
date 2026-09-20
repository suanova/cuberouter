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

import {
  ORGANIZATION_DESCRIPTION_MAX_LENGTH,
  ORGANIZATION_NAME_MAX_LENGTH,
} from '@/features/organization/lib'
import type { OrganizationManagementView } from '@/features/organization/types'

import {
  platformOrganizationQuotaDelta,
  platformOrganizationRemainingQuota,
} from './organization-platform'

/**
 * The platform edit form.
 *
 * Unlike the organization center's own form, two of the fields are required: the
 * group, because an organization's group decides what every key in it can reach,
 * and the reason, because this form writes to an organization the administrator
 * does not belong to. The reason is what the organization's audit trail shows its
 * members, and a quota change with no recorded justification is not something
 * they can act on.
 *
 * The name limits match the create form so a name that was accepted at creation
 * can always be saved again here.
 */
export const platformOrganizationEditSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Name is required')
    .refine((value) => [...value].length <= ORGANIZATION_NAME_MAX_LENGTH, {
      message: `Name must be at most ${ORGANIZATION_NAME_MAX_LENGTH} characters`,
    }),
  description: z
    .string()
    .trim()
    .optional()
    .refine(
      (value) =>
        [...(value ?? '')].length <= ORGANIZATION_DESCRIPTION_MAX_LENGTH,
      {
        message: `Description must be at most ${ORGANIZATION_DESCRIPTION_MAX_LENGTH} characters`,
      }
    ),
  group: z.string().trim().min(1, 'Group is required'),
  /**
   * The remaining quota, in quota units — the unit the backend counts in, not
   * the currency amount shown beside it.
   *
   * Quota is deliberately *not* edited through the display amount the rest of
   * the console uses for quota inputs. Those inputs start from nothing and set a
   * value, so rounding the typed amount is harmless; this one starts from the
   * organization's current quota and has to be able to leave it untouched. A
   * round-trip through a display amount rounded to the currency's precision does
   * not come back to the same integer, so opening this drawer and saving without
   * touching the field would post a small adjustment and write a ledger row for
   * an edit that was never asked for. Integer units round-trip exactly, which
   * makes "unchanged" mean an adjustment of zero and no request at all.
   */
  remain_quota: z.coerce
    .number()
    .refine(Number.isFinite, { message: 'Quota must be a number' })
    .refine(Number.isInteger, { message: 'Quota must be a whole number' })
    .refine((value) => value >= 0, { message: 'Quota cannot be negative' }),
  reason: z.string().trim().min(1, 'Reason is required'),
})

export type PlatformOrganizationEditValues = z.infer<
  typeof platformOrganizationEditSchema
>

/** The form's starting point, taken from the row being edited. */
export function transformPlatformOrganizationToFormDefaults(
  organization: OrganizationManagementView
): PlatformOrganizationEditValues {
  return {
    name: organization.name,
    description: organization.description ?? '',
    group: organization.group || 'default',
    remain_quota: platformOrganizationRemainingQuota(organization),
    reason: '',
  }
}

/**
 * The two requests one save makes.
 *
 * They are separate because they are separate permissions on the backend: the
 * platform edit is `update_organization` and the quota change is
 * `adjust_organization_quota`. Sending an adjustment of zero when the operator
 * only renamed the organization would write a ledger row recording nothing — so
 * `quotaDelta` is `null` in that case and the caller sends only the edit.
 */
export function transformPlatformOrganizationEditToRequests(
  values: PlatformOrganizationEditValues,
  organization: OrganizationManagementView
): {
  edit: { name: string; description: string; group: string; reason: string }
  quotaDelta: number | null
} {
  const quotaDelta = platformOrganizationQuotaDelta(
    organization,
    values.remain_quota
  )

  return {
    edit: {
      name: values.name.trim(),
      description: values.description?.trim() ?? '',
      group: values.group.trim(),
      reason: values.reason.trim(),
    },
    quotaDelta: quotaDelta === 0 ? null : quotaDelta,
  }
}
