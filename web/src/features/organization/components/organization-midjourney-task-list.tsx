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

import { DataTablePage, useDataTable } from '@/components/data-table'
import { useDrawingLogsColumns } from '@/features/usage-logs/components/columns/drawing-logs-columns'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import {
  LogsFilterField,
  LogsFilterToolbar,
} from '@/features/usage-logs/components/logs-filter-toolbar'
import { getDefaultTimeRange } from '@/features/usage-logs/lib/utils'
import { useTableUrlState } from '@/hooks/use-table-url-state'

import { listOrganizationMidjourneyTasks } from '../api'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import {
  insertColumnsAfter,
  organizationColumnFilterValue,
  organizationLogTimeWindow,
  organizationMidjourneyTaskToLog,
  organizationTaskTimeRangeParams,
  setOrganizationTextFilter,
  type OrganizationMidjourneyTaskLog,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationMidjourneyTaskRow } from '../types'
import { OrganizationTextFilter } from './organization-filter-fields'
import { buildOrganizationTaskColumns } from './organization-task-columns'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

const MIDJOURNEY_COLUMN_VISIBILITY_STORAGE_KEY =
  'organization-midjourney-column-visibility'

type OrganizationMidjourneyTaskListProps = {
  organizationId: number
  /** The caller reads every member's tasks, not just their own. */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * The organization's Midjourney tasks.
 *
 * A separate table from the async tasks rather than a panel of the same one,
 * because a Midjourney task is a different record: it has a prompt, an image and
 * a submit result, and none of those exist on the async tasks. The endpoint
 * filters on `mj_id` and `channel_id` where the async one filters on `task_id`
 * and `platform`, so even the search boxes are different.
 *
 * The one thing the two share is the unit trap. This table stores `submit_time`
 * in milliseconds (`relay/mjproxy_handler.go` writes `UnixNano() / Millisecond`)
 * where the async task table stores whole seconds, and neither endpoint converts
 * the parameter — so the window below is built differently on purpose.
 */
export function OrganizationMidjourneyTaskList(
  props: OrganizationMidjourneyTaskListProps
) {
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
      { columnId: 'mj_id', searchKey: 'mjId', type: 'string' },
      { columnId: 'channel_id', searchKey: 'mjChannel', type: 'string' },
    ],
  })

  const window = organizationLogTimeWindow(search, getDefaultTimeRange())
  const timeRange = organizationTaskTimeRangeParams(
    window.start,
    window.end,
    'milliseconds'
  )

  const filters = {
    mj_id: organizationColumnFilterValue(columnFilters, 'mj_id'),
    // The endpoint compares the channel exactly, as a string, so an id typed
    // into the box is a channel id and nothing else — this is not a name search.
    channel_id: props.canViewWideData
      ? organizationColumnFilterValue(columnFilters, 'channel_id')
      : undefined,
  }

  const listParams = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    ...filters,
    ...timeRange,
  }

  const tasks = useOrganizationPagedSection<OrganizationMidjourneyTaskRow>({
    organizationId: props.organizationId,
    resource: 'midjourney-tasks',
    params: listParams,
    query: () =>
      listOrganizationMidjourneyTasks(props.organizationId, listParams),
    onForbidden: props.onForbidden,
  })

  const sharedColumns = useDrawingLogsColumns(false)
  const organizationColumns =
    buildOrganizationTaskColumns<OrganizationMidjourneyTaskLog>({
      showMember: props.canViewWideData,
      translate: t,
    })
  const columns = insertColumnsAfter(
    sharedColumns,
    'mj_id',
    organizationColumns
  )

  const rows = tasks.items.map(organizationMidjourneyTaskToLog)

  const { table } = useDataTable({
    data: rows,
    columns,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: tasks.total,
    ensurePageInRange,
    columnVisibilityStorageKey: MIDJOURNEY_COLUMN_VISIBILITY_STORAGE_KEY,
  })

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
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={tasks.isLoading}
      isFetching={tasks.isFetching}
      emptyTitle={t('No Midjourney tasks in this range')}
      emptyDescription={t(
        'Adjust the time range or the filters to find what the organization drew.'
      )}
      skeletonKeyPrefix='organization-midjourney-skeleton'
      applyHeaderSize
      toolbar={
        <LogsFilterToolbar
          table={table}
          hasActiveFilters={columnFilters.length > 0 || window.fromUrl}
          searchLoading={tasks.isFetching}
          onReset={resetFilters}
          onSearch={tasks.refetch}
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
                value={filters.mj_id}
                placeholder={t('Task ID')}
                onChange={(value) =>
                  setOrganizationTextFilter(
                    onColumnFiltersChange,
                    columnFilters,
                    'mj_id',
                    value
                  )
                }
              />
              {props.canViewWideData ? (
                // The channel is an internal routing detail rather than the
                // question a reader arrives with, so it is offered only to a
                // caller who can see rows routed through more than one — for
                // anyone else it would be a filter over their own single path.
                // The endpoint compares it exactly, so an id typed here is a
                // channel id and nothing else; this is not a name search.
                <OrganizationTextFilter
                  value={filters.channel_id}
                  placeholder={t('Channel')}
                  onChange={(value) =>
                    setOrganizationTextFilter(
                      onColumnFiltersChange,
                      columnFilters,
                      'channel_id',
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
