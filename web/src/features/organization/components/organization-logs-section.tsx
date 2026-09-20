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
import type { ColumnDef } from '@tanstack/react-table'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { LogCostDisplay } from '@/features/usage-logs/components/log-cost-display'
import {
  LogsFilterField,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'
import { ModelBadge } from '@/features/usage-logs/components/model-badge'
import { TimingMetricsCell } from '@/features/usage-logs/components/timing-metrics-cell'
import { parseLogOther } from '@/features/usage-logs/lib/format'
import {
  getDefaultTimeRange,
  isDisplayableLogType,
  isTimingLogType,
} from '@/features/usage-logs/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatLogQuota, formatTimestamp } from '@/lib/format'

import { getOrganizationLogStats, listOrganizationLogs } from '../api'
import {
  useOrganizationPagedSection,
  useOrganizationResourceSection,
} from '../hooks/use-organization-paged-query'
import {
  organizationColumnFilterValue,
  organizationLogModelInfo,
  organizationLogResponsibleName,
  organizationLogTimeRangeParams,
  organizationLogTimeWindow,
  organizationLogTokenName,
  organizationLogTypeMeta,
  ORGANIZATION_LOG_TYPE,
  ORGANIZATION_LOG_TYPE_FILTER_OPTIONS,
  setOrganizationTextFilter,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationLogRow, OrganizationLogStats } from '../types'
import { OrganizationTextFilter } from './organization-filter-fields'
import { OrganizationLogDetailsDialog } from './organization-log-details-dialog'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
} from './organization-section'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

const LOG_COLUMN_VISIBILITY_STORAGE_KEY = 'organization-log-column-visibility'

type OrganizationLogsSectionProps = {
  organizationId: number
  /** The caller may read the organization's log records. */
  canView: boolean
  /**
   * The caller reads every member's records, not just their own.
   *
   * Without it the backend pins every query to the caller, so a holder filter
   * would be inert and a holder column would repeat one name down the table.
   */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * What the organization spent, entry by entry.
 *
 * The window is part of the filter set rather than a fixed look-back: an
 * operator asking "what happened last night" and one asking "what has this
 * month cost" are the same question over different ranges.
 */
export function OrganizationLogsSection(props: OrganizationLogsSectionProps) {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()

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
      { columnId: 'type', searchKey: 'logType', type: 'string' },
      { columnId: 'model_name', searchKey: 'logModel', type: 'string' },
      { columnId: 'token_name', searchKey: 'logToken', type: 'string' },
      { columnId: 'group', searchKey: 'logGroup', type: 'string' },
      { columnId: 'request_id', searchKey: 'logRequest', type: 'string' },
      {
        columnId: 'responsible_user_id',
        searchKey: 'logResponsible',
        type: 'string',
      },
    ],
  })

  const window = organizationLogTimeWindow(search, getDefaultTimeRange())
  const timeRange = organizationLogTimeRangeParams(window.start, window.end)

  const filters = {
    type: parseLogType(organizationColumnFilterValue(columnFilters, 'type')),
    model_name: organizationColumnFilterValue(columnFilters, 'model_name'),
    token_name: organizationColumnFilterValue(columnFilters, 'token_name'),
    group: organizationColumnFilterValue(columnFilters, 'group'),
    request_id: organizationColumnFilterValue(columnFilters, 'request_id'),
    responsible_name: props.canViewWideData
      ? organizationColumnFilterValue(columnFilters, 'responsible_user_id')
      : undefined,
  }

  const listParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    ...filters,
    ...timeRange,
  }

  const logs = useOrganizationPagedSection<OrganizationLogRow>({
    organizationId: props.organizationId,
    resource: 'logs',
    params: listParams,
    query: () => listOrganizationLogs(props.organizationId, listParams),
    enabled: props.canView,
    onForbidden: props.onForbidden,
  })

  const statsParams = { ...filters, ...timeRange }
  const stats = useOrganizationResourceSection<OrganizationLogStats>({
    organizationId: props.organizationId,
    resource: 'log-stats',
    params: statsParams,
    query: () => getOrganizationLogStats(props.organizationId, statsParams),
    enabled: props.canView,
    onForbidden: props.onForbidden,
  })

  const columns = buildLogColumns(t, props.canViewWideData)

  const { table } = useDataTable({
    data: logs.items,
    columns,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: logs.total,
    ensurePageInRange,
    columnVisibilityStorageKey: LOG_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  if (!props.canView) {
    return (
      <OrganizationSectionEmpty
        icon='logs'
        title={t('Logs')}
        message={t(
          'Only an organization owner or administrator can read its records.'
        )}
      />
    )
  }

  const isFiltered = Object.values(filters).some(
    (value) => value !== undefined && value !== ''
  )

  const resetFilters = () => {
    onColumnFiltersChange([])
    navigate({
      search: (previous) => ({
        ...previous,
        startTime: undefined,
        endTime: undefined,
      }),
    })
    onPaginationChange((previous) => ({ ...previous, pageIndex: 0 }))
  }

  return (
    <OrganizationSection
      icon='logs'
      title={t('Logs')}
      description={t(
        'Every request the organization was charged for, and who was responsible for it.'
      )}
      count={logs.total}
    >
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={logs.isLoading}
        isFetching={logs.isFetching}
        emptyTitle={t('No log records yet')}
        skeletonKeyPrefix='organization-log-skeleton'
        applyHeaderSize
        // The generic toolbar cannot hold the filter grid, the time picker and
        // the stat pills in one row, so this section brings its own.
        toolbar={
          <LogsFilterToolbar
            table={table}
            primaryFilters={
              <>
                <LogsFilterField className='sm:col-span-2'>
                  <CompactDateTimeRangePicker
                    start={window.start}
                    end={window.end}
                    onChange={({ start, end }) =>
                      navigate({
                        search: (previous) => ({
                          ...previous,
                          startTime: start?.getTime(),
                          endTime: end?.getTime(),
                        }),
                      })
                    }
                  />
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
                    placeholder={t('Responsible user')}
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
            advancedFilters={
              <LogsFilterField wide>
                <LogTypeFilter
                  value={filters.type}
                  onChange={(value) =>
                    setOrganizationTextFilter(
                      onColumnFiltersChange,
                      columnFilters,
                      'type',
                      value
                    )
                  }
                />
              </LogsFilterField>
            }
            stats={<OrganizationLogStatsPills stats={stats.data} />}
            hasActiveFilters={isFiltered || window.fromUrl}
            hasAdvancedActiveFilters={filters.type !== undefined}
            searchLoading={logs.isFetching}
            onReset={resetFilters}
            onSearch={logs.refetch}
          />
        }
      />
    </OrganizationSection>
  )
}

/** One of the text filters above the table. */
/**
 * The type, as the endpoint wants it.
 *
 * Zero is the backend's "unset" (`model.LogTypeUnknown`), so it is treated as
 * no choice at all rather than sent as one — a hand-edited link reading
 * `logType=0` means the same thing as no link, and should look the same too.
 */
function parseLogType(value?: string): number | undefined {
  if (value === undefined) return undefined
  const parsed = Number.parseInt(value, 10)
  return Number.isInteger(parsed) && parsed > 0 ? parsed : undefined
}

/**
 * The type filter.
 *
 * A plain select rather than a faceted one: the set is fixed, the backend takes
 * one value, and "every type" is the absence of a choice rather than a choice
 * of its own. The sentinel below never reaches the URL — it clears the filter.
 */
function LogTypeFilter(props: {
  value?: number
  onChange: (value: string) => void
}) {
  const { t } = useTranslation()
  const ANY_TYPE = ''

  return (
    <Select
      value={props.value === undefined ? ANY_TYPE : String(props.value)}
      onValueChange={(next) => props.onChange(next ?? ANY_TYPE)}
    >
      <SelectTrigger className='h-8 w-full' size='sm'>
        <SelectValue placeholder={t('Type')} />
      </SelectTrigger>
      <SelectContent>
        <SelectItem value={ANY_TYPE}>{t('All types')}</SelectItem>
        {ORGANIZATION_LOG_TYPE_FILTER_OPTIONS.map((option) => (
          <SelectItem key={option.value} value={option.value}>
            {t(option.labelKey)}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  )
}

/**
 * The figures beside the filter bar.
 *
 * The cost follows the chosen window. RPM and TPM do not: the endpoint reports
 * them over the last sixty seconds regardless of the range it is given, so
 * showing them as if they were averages over the window would misread them.
 * Each says which it is.
 */
function OrganizationLogStatsPills(props: {
  stats: OrganizationLogStats | undefined
}) {
  const { t } = useTranslation()

  return (
    <div className='flex flex-wrap items-center gap-1.5'>
      <StatPill
        label={t('Cost')}
        value={formatStat(props.stats?.quota, formatLogQuota)}
        accent='bg-emerald-500'
        hint={t('The cost of the entries in the selected time range.')}
      />
      <StatPill
        label='RPM'
        value={formatStat(props.stats?.rpm, (value) => value.toLocaleString())}
        accent='bg-sky-500'
        hint={t('Requests in the last 60 seconds, whatever the range above.')}
      />
      <StatPill
        label='TPM'
        value={formatStat(props.stats?.tpm, (value) => value.toLocaleString())}
        accent='bg-violet-500'
        hint={t('Tokens in the last 60 seconds, whatever the range above.')}
      />
    </div>
  )
}

/** A figure that has not arrived yet, or has not loaded, reads as absent. */
function formatStat(
  value: number | undefined,
  format: (value: number) => string
): string {
  if (value === undefined) return '—'
  return format(value)
}

function StatPill(props: {
  label: string
  value: string
  accent: string
  hint: string
}) {
  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='border-border/60 bg-muted/25 inline-flex h-7 cursor-help items-center gap-2 rounded-md border px-2.5 text-xs shadow-xs' />
          }
        >
          <span className={`h-3.5 w-0.5 rounded-full ${props.accent}`} />
          <span className='text-muted-foreground'>{props.label}</span>
          <span className='font-mono tabular-nums'>{props.value}</span>
        </TooltipTrigger>
        <TooltipContent>{props.hint}</TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

function buildLogColumns(
  t: (key: string) => string,
  showResponsible: boolean
): ColumnDef<OrganizationLogRow>[] {
  const columns: ColumnDef<OrganizationLogRow>[] = [
    {
      accessorKey: 'created_at',
      header: t('Time'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap tabular-nums'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
    },
    {
      accessorKey: 'type',
      header: t('Type'),
      enableSorting: false,
      size: 100,
      cell: ({ row }) => {
        const meta = organizationLogTypeMeta(row.original.type)
        return (
          <StatusBadge
            label={t(meta.labelKey)}
            variant={meta.variant}
            copyable={false}
          />
        )
      },
    },
  ]

  if (showResponsible) {
    columns.push({
      accessorKey: 'responsible_user_id',
      header: t('Responsible user'),
      enableSorting: false,
      size: 150,
      cell: ({ row }) => (
        <span className='text-sm whitespace-nowrap'>
          {organizationLogResponsibleName(row.original)}
        </span>
      ),
    })
  }

  columns.push(
    {
      accessorKey: 'token_name',
      header: t('Key'),
      enableSorting: false,
      size: 160,
      cell: ({ row }) => (
        <span className='font-mono text-xs'>
          {organizationLogTokenName(row.original)}
        </span>
      ),
    },
    {
      accessorKey: 'group',
      header: t('Group'),
      enableSorting: false,
      size: 110,
      cell: ({ row }) =>
        row.original.group ? (
          <span className='font-mono text-xs'>{row.original.group}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        ),
    },
    {
      accessorKey: 'model_name',
      header: t('Model'),
      enableSorting: false,
      size: 180,
      cell: ({ row }) => {
        if (!isDisplayableLogType(row.original.type)) return null
        const model = organizationLogModelInfo(row.original)
        return (
          <ModelBadge modelName={model.name} actualModel={model.actualModel} />
        )
      },
    },
    {
      accessorKey: 'prompt_tokens',
      header: t('Tokens'),
      enableSorting: false,
      size: 140,
      cell: ({ row }) => {
        if (!isDisplayableLogType(row.original.type)) return null
        const prompt = row.original.prompt_tokens || 0
        const completion = row.original.completion_tokens || 0
        if (prompt === 0 && completion === 0) {
          return <span className='text-muted-foreground text-xs'>-</span>
        }
        return (
          <span className='font-mono text-xs tabular-nums'>
            {prompt.toLocaleString()} / {completion.toLocaleString()}
          </span>
        )
      },
    },
    {
      accessorKey: 'quota',
      header: t('Cost'),
      enableSorting: false,
      size: 120,
      cell: ({ row }) => {
        if (!isDisplayableLogType(row.original.type)) return null
        return (
          <LogCostDisplay
            quota={row.original.quota}
            other={parseLogOther(row.original.other ?? '')}
          />
        )
      },
    },
    {
      accessorKey: 'use_time',
      header: t('Timing'),
      enableSorting: false,
      size: 200,
      cell: ({ row }) => {
        if (!isTimingLogType(row.original.type)) return null
        const other = parseLogOther(row.original.other ?? '')
        return (
          <TimingMetricsCell
            useTimeSec={row.original.use_time}
            completionTokens={row.original.completion_tokens}
            frtMs={other?.frt}
            isStream={row.original.is_stream}
          />
        )
      },
    },
    {
      accessorKey: 'request_id',
      header: t('Request ID'),
      enableSorting: false,
      size: 150,
      cell: ({ row }) =>
        row.original.request_id ? (
          <span className='font-mono text-xs'>{row.original.request_id}</span>
        ) : (
          <span className='text-muted-foreground'>-</span>
        ),
    },
    {
      accessorKey: 'ip',
      header: t('IP'),
      enableSorting: false,
      size: 130,
      cell: ({ row }) => {
        const { type, ip } = row.original
        // Only the request types carry an address worth reading; every other
        // row's column would be a dash, since the backend records one for the
        // caller that made the charge, not for the event.
        const showable =
          (type === ORGANIZATION_LOG_TYPE.CONSUME ||
            type === ORGANIZATION_LOG_TYPE.ERROR) &&
          Boolean(ip)
        if (!showable) return <span className='text-muted-foreground'>-</span>
        return <span className='font-mono text-xs whitespace-nowrap'>{ip}</span>
      },
    },
    {
      id: 'details',
      header: t('Details'),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <LogDetailsCell
          row={row.original}
          showResponsible={showResponsible}
        />
      ),
    }
  )

  return columns
}

function LogDetailsCell(props: {
  row: OrganizationLogRow
  showResponsible: boolean
}) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)

  return (
    <>
      <button
        type='button'
        className='text-primary text-xs underline decoration-dotted underline-offset-2'
        onClick={() => setOpen(true)}
      >
        {t('View')}
      </button>
      {open ? (
        <OrganizationLogDetailsDialog
          row={props.row}
          showResponsible={props.showResponsible}
          onOpenChange={setOpen}
        />
      ) : null}
    </>
  )
}
