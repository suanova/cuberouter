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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDataTable,
} from '@/components/data-table'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { ORGANIZATION_LIST_STATUSES, ORGANIZATION_STATUSES } from '../constants'
import { useOrganizationsQuery } from '../hooks/use-organizations-query'
import type { UserOrganization } from '../types'
import { useOrganizationsColumns } from './organizations-columns'
import { useOrganizations } from './organizations-provider'

const route = getRouteApi('/_authenticated/organizations/')

/** The list is short (one user barely exceeds a page), so it is filtered,
 * sorted and paginated in the browser and never refetched for it. */
export function OrganizationsTable() {
  const { t } = useTranslation()
  const columns = useOrganizationsColumns()
  const { refreshTrigger } = useOrganizations()
  const isMobile = useMediaQuery('(max-width: 640px)')

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: isMobile ? 10 : 20 },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'role', searchKey: 'role', type: 'array' },
    ],
  })

  // The rows carry the caller's role and capabilities for this organization, so
  // the response is different in each account context; the shared hook keys on
  // it.
  const { data, isLoading, isFetching } = useOrganizationsQuery(refreshTrigger)

  const { table } = useDataTable({
    data: data ?? [],
    columns,
    globalFilter,
    columnFilters,
    pagination,
    onGlobalFilterChange,
    onColumnFiltersChange,
    onPaginationChange,
    ensurePageInRange,
    globalFilterFn: (row, _columnId, filterValue) => {
      const search = String(filterValue).toLowerCase()
      return [row.original.name, row.original.description]
        .map((field) => String(field ?? '').toLowerCase())
        .some((field) => field.includes(search))
    },
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Organizations')}
      emptyDescription={t(
        'Organizations are created by platform administrators. Once you are invited to one, it will appear here.'
      )}
      skeletonKeyPrefix='organizations-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Filter by name or description...'),
        searchDebounceMs: 300,
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: ORGANIZATION_LIST_STATUSES.map((status) => ({
              label: t(ORGANIZATION_STATUSES[status].labelKey),
              value: status,
            })),
          },
          {
            columnId: 'role',
            title: t('My Role'),
            options: [
              { label: t('Owner'), value: 'owner' },
              { label: t('Admin'), value: 'admin' },
              { label: t('Member'), value: 'member' },
            ],
          },
        ],
      }}
      getRowClassName={(row, { isMobile: mobile }) =>
        getOrganizationRowClassName(row.original, mobile)
      }
    />
  )
}

function isInactiveOrganization(organization: UserOrganization) {
  return organization.status !== 'active'
}

/** A disabled or dissolved organization reads as inactive, not as an error. */
function getOrganizationRowClassName(
  organization: UserOrganization,
  isMobile: boolean
) {
  if (!isInactiveOrganization(organization)) return undefined
  return isMobile ? DISABLED_ROW_MOBILE : DISABLED_ROW_DESKTOP
}
