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

import type { OrganizationAuditLogParams } from '@/features/organization/api'
import { organizationColumnFilterValue } from '@/features/organization/lib'
import type { OrganizationAuditLogRow } from '@/features/organization/types'

/**
 * The platform audit page's view state.
 *
 * The read filters on one exact value per field, so each filter is one value or
 * none: `organization` is a slug rather than a keyword, and the two closed sets
 * are single choices. All of them live in the URL, because the reason to read
 * this page is usually to show someone else what happened.
 */
export const platformOrganizationAuditSearchSchema = z.object({
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
  organization: z.string().optional().catch(''),
  action: z.array(z.string()).optional().catch([]),
  targetType: z.array(z.string()).optional().catch([]),
})

export type PlatformOrganizationAuditSearch = z.infer<
  typeof platformOrganizationAuditSearchSchema
>

/**
 * What the cross-organization read actually sends.
 *
 * The table holds a filter as an array whatever the endpoint does with it — the
 * shared multi-select does — so each one is reduced to the single value the
 * backend compares against, and a cleared filter becomes an absent parameter
 * rather than an empty one.
 */
export function platformOrganizationAuditQuery(input: {
  columnFilters: Array<{ id: string; value: unknown }>
  page: number
  pageSize: number
}): OrganizationAuditLogParams {
  return {
    p: input.page,
    page_size: input.pageSize,
    organization_slug: organizationColumnFilterValue(
      input.columnFilters,
      'organization_slug'
    ),
    action_type: organizationColumnFilterValue(
      input.columnFilters,
      'action_type'
    ),
    target_type: organizationColumnFilterValue(
      input.columnFilters,
      'target_type'
    ),
  }
}

/**
 * Which organization a cross-organization audit row is about.
 *
 * The name and the slug are written onto the record when it is written, not
 * read from the organization now, so a renamed organization leaves its older
 * entries naming it as it was. That is the record of what happened being kept
 * as it happened, not a stale field to correct — but it does mean the name
 * cannot be treated as the organization's current one, which is why the slug
 * and the id are here with it.
 */
export function platformOrganizationAuditOrganizationName(
  row: Pick<OrganizationAuditLogRow, 'organization_id' | 'organization_name' | 'organization_slug'>
): string {
  const name = String(row.organization_name ?? '').trim()
  if (name) return name

  const slug = String(row.organization_slug ?? '').trim()
  if (slug) return slug

  const id = Number(row.organization_id ?? 0)
  return id > 0 ? `#${id}` : '-'
}
