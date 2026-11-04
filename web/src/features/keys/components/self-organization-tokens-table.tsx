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
import { getRouteApi, Link } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  DataTablePage,
  useDataTable,
  useDebouncedColumnFilter,
} from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Input } from '@/components/ui/input'
import { getOrganizations } from '@/features/organization/api'
import { OrganizationTokenKeyCell } from '@/features/organization/components/organization-token-key-cell'
import {
  ORGANIZATION_TOKEN_STATUS,
  ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS,
  ORGANIZATION_TOKEN_VISIBILITIES,
  organizationTokenStatusMeta,
  organizationTokenVisibilityMeta,
} from '@/features/organization/lib'
import type { OrganizationTokenRow } from '@/features/organization/types'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatQuota, formatTimestamp } from '@/lib/format'

import { getSelfOrganizationTokens } from '../api'

const route = getRouteApi('/_authenticated/keys/')
const COLUMN_VISIBILITY_STORAGE_KEY =
  'self-organization-tokens-column-visibility'

function firstFilterValue(
  columnFilters: Array<{ id: string; value: unknown }>,
  columnId: string
): string | undefined {
  const value = columnFilters.find((filter) => filter.id === columnId)?.value
  if (Array.isArray(value)) return value[0] as string | undefined
  return typeof value === 'string' && value ? value : undefined
}

function buildColumns(
  t: (key: string) => string
): ColumnDef<OrganizationTokenRow>[] {
  return [
    {
      id: 'organization_id',
      accessorKey: 'organization_id',
      header: t('Organization'),
      enableSorting: false,
      size: 140,
      cell: ({ row }) => (
        <span className='truncate font-medium'>
          {row.original.organization_name || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'name',
      header: t('Name'),
      enableSorting: false,
      size: 180,
      meta: { mobileTitle: true },
      cell: ({ row }) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>{row.original.name || '-'}</div>
          <div className='text-muted-foreground text-xs'>
            #{row.original.id}
          </div>
        </div>
      ),
    },
    {
      accessorKey: 'key',
      header: t('Key'),
      enableSorting: false,
      size: 250,
      cell: ({ row }) => (
        <OrganizationTokenKeyCell token={row.original} canCopy />
      ),
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      enableSorting: false,
      size: 110,
      meta: { mobileBadge: true },
      cell: ({ row }) => {
        const meta = organizationTokenStatusMeta(row.original.status)
        return (
          <StatusBadge
            label={t(meta.labelKey)}
            variant={meta.variant}
            copyable={false}
          />
        )
      },
    },
    {
      accessorKey: 'visibility',
      header: t('Visibility'),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <span className='text-sm'>
          {t(organizationTokenVisibilityMeta(row.original.visibility).labelKey)}
        </span>
      ),
    },
    {
      accessorKey: 'responsible_user_id',
      header: t('Responsible user'),
      enableSorting: false,
      size: 140,
      cell: ({ row }) => (
        <span className='truncate'>
          {row.original.responsible_username ||
            row.original.responsible_display_name ||
            '-'}
        </span>
      ),
    },
    {
      accessorKey: 'remain_quota',
      header: t('Remaining quota'),
      enableSorting: false,
      size: 130,
      cell: ({ row }) =>
        row.original.unlimited_quota ? (
          <Badge variant='secondary'>{t('Unlimited')}</Badge>
        ) : (
          <span className='whitespace-nowrap tabular-nums'>
            {formatQuota(row.original.remain_quota ?? 0)}
          </span>
        ),
    },
    {
      accessorKey: 'used_quota',
      header: t('Used Quota'),
      enableSorting: false,
      size: 120,
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='whitespace-nowrap tabular-nums'>
          {formatQuota(row.original.used_quota ?? 0)}
        </span>
      ),
    },
    {
      accessorKey: 'group',
      header: t('Group'),
      enableSorting: false,
      size: 140,
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='truncate'>{row.original.group || '-'}</span>
      ),
    },
    {
      accessorKey: 'expired_time',
      header: t('Expires at'),
      enableSorting: false,
      size: 150,
      meta: { mobileHidden: true },
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {!row.original.expired_time || row.original.expired_time === -1
            ? t('Never expires')
            : formatTimestamp(row.original.expired_time)}
        </span>
      ),
    },
    {
      id: 'actions',
      header: t('Actions'),
      enableSorting: false,
      enableHiding: false,
      size: 170,
      cell: ({ row }) =>
        row.original.organization_id ? (
          <Link
            to='/organizations/$organizationId/$section'
            params={{
              organizationId: String(row.original.organization_id),
              section: 'tokens',
            }}
            className='text-primary text-xs underline decoration-dotted underline-offset-2'
          >
            {t('Manage in organization')}
          </Link>
        ) : (
          <span className='text-muted-foreground'>-</span>
        ),
    },
  ]
}

/**
 * The organization keys a member can use, across every organization they
 * belong to. This view is deliberately read-only: creating, editing and
 * deleting an organization key belongs to the organization's own page, where
 * the organization account context and its per-key authority are in force.
 */
export function SelfOrganizationTokensTable() {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const {
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: {
      pageKey: 'orgPage',
      pageSizeKey: 'orgPageSize',
      defaultPage: 1,
      defaultPageSize: 20,
    },
    globalFilter: { enabled: true, key: 'orgFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'orgStatus', type: 'array' },
      { columnId: 'visibility', searchKey: 'orgVisibility', type: 'array' },
      {
        columnId: 'organization_id',
        searchKey: 'orgOrganization',
        type: 'array',
      },
      { columnId: '_tokenSearch', searchKey: 'orgToken', type: 'string' },
    ],
  })

  const {
    value: tokenFilter,
    inputValue: tokenFilterInput,
    setInputValue: setTokenFilterInput,
  } = useDebouncedColumnFilter({
    columnFilters,
    columnId: '_tokenSearch',
    onColumnFiltersChange,
  })

  const status = Number(firstFilterValue(columnFilters, 'status')) || undefined
  const visibility = firstFilterValue(columnFilters, 'visibility')
  const organizationId =
    Number(firstFilterValue(columnFilters, 'organization_id')) || undefined

  const params = {
    p: pagination.pageIndex + 1,
    size: pagination.pageSize,
    keyword: globalFilter?.trim() || undefined,
    token: tokenFilter.trim() || undefined,
    status,
    visibility,
    organization_id: organizationId,
  }

  const { data, isLoading, isFetching } = useQuery({
    queryKey: ['self-organization-tokens', params],
    queryFn: async () => {
      const result = await getSelfOrganizationTokens(params)
      if (!result.success) {
        toast.error(result.message || t('Failed to load'))
        return { items: [], total: 0 }
      }
      return {
        items: result.data?.items ?? [],
        total: result.data?.total ?? 0,
      }
    },
    placeholderData: (previousData) => previousData,
  })

  const organizationsQuery = useQuery({
    queryKey: ['self-organization-tokens', 'organizations'],
    queryFn: getOrganizations,
    staleTime: 5 * 60 * 1000,
  })
  const organizations = organizationsQuery.data?.data ?? []

  const columns = useMemo(() => buildColumns(t), [t])

  const { table } = useDataTable({
    data: data?.items ?? [],
    columns,
    columnFilters,
    onColumnFiltersChange,
    globalFilter,
    globalFilterFn: () => true,
    onGlobalFilterChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: data?.total ?? 0,
    ensurePageInRange,
    columnVisibilityStorageKey: COLUMN_VISIBILITY_STORAGE_KEY,
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No organization keys')}
      emptyDescription={t(
        'Keys from organizations you belong to will appear here.'
      )}
      skeletonKeyPrefix='self-organization-tokens-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Search by key name...'),
        searchDebounceMs: 500,
        additionalSearch: (
          <Input
            placeholder={t('Filter by API key...')}
            aria-label={t('Filter by API key...')}
            value={tokenFilterInput}
            onChange={(event) => setTokenFilterInput(event.target.value)}
            className='w-full sm:w-50 lg:w-60'
          />
        ),
        filters: [
          {
            columnId: 'organization_id',
            title: t('Organization'),
            singleSelect: true,
            options: organizations.map((organization) => ({
              value: String(organization.id),
              label: organization.name,
            })),
          },
          {
            columnId: 'status',
            title: t('Status'),
            singleSelect: true,
            options: ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS.map((option) => ({
              value: option.value,
              label: t(option.labelKey),
            })),
          },
          {
            columnId: 'visibility',
            title: t('Visibility'),
            singleSelect: true,
            options: Object.entries(ORGANIZATION_TOKEN_VISIBILITIES).map(
              ([value, meta]) => ({
                value,
                label: t(meta.labelKey),
              })
            ),
          },
        ],
        hasAdditionalFilters: tokenFilter.trim() !== '',
        onReset: () => setTokenFilterInput(''),
      }}
      getRowClassName={(row, { isMobile }) => {
        if (row.original.status === ORGANIZATION_TOKEN_STATUS.ENABLED) {
          return undefined
        }
        return isMobile ? DISABLED_ROW_MOBILE : DISABLED_ROW_DESKTOP
      }}
    />
  )
}
