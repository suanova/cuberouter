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
import type { ColumnDef } from '@tanstack/react-table'
import { useQuery } from '@tanstack/react-query'
import { Link, getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { SectionPageLayout } from '@/components/layout'
import { Input } from '@/components/ui/input'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { listAllOrganizationAuditLogs } from '@/features/organization/api'
import { buildOrganizationAuditColumns } from '@/features/organization/components/organization-audit-columns'
import {
  ORGANIZATION_DEFAULT_PAGE_SIZE,
  organizationAuditActionOptions,
  organizationAuditTargetTypeOptions,
  setOrganizationTextFilter,
  type Translate,
} from '@/features/organization/lib'
import type { OrganizationAuditLogRow } from '@/features/organization/types'

import {
  platformOrganizationAuditOrganizationName,
  platformOrganizationAuditQuery,
} from '../lib'

const route = getRouteApi('/_authenticated/admin/organization-audit-logs')

const PLATFORM_AUDIT_COLUMN_VISIBILITY_STORAGE_KEY =
  'platform-organization-audit-column-visibility'

/**
 * The platform's audit trail across every organization.
 *
 * An organization's own page answers "what happened inside this one"; this
 * answers "who did that, and where else have they been" — the question an
 * administrator arrives with when a record has to be explained rather than
 * browsed. Nothing here is written to: the records are the backend's, and there
 * is no endpoint to amend them.
 */
export function PlatformOrganizationAuditLog() {
  const { t } = useTranslation()
  // The translate helpers are plain functions so they can be unit tested; this
  // adapts the component's `t` to that narrower signature.
  const translate: Translate = (key, options) => t(key, options)
  const navigate = route.useNavigate()
  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate,
    pagination: { defaultPage: 1, defaultPageSize: ORGANIZATION_DEFAULT_PAGE_SIZE },
    globalFilter: { enabled: false },
    columnFilters: [
      {
        columnId: 'organization_slug',
        searchKey: 'organization',
        type: 'string',
      },
      { columnId: 'action_type', searchKey: 'action', type: 'array' },
      { columnId: 'target_type', searchKey: 'targetType', type: 'array' },
    ],
  })

  const params = platformOrganizationAuditQuery({
    columnFilters,
    page: pagination.pageIndex + 1,
    pageSize: pagination.pageSize,
  })

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['platform-organization-audit-logs', params],
    queryFn: async (): Promise<{
      items: OrganizationAuditLogRow[]
      total: number
    }> => {
      const result = await listAllOrganizationAuditLogs(params)
      if (!result.success) {
        toast.error(result.message || t('Failed to load audit records'))
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const rows = data?.items ?? []
  const columns = buildPlatformAuditColumns(translate)

  const { table } = useDataTable({
    data: rows,
    columns,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange,
    columnVisibilityStorageKey: PLATFORM_AUDIT_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  const slugFilter =
    (columnFilters.find((filter) => filter.id === 'organization_slug')
      ?.value as string) ?? ''

  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        {t('Organization Audit Log')}
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <DataTablePage
          table={table}
          columns={columns}
          isLoading={isLoading}
          isFetching={isFetching}
          emptyTitle={t('No audit records yet')}
          emptyDescription={t(
            'No audit record matches the current filters. Try widening them.'
          )}
          skeletonKeyPrefix='platform-organization-audit-skeleton'
          applyHeaderSize
          toolbarProps={{
            // The read has no keyword search: it filters by organization,
            // action and target, all of them exact matches on one value.
            customSearch: null,
            hideViewOptions: true,
            additionalSearch: (
              <Input
                // The backend compares the slug exactly, so this is a slug box
                // rather than a name search — and it says so.
                placeholder={t('Filter by organization slug...')}
                aria-label={t('Filter by organization slug...')}
                value={slugFilter}
                onChange={(event) =>
                  setOrganizationTextFilter(
                    onColumnFiltersChange,
                    columnFilters,
                    'organization_slug',
                    event.target.value
                  )
                }
                className='w-full sm:w-50 lg:w-60'
              />
            ),
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
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}

/**
 * The shared audit row, with the one thing a single organization's trail cannot
 * vary named in front of it.
 *
 * The organization links to its own platform page, because the next question
 * after "what is this record" is "what is this organization" — and the answer
 * is one click rather than a copy of the slug into another page's filter.
 */
function buildPlatformAuditColumns(
  translate: Translate
): ColumnDef<OrganizationAuditLogRow>[] {
  const t = translate

  return [
    {
      accessorKey: 'organization_name',
      header: t('Organization'),
      enableSorting: false,
      size: 200,
      cell: ({ row }) => (
        <Link
          to='/admin/organizations/$organizationId/$section'
          params={{
            organizationId: String(row.original.organization_id),
            section: 'overview',
          }}
          className='font-medium hover:underline'
        >
          {platformOrganizationAuditOrganizationName(row.original)}
        </Link>
      ),
    },
    ...buildOrganizationAuditColumns(translate),
  ]
}
