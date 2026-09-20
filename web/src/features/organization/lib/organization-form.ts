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

import type { Organization } from '../types'

/** Enforced by the backend on update; applied to create as well so a name that
 * was accepted at creation can always be saved again later. */
export const ORGANIZATION_NAME_MAX_LENGTH = 64

/** The `name` column is varchar(512). */
export const ORGANIZATION_DESCRIPTION_MAX_LENGTH = 512

/**
 * Length in code points, matching Go's `utf8.RuneCountInString`. `String.length`
 * counts UTF-16 code units, so an astral character such as an emoji counts twice
 * and the form would refuse a name the backend accepts.
 */
function withinCodePoints(value: string, limit: number): boolean {
  return [...value].length <= limit
}

function codePointLimitMessage(label: string, limit: number): string {
  return `${label} must be at most ${limit} characters`
}

// ============================================================================
// Organization Form
// ============================================================================

export const organizationFormSchema = z.object({
  name: z
    .string()
    .trim()
    .min(1, 'Name is required')
    .refine((value) => withinCodePoints(value, ORGANIZATION_NAME_MAX_LENGTH), {
      message: codePointLimitMessage('Name', ORGANIZATION_NAME_MAX_LENGTH),
    }),
  description: z
    .string()
    .trim()
    .optional()
    .refine(
      (value) =>
        withinCodePoints(value ?? '', ORGANIZATION_DESCRIPTION_MAX_LENGTH),
      {
        message: codePointLimitMessage(
          'Description',
          ORGANIZATION_DESCRIPTION_MAX_LENGTH
        ),
      }
    ),
  group: z.string().optional(),
  reason: z.string().trim().optional(),
})

export type OrganizationFormValues = z.infer<typeof organizationFormSchema>

export const ORGANIZATION_FORM_DEFAULT_VALUES: OrganizationFormValues = {
  name: '',
  description: '',
  group: '',
  reason: '',
}

export function transformOrganizationToFormDefaults(
  organization: Pick<Organization, 'name' | 'description' | 'group'>
): OrganizationFormValues {
  return {
    name: organization.name,
    description: organization.description ?? '',
    group: organization.group ?? '',
    reason: '',
  }
}

/**
 * The create endpoint ignores `group` (a new organization always starts on the
 * default group) and the update endpoint ignores it unless the caller may change
 * the organization group, so the field is only sent when it is actually
 * editable.
 */
export function transformOrganizationFormToPayload(
  values: OrganizationFormValues,
  options: { organizationId?: number; includeGroup?: boolean } = {}
): {
  name: string
  description: string
  group?: string
  reason?: string
} {
  const payload: {
    name: string
    description: string
    group?: string
    reason?: string
  } = {
    name: values.name.trim(),
    description: values.description?.trim() ?? '',
  }

  if (options.organizationId !== undefined) {
    if (options.includeGroup) payload.group = values.group?.trim() || 'default'
    const reason = values.reason?.trim()
    if (reason) payload.reason = reason
  }

  return payload
}

// ============================================================================
// Destructive Actions
// ============================================================================

/**
 * Both the status change and the dissolve make the operator retype the
 * organization's slug.
 *
 * The slug and not the name, because that is what the backend compares against:
 * `setOrganizationStatus` accepts only `organization.Slug`, and
 * `DissolveOrganization` accepts the name or the slug. The slug is the narrower
 * of the two and it is also the stable one — a rename does not change it — so
 * the dialogs show it and ask for it.
 *
 * Compared trimmed because the backend trims before comparing, so a trailing
 * space must not block the action. An empty slug never matches: an organization
 * always has one, and `'' === ''` would otherwise let the dialog through.
 */
export function isOrganizationSlugConfirmed(
  expectedSlug: string | undefined,
  input: string
): boolean {
  const expected = expectedSlug?.trim() ?? ''
  if (!expected) return false
  return input.trim() === expected
}

// ============================================================================
// Idempotency
// ============================================================================

/**
 * Dissolve and owner transfer are retryable but must not apply twice, so each
 * is sent with an `Idempotency-Key`. One key per intent: it is generated when
 * the dialog opens and reused for every retry the user makes from that dialog,
 * and a new key is generated the next time the dialog is opened.
 */
export function createIdempotencyKey(): string {
  const cryptoApi = globalThis.crypto
  if (typeof cryptoApi?.randomUUID === 'function') {
    return cryptoApi.randomUUID()
  }
  return `org-${Date.now()}-${Math.random().toString(36).slice(2, 12)}`
}

// ============================================================================
// Group Options
// ============================================================================

/** Shape accepted by ApiKeyGroupCombobox, so the picker can be reused as-is. */
export interface OrganizationGroupOption {
  value: string
  label: string
  desc?: string
  ratio?: number | string
}

/**
 * The groups endpoint returns one entry per group the organization may use,
 * keyed by group name. Sorted here rather than trusting map order, which is
 * insertion order and therefore unstable across responses.
 */
export function buildOrganizationGroupOptions(
  groups: Record<string, { desc?: string; ratio?: number | string }> = {}
): OrganizationGroupOption[] {
  return Object.entries(groups)
    .map(([value, group]) => ({
      value,
      label: value,
      desc: group?.desc,
      ratio: group?.ratio,
    }))
    .sort((a, b) => a.value.localeCompare(b.value))
}
