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
import { Download } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  LogsFilterField,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { formatQuota, formatTimestamp } from '@/lib/format'

import { listOrganizationBillingDetails } from '../api'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import {
  buildOrganizationBillingCsv,
  collectOrganizationRows,
  currentOrganizationBillingMonth,
  downloadOrganizationBillingCsv,
  formatOrganizationBillingExportTime,
  organizationBillingCsvFilename,
  organizationBillingDetailFilters,
  organizationBillingLedgerDelta,
  organizationBillingRecordTypeMeta,
  organizationBillingResponsibleName,
  organizationColumnFilterValue,
  recentOrganizationBillingMonths,
  setOrganizationTextFilter,
  type OrganizationCsvColumn,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationBillingRecord } from '../types'
import { OrganizationTextFilter } from './organization-filter-fields'
import {
  useOrganizationSectionRoute,
  useOrganizationSurface,
} from './organization-page-provider'

const BILLING_COLUMN_VISIBILITY_STORAGE_KEY =
  'organization-billing-column-visibility'

/**
 * The month select's value for "do not filter by month".
 *
 * Spelled out in the URL rather than left out of it, because an absent month is
 * already taken: the panel opens on the current month, and clearing the filter
 * has to be a different state from never having chosen one.
 */
const ALL_MONTHS = 'all'

type OrganizationBillingDetailsProps = {
  organizationId: number
  /** The caller reads every member's rows, not just their own. */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * The ledger itself: one row per movement of the organization's quota.
 *
 * A request appears twice when it was pre-consumed and then settled, which is
 * what the type column is for — the two rows are a reservation and the
 * correction that replaces it, not a double charge.
 */
export function OrganizationBillingDetails(props: OrganizationBillingDetailsProps) {
  const { t } = useTranslation()
  const { search, navigate } = useOrganizationSectionRoute()
  const surface = useOrganizationSurface()
  const [isExporting, setIsExporting] = useState(false)

  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search,
    navigate,
    pagination: {
      defaultPage: 1,
      defaultPageSize: ORGANIZATION_DEFAULT_PAGE_SIZE,
    },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'month', searchKey: 'billingMonth', type: 'string' },
      { columnId: 'token_name', searchKey: 'billingToken', type: 'string' },
      { columnId: 'model_name', searchKey: 'billingModel', type: 'string' },
      { columnId: 'group', searchKey: 'billingGroup', type: 'string' },
      { columnId: 'request_id', searchKey: 'billingRequest', type: 'string' },
      {
        columnId: 'responsible_user_id',
        searchKey: 'billingResponsible',
        type: 'string',
      },
    ],
  })

  const monthFilter = organizationColumnFilterValue(columnFilters, 'month')
  // No month chosen means the current one. The endpoint would otherwise answer
  // with every record the organization has ever written, which is not the
  // question a reader opening this panel is asking; "all" is how they ask it.
  const month = monthFilter ?? currentOrganizationBillingMonth()
  // The picker offers the last year. A month named by a link from further back
  // is added to it rather than left to render as a blank control above a table
  // that is visibly filtered by it.
  const monthOptions = recentOrganizationBillingMonths()
  if (
    monthFilter !== undefined &&
    monthFilter !== ALL_MONTHS &&
    !monthOptions.includes(monthFilter)
  ) {
    monthOptions.push(monthFilter)
    monthOptions.sort((left, right) => (left < right ? 1 : -1))
  }
  const filters = organizationBillingDetailFilters(
    {
      month,
      token_name: organizationColumnFilterValue(columnFilters, 'token_name'),
      model_name: organizationColumnFilterValue(columnFilters, 'model_name'),
      group: organizationColumnFilterValue(columnFilters, 'group'),
      request_id: organizationColumnFilterValue(columnFilters, 'request_id'),
      responsible_name: organizationColumnFilterValue(
        columnFilters,
        'responsible_user_id'
      ),
    },
    { canViewWideData: props.canViewWideData }
  )

  const listParams = {
    ...filters,
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
  }

  const records = useOrganizationPagedSection<OrganizationBillingRecord>({
    surface,
    organizationId: props.organizationId,
    resource: 'billing-records',
    params: listParams,
    query: () =>
      listOrganizationBillingDetails(surface, props.organizationId, listParams),
    onForbidden: props.onForbidden,
  })

  const columns = buildBillingColumns(t, props.canViewWideData)

  const { table } = useDataTable({
    data: records.items,
    columns,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: records.total,
    ensurePageInRange,
    columnVisibilityStorageKey: BILLING_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  // The default month is not a filter the reader applied, so it does not count
  // towards "something is filtered" — only a choice they made does.
  const hasFilters = columnFilters.length > 0

  const resetFilters = () => {
    onColumnFiltersChange([])
    onPaginationChange((previous) => ({ ...previous, pageIndex: 0 }))
  }

  const buildExportColumns = (): OrganizationCsvColumn<OrganizationBillingRecord>[] => {
    const exportColumns: OrganizationCsvColumn<OrganizationBillingRecord>[] = [
      {
        title: t('Request ID'),
        value: (row) => row.request_id || '',
      },
      {
        title: t('Time'),
        value: (row) => formatOrganizationBillingExportTime(row.created_at),
      },
      {
        title: t('Type'),
        value: (row) => organizationBillingRecordTypeMeta(row.record_type, t).label,
      },
    ]

    if (props.canViewWideData) {
      exportColumns.push({
        title: t('Member'),
        value: (row) => organizationBillingResponsibleName(row),
      })
    }

    exportColumns.push(
      { title: t('Key'), value: (row) => row.token_name || '' },
      { title: t('Model'), value: (row) => row.model_name || '' },
      { title: t('Group'), value: (row) => row.group || '' },
      { title: t('Prompt Tokens'), value: (row) => row.prompt_tokens || 0 },
      {
        title: t('Completion Tokens'),
        value: (row) => row.completion_tokens || 0,
      },
      {
        title: t('Quota change'),
        value: (row) => organizationBillingLedgerDelta(row),
      }
    )

    return exportColumns
  }

  const handleExport = async () => {
    setIsExporting(true)
    try {
      // The export covers the whole filtered set, not the visible page: the
      // endpoint clamps `page_size` to 100, so it is assembled page by page.
      const collected = await collectOrganizationRows<OrganizationBillingRecord>(
        async (page, pageSize) => {
          const response = await listOrganizationBillingDetails(
            surface,
            props.organizationId,
            { ...filters, p: page, page_size: pageSize }
          )
          return (
            response.data ?? { page, page_size: pageSize, total: 0, items: [] }
          )
        }
      )

      downloadOrganizationBillingCsv(
        buildOrganizationBillingCsv(buildExportColumns(), collected.items),
        organizationBillingCsvFilename('details')
      )

      // Being cut short is reported rather than left to be noticed in the file.
      if (collected.truncated) {
        toast.warning(
          t('Exported the first {{exported}} of {{total}} records.', {
            exported: collected.items.length,
            total: collected.total,
          })
        )
      }
    } catch {
      // The interceptor has already reported the reason.
      toast.error(t('The export could not be prepared.'))
    } finally {
      setIsExporting(false)
    }
  }

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={records.isLoading}
      isFetching={records.isFetching}
      emptyTitle={t('No billing records in this range')}
      emptyDescription={t(
        'Adjust the month or the filters to find what the organization was charged.'
      )}
      skeletonKeyPrefix='organization-billing-skeleton'
      applyHeaderSize
      toolbar={
        <LogsFilterToolbar
          table={table}
          hasActiveFilters={hasFilters}
          searchLoading={records.isFetching}
          onReset={resetFilters}
          onSearch={records.refetch}
          actionStart={
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={() => void handleExport()}
              disabled={isExporting || records.total === 0}
            >
              <Download className='size-4' />
              {isExporting ? t('Exporting...') : t('Export')}
            </Button>
          }
          primaryFilters={
            <>
              <LogsFilterField>
                <Select
                  value={monthFilter === undefined ? month : monthFilter}
                  onValueChange={(value) =>
                    setOrganizationTextFilter(
                      onColumnFiltersChange,
                      columnFilters,
                      'month',
                      String(value)
                    )
                  }
                >
                  <SelectTrigger className='h-8 w-full' size='sm'>
                    <SelectValue placeholder={t('Month')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value={ALL_MONTHS}>{t('All months')}</SelectItem>
                    {monthOptions.map((option) => (
                      <SelectItem key={option} value={option}>
                        {option}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </LogsFilterField>
              <OrganizationTextFilter
                value={filters.model_name}
                placeholder={t('Model')}
                onChange={(value) =>
                  setOrganizationTextFilter(
                    onColumnFiltersChange,
                    columnFilters,
                    'model_name',
                    value
                  )
                }
              />
              <OrganizationTextFilter
                value={filters.token_name}
                placeholder={t('Key name')}
                onChange={(value) =>
                  setOrganizationTextFilter(
                    onColumnFiltersChange,
                    columnFilters,
                    'token_name',
                    value
                  )
                }
              />
              <OrganizationTextFilter
                value={filters.group}
                placeholder={t('Group')}
                onChange={(value) =>
                  setOrganizationTextFilter(
                    onColumnFiltersChange,
                    columnFilters,
                    'group',
                    value
                  )
                }
              />
              <OrganizationTextFilter
                value={filters.request_id}
                placeholder={t('Request ID')}
                onChange={(value) =>
                  setOrganizationTextFilter(
                    onColumnFiltersChange,
                    columnFilters,
                    'request_id',
                    value
                  )
                }
              />
              {props.canViewWideData ? (
                <OrganizationTextFilter
                  value={filters.responsible_name}
                  placeholder={t('Member')}
                  onChange={(value) =>
                    setOrganizationTextFilter(
                      onColumnFiltersChange,
                      columnFilters,
                      'responsible_user_id',
                      value
                    )
                  }
                />
              ) : null}
            </>
          }
        />
      }
    />
  )
}

/** The balance movement for one record, signed so the direction is explicit. */
function formatLedgerDelta(value: number): string {
  if (value === 0) return formatQuota(0)
  return `${value > 0 ? '+' : '-'}${formatQuota(Math.abs(value))}`
}

function buildBillingColumns(
  t: (key: string) => string,
  canViewWideData: boolean
): ColumnDef<OrganizationBillingRecord>[] {
  const columns: ColumnDef<OrganizationBillingRecord>[] = [
    {
      id: 'request_id',
      accessorKey: 'request_id',
      header: () => <span className='whitespace-nowrap'>{t('Request ID')}</span>,
      size: 220,
      cell: ({ row }) => (
        <CopyableRequestId value={row.original.request_id} label={t('Request ID')} />
      ),
    },
    {
      id: 'created_at',
      accessorKey: 'created_at',
      header: () => <span className='whitespace-nowrap'>{t('Time')}</span>,
      size: 180,
      cell: ({ row }) => (
        <span className='whitespace-nowrap'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
    },
    {
      id: 'record_type',
      accessorKey: 'record_type',
      header: () => <span className='whitespace-nowrap'>{t('Type')}</span>,
      size: 130,
      cell: ({ row }) => {
        const meta = organizationBillingRecordTypeMeta(
          row.original.record_type,
          t
        )
        return (
          <StatusBadge variant={meta.variant} copyable={false}>
            {meta.label}
          </StatusBadge>
        )
      },
    },
  ]

  if (canViewWideData) {
    columns.push({
      id: 'responsible_user_id',
      accessorKey: 'responsible_user_id',
      header: () => <span className='whitespace-nowrap'>{t('Member')}</span>,
      size: 160,
      cell: ({ row }) => (
        // The endpoint resolves the name; without wide-data access it never
        // returns another member's row for this column to describe.
        <span className='whitespace-nowrap font-medium'>
          {organizationBillingResponsibleName(row.original)}
        </span>
      ),
    })
  }

  columns.push(
    {
      id: 'token_name',
      accessorKey: 'token_name',
      header: () => <span className='whitespace-nowrap'>{t('Key')}</span>,
      size: 160,
      cell: ({ row }) => (
        <span className='whitespace-nowrap'>{row.original.token_name || '-'}</span>
      ),
    },
    {
      id: 'model_name',
      accessorKey: 'model_name',
      header: () => <span className='whitespace-nowrap'>{t('Model')}</span>,
      size: 180,
      cell: ({ row }) => (
        <span className='block max-w-44 truncate font-mono text-xs'>
          {row.original.model_name || '-'}
        </span>
      ),
    },
    {
      id: 'group',
      accessorKey: 'group',
      header: () => <span className='whitespace-nowrap'>{t('Group')}</span>,
      size: 140,
      cell: ({ row }) => (
        <span className='whitespace-nowrap'>{row.original.group || '-'}</span>
      ),
    },
    {
      id: 'prompt_tokens',
      accessorKey: 'prompt_tokens',
      header: () => <span className='whitespace-nowrap'>{t('Prompt Tokens')}</span>,
      size: 120,
      cell: ({ row }) => (
        <span className='tabular-nums'>{row.original.prompt_tokens || 0}</span>
      ),
    },
    {
      id: 'completion_tokens',
      accessorKey: 'completion_tokens',
      header: () => (
        <span className='whitespace-nowrap'>{t('Completion Tokens')}</span>
      ),
      size: 130,
      cell: ({ row }) => (
        <span className='tabular-nums'>{row.original.completion_tokens || 0}</span>
      ),
    },
    {
      id: 'ledger_quota_delta',
      accessorFn: (row) => organizationBillingLedgerDelta(row),
      header: () => <span className='whitespace-nowrap'>{t('Quota change')}</span>,
      size: 140,
      cell: ({ row }) => {
        const delta = organizationBillingLedgerDelta(row.original)
        return (
          <span
            className={
              delta > 0
                ? 'text-success tabular-nums'
                : 'text-foreground tabular-nums'
            }
          >
            {formatLedgerDelta(delta)}
          </span>
        )
      },
    }
  )

  return columns
}

/**
 * A request id worth copying.
 *
 * The id is what ties a ledger row to a log entry and to a support thread, so
 * reading it is almost always the prelude to pasting it somewhere.
 */
function CopyableRequestId(props: { value?: string; label: string }) {
  const { t } = useTranslation()
  if (!props.value) return <span className='text-muted-foreground'>-</span>

  return (
    <button
      type='button'
      className='hover:text-foreground max-w-full truncate text-left font-mono text-xs underline decoration-dotted underline-offset-2'
      title={props.value}
      onClick={() => {
        void copyToClipboard(props.value ?? '').then((ok) => {
          if (ok) toast.success(t('Copied'))
        })
      }}
    >
      {props.value}
    </button>
  )
}
