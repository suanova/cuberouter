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
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { listOrganizationAuditLogs } from '../api'
import {
  organizationAuditActionOptions,
  organizationAuditTargetTypeOptions,
  organizationColumnFilterValue,
  type Translate,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import type { OrganizationAuditLogRow } from '../types'
import { buildOrganizationAuditColumns } from './organization-audit-columns'
import {
  useOrganizationSectionRoute,
  useOrganizationSurface,
} from './organization-page-provider'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
} from './organization-section'

const AUDIT_COLUMN_VISIBILITY_STORAGE_KEY = 'organization-audit-column-visibility'

type OrganizationAuditSectionProps = {
  organizationId: number
  /** The caller may read the organization's audit trail. */
  canViewAudit: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * The organization's audit trail: who did what, to which object, and why.
 *
 * Read-only by construction — the backend writes these records, and there is no
 * endpoint to amend them.
 */
export function OrganizationAuditSection(props: OrganizationAuditSectionProps) {
  const { t } = useTranslation()
  // The translate helpers are plain functions so they can be unit tested; this
  // adapts the component's `t` to that narrower signature.
  const translate: Translate = (key, options) => t(key, options)
  const { search, navigate } = useOrganizationSectionRoute()
  const surface = useOrganizationSurface()
  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: { defaultPage: 1, defaultPageSize: ORGANIZATION_DEFAULT_PAGE_SIZE },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'action_type', searchKey: 'auditAction', type: 'array' },
      {
        columnId: 'target_type',
        searchKey: 'auditTargetType',
        type: 'array',
      },
    ],
  })

  const params = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    action_type: organizationColumnFilterValue(columnFilters, 'action_type'),
    target_type: organizationColumnFilterValue(columnFilters, 'target_type'),
  }

  const audit = useOrganizationPagedSection<OrganizationAuditLogRow>({
    surface,
    organizationId: props.organizationId,
    resource: 'audit-logs',
    params,
    query: () =>
      listOrganizationAuditLogs(surface, props.organizationId, {
        p: params.p,
        page_size: params.page_size,
        action_type: params.action_type,
        target_type: params.target_type,
      }),
    enabled: props.canViewAudit,
    onForbidden: props.onForbidden,
  })

  const columns = buildOrganizationAuditColumns(translate)

  const { table } = useDataTable({
    data: audit.items,
    columns,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: audit.total,
    ensurePageInRange,
    columnVisibilityStorageKey: AUDIT_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  if (!props.canViewAudit) {
    return (
      <OrganizationSectionEmpty
        icon='audit-logs'
        title={t('Audit Logs')}
        message={t(
          'Only an organization owner or administrator can read the audit trail.'
        )}
      />
    )
  }

  return (
    <OrganizationSection
      icon='audit-logs'
      title={t('Audit Logs')}
      description={t(
        'Every change made to this organization, its members, its API keys and its invitations.'
      )}
      count={audit.total}
    >
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={audit.isLoading}
        isFetching={audit.isFetching}
        emptyTitle={t('No audit records yet')}
        skeletonKeyPrefix='organization-audit-skeleton'
        applyHeaderSize
        toolbarProps={{
          // The audit endpoint has no keyword search: it filters by action,
          // target, operator and time, all of which are exact matches.
          customSearch: null,
          hideViewOptions: true,
          filters: [
            {
              columnId: 'action_type',
              title: t('Action'),
              options: organizationAuditActionOptions(translate),
              singleSelect: true,
            },
            {
              columnId: 'target_type',
              title: t('Target Type'),
              options: organizationAuditTargetTypeOptions(translate),
              singleSelect: true,
            },
          ],
        }}
      />
    </OrganizationSection>
  )
}

