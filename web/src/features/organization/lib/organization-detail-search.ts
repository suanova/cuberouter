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
import z from 'zod'

/**
 * A multi-select filter that arrives as `?x=a` for a single choice and as
 * `?x=a&x=b` for several. Both spellings resolve to an array so the table
 * filters do not have to care which one a hand-edited URL used.
 *
 * Enumerated where the backend's value set is closed, free-form where the set
 * comes from data the deployment defines — a group name is as open-ended as the
 * task platforms an installation has configured, and narrowing those in the
 * schema would turn an unlisted value into an empty table.
 */
function enumArrayParam(values: readonly [string, ...string[]]) {
  return z
    .preprocess(
      (value) => {
        if (value == null || value === '') return undefined
        return Array.isArray(value) ? value : [value]
      },
      z.array(z.enum(values)).optional()
    )
    .catch([])
}

function freeArrayParam() {
  return z
    .preprocess(
      (value) => {
        if (value == null || value === '') return undefined
        return Array.isArray(value) ? value : [value]
      },
      z.array(z.string()).optional()
    )
    .catch([])
}

export const ORGANIZATION_MEMBER_STATUS_VALUES = [
  'active',
  'disabled',
  'exited',
  'removed',
] as const

export const ORGANIZATION_MEMBER_ROLE_VALUES = [
  'owner',
  'admin',
  'member',
] as const

export const ORGANIZATION_INVITE_STATUS_VALUES = [
  'pending',
  'accepted',
  'expired',
  'revoked',
] as const

/**
 * `model.Token.Status`: the four values the backend can store. They stay
 * strings here because that is what a URL carries and what the filter compares.
 */
export const ORGANIZATION_TOKEN_STATUS_VALUES = [
  '1',
  '2',
  '3',
  '4',
] as const

export const ORGANIZATION_TOKEN_VISIBILITY_VALUES = [
  'private',
  'public',
] as const

/**
 * Every search parameter the organization detail page reads.
 *
 * One schema covers all nine sections rather than one each: the route owns the
 * schema, and a section that stopped declaring its parameters would silently
 * drop them from the URL. Keys are prefixed per section so two sections can
 * never read each other's filter, and the two timestamp keys are shared because
 * the logs, tasks, usage and audit sections all want the same time window.
 *
 * Each field is `.optional().catch(...)`: a hand-edited or truncated link must
 * narrow the filter, never blank the page.
 */
export const organizationDetailSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),

  // Members
  memberFilter: z.string().optional().catch(''),
  memberRole: enumArrayParam(ORGANIZATION_MEMBER_ROLE_VALUES).optional(),
  memberStatus: enumArrayParam(ORGANIZATION_MEMBER_STATUS_VALUES).optional(),

  // Invitations
  inviteFilter: z.string().optional().catch(''),
  inviteStatus: enumArrayParam(ORGANIZATION_INVITE_STATUS_VALUES).optional(),

  // API keys
  tokenFilter: z.string().optional().catch(''),
  tokenStatus: enumArrayParam(ORGANIZATION_TOKEN_STATUS_VALUES).optional(),
  tokenVisibility: enumArrayParam(ORGANIZATION_TOKEN_VISIBILITY_VALUES).optional(),

  // Logs
  logFilter: z.string().optional().catch(''),
  logType: freeArrayParam().optional(),
  logModel: z.string().optional().catch(''),
  logToken: z.string().optional().catch(''),
  logGroup: z.string().optional().catch(''),
  logRequest: z.string().optional().catch(''),
  logResponsible: z.string().optional().catch(''),

  // Tasks and Midjourney tasks
  taskFilter: z.string().optional().catch(''),
  taskPlatform: freeArrayParam().optional(),
  taskStatus: z.string().optional().catch(''),
  taskAction: z.string().optional().catch(''),

  // Audit trail
  auditFilter: z.string().optional().catch(''),
  auditAction: freeArrayParam().optional(),
  auditTargetType: freeArrayParam().optional(),

  // Billing / usage
  billingMonth: z.string().optional().catch(''),

  // Time window shared by logs, tasks, usage and audit
  startTime: z.number().optional().catch(undefined),
  endTime: z.number().optional().catch(undefined),
})

export type OrganizationDetailSearch = z.infer<
  typeof organizationDetailSearchSchema
>
