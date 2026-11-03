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
import type { PaginationState } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { formatLogQuota, formatNumber } from '@/lib/format'

import type {
  SelfUsageData,
  SelfUsageTokenOption,
  SelfUsageTotals,
  UsageOrganizationOption,
} from '../types'
import { buildUsageColumns } from './usage-columns'
import { UsageFilterBar, type UsageFilterValues } from './usage-filter-bar'

interface UsageTableProps {
  data: SelfUsageData
  loading: boolean
  filtering: boolean
  organizations: UsageOrganizationOption[]
  modelOptions: string[]
  tokenOptions: SelfUsageTokenOption[]
  initialFilters: UsageFilterValues
  page: number
  pageSize: number
  onFiltersChange: (values: UsageFilterValues) => void
  onPageChange: (page: number, pageSize: number) => void
}

export function UsageTable(props: UsageTableProps) {
  const { t } = useTranslation()
  const columns = buildUsageColumns(t)
  const pagination: PaginationState = {
    pageIndex: Math.max(0, props.page - 1),
    pageSize: props.pageSize,
  }
  const { table } = useDataTable({
    data: props.data.rows.items,
    columns,
    manualPagination: true,
    manualFiltering: true,
    totalCount: props.data.rows.total,
    pagination,
    onPaginationChange: (updater) => {
      const next =
        typeof updater === 'function' ? updater(pagination) : updater
      props.onPageChange(next.pageIndex + 1, next.pageSize)
    },
    columnVisibilityStorageKey: 'usage-table-column-visibility',
  })

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={props.loading}
      isFetching={props.filtering}
      emptyTitle={t('No usage in this period.')}
      emptyDescription={t('Adjust the time range or filters to find usage.')}
      toolbar={
        <UsageFilterBar
          table={table}
          initial={props.initialFilters}
          organizations={props.organizations}
          modelOptions={props.modelOptions}
          tokenOptions={props.tokenOptions}
          loading={props.filtering}
          onApply={props.onFiltersChange}
          onReset={() => props.onFiltersChange({})}
        />
      }
      afterTable={<UsageTotalsStrip totals={props.data.totals} />}
    />
  )
}

function UsageTotalsStrip(props: { totals: SelfUsageTotals }) {
  const { t } = useTranslation()
  const items = [
    { label: t('Requests'), value: formatNumber(props.totals.request_count) },
    { label: t('Success'), value: formatNumber(props.totals.success_count) },
    { label: t('Failed'), value: formatNumber(props.totals.failure_count) },
    {
      label: t('Tokens'),
      value: formatNumber(
        props.totals.prompt_tokens + props.totals.completion_tokens
      ),
    },
    { label: t('Cost'), value: formatLogQuota(props.totals.quota) },
  ]
  return (
    <div className='flex flex-wrap items-baseline gap-x-6 gap-y-2 border-t px-3 py-3'>
      <span className='text-muted-foreground text-xs font-medium tracking-wider uppercase'>
        {t('Total')}
      </span>
      {items.map((item) => (
        <span key={item.label} className='text-sm'>
          <span className='text-muted-foreground me-1.5 text-xs'>
            {item.label}
          </span>
          <span className='font-mono font-semibold tabular-nums'>
            {item.value}
          </span>
        </span>
      ))}
    </div>
  )
}
