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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDataTable,
} from '@/components/data-table'
import { useMediaQuery } from '@/hooks'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import {
  listPlatformGroups,
  listPlatformOrganizations,
} from '@/features/organization/api'
import { ORGANIZATION_STATUSES } from '@/features/organization/constants'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '@/features/organization/lib'
import type { OrganizationManagementView } from '@/features/organization/types'

import {
  PLATFORM_ORGANIZATION_STATUSES,
  isPlatformOrganizationInactive,
  platformOrganizationStatusParam,
} from '../lib'
import { usePlatformOrganizationsColumns } from './platform-organizations-columns'
import { usePlatformOrganizations } from './platform-organizations-provider'

const route = getRouteApi('/_authenticated/admin/organizations/')

const PLATFORM_ORGANIZATIONS_COLUMN_VISIBILITY_STORAGE_KEY =
  'platform-organizations:column-visibility'

/**
 * Every organization on the platform.
 *
 * Paged and filtered by the server, unlike the organization center's own list: a
 * user belongs to a handful of organizations and the browser can hold them all,
 * while the platform holds every organization that exists. The filters and the
 * page number are kept in the URL, so the view an administrator is looking at is
 * a link they can send to another administrator.
 *
 * Nothing here reads or switches the account context. An administrator is not a
 * member of the organizations they are listing, so there is no context to
 * apply — the request carries the administrator's own identity and the platform
 * prefix says what it is for.
 */
export function PlatformOrganizationsTable() {
  const { t } = useTranslation()
  const { refreshTrigger } = usePlatformOrganizations()
  const isMobile = useMediaQuery('(max-width: 640px)')
  const isRoot =
    useAuthStore((state) => state.auth.user?.role ?? 0) >= ROLE.SUPER_ADMIN

  const columns = usePlatformOrganizationsColumns({ isRoot })
  const groupFilterOptions = usePlatformGroupFilterOptions()

  const {
    globalFilter,
    columnFilters,
    pagination,
    onPaginationChange,
    onColumnFiltersChange,
    onGlobalFilterChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: {
      defaultPage: 1,
      defaultPageSize: isMobile ? 10 : ORGANIZATION_DEFAULT_PAGE_SIZE,
    },
    globalFilter: { enabled: true, key: 'filter' },
    columnFilters: [
      // The status is a selection rather than one value, because an
      // administrator triaging the list is usually interested in more than one
      // state and the endpoint accepts a comma-joined set. No default is
      // seeded: an empty selection and all three statuses are the same request,
      // and the endpoint already reads an empty set that way.
      { columnId: 'status', searchKey: 'status', type: 'array' },
      { columnId: 'group', searchKey: 'group', type: 'string' },
    ],
  })

  const statusFilter =
    (columnFilters.find((filter) => filter.id === 'status')?.value as
      | string[]
      | undefined) ?? []
  const groupFilter =
    (columnFilters.find((filter) => filter.id === 'group')?.value as string) ??
    ''

  const { data, isLoading, isFetching } = useQuery({
    queryKey: [
      'platform-organizations',
      pagination.pageIndex + 1,
      pagination.pageSize,
      globalFilter,
      statusFilter,
      groupFilter,
      refreshTrigger,
    ],
    queryFn: async (): Promise<{
      items: OrganizationManagementView[]
      total: number
    }> => {
      const result = await listPlatformOrganizations({
        p: pagination.pageIndex + 1,
        page_size: pagination.pageSize,
        keyword: globalFilter,
        status: platformOrganizationStatusParam(statusFilter),
        group: groupFilter,
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to load organizations'))
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const organizations = data?.items ?? []

  const { table } = useDataTable({
    data: organizations,
    columns,
    columnFilters,
    globalFilter,
    pagination,
    onPaginationChange,
    onColumnFiltersChange,
    onGlobalFilterChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange,
    columnVisibilityStorageKey:
      PLATFORM_ORGANIZATIONS_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No Organizations Found')}
      emptyDescription={t(
        'No organization matches the current filters. Try widening them or clearing the keyword.'
      )}
      skeletonKeyPrefix='platform-organizations-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Filter by name, slug or owner...'),
        searchDebounceMs: 300,
        filters: [
          {
            columnId: 'status',
            title: t('Status'),
            options: PLATFORM_ORGANIZATION_STATUSES.map((status) => ({
              label: t(ORGANIZATION_STATUSES[status].labelKey),
              value: status,
            })),
          },
          {
            columnId: 'group',
            title: t('Group'),
            options: groupFilterOptions,
            singleSelect: true,
          },
        ],
      }}
      getRowClassName={(row, { isMobile: mobile }) => {
        if (!isPlatformOrganizationInactive(row.original)) return undefined

        return mobile ? DISABLED_ROW_MOBILE : DISABLED_ROW_DESKTOP
      }}
    />
  )
}

/**
 * The groups a filter can select.
 *
 * Read once for the toolbar and shared across renders — the platform's group
 * list changes when an administrator adds a group, not while a list is being
 * read — and degrading to no options rather than failing the page: a filter
 * with nothing to choose from is better than a table that will not load.
 */
function usePlatformGroupFilterOptions(): Array<{
  label: string
  value: string
}> {
  const { data } = useQuery({
    queryKey: ['platform-groups'],
    queryFn: async () => {
      const result = await listPlatformGroups()
      return result.data ?? []
    },
    staleTime: 5 * 60 * 1000,
  })

  return (data ?? []).map((group) => ({ label: group, value: group }))
}
