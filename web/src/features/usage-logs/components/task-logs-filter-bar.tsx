/*
Copyright (C) 2023-2026 QuantumNous

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
import { useQuery, useQueryClient, useIsFetching } from '@tanstack/react-query'
import { useNavigate, getRouteApi } from '@tanstack/react-router'
import type { Table } from '@tanstack/react-table'
import { useState, useEffect, useCallback, useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { getOrganizations } from '@/features/organization/api'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

import { TASK_ACTION_MAPPINGS, TASK_STATUS_MAPPINGS } from '../constants'
import { buildSearchParams } from '../lib/filter'
import { getDefaultTimeRange } from '../lib/utils'
import type { DrawingLogFilters, LogCategory, TaskLogFilters } from '../types'
import { CompactDateTimeRangePicker } from './compact-date-time-range-picker'
import {
  LogsFilterField,
  LogsFilterInput,
  LogsFilterToolbar,
} from './logs-filter-toolbar'
import { useLogsViewScope } from './usage-logs-provider'

const route = getRouteApi('/_authenticated/usage-logs/$section')

type TaskLikeLogCategory = Extract<LogCategory, 'drawing' | 'task'>
type TaskLogsFilters = DrawingLogFilters | TaskLogFilters

interface TaskLogsFilterBarProps<TData> {
  table: Table<TData>
  logCategory: TaskLikeLogCategory
}

function getFilterValue(
  filters: TaskLogsFilters,
  logCategory: TaskLikeLogCategory
): string {
  if (logCategory === 'drawing') {
    return (filters as DrawingLogFilters).mjId || ''
  }
  return (filters as TaskLogFilters).taskId || ''
}

function setFilterValue(
  filters: TaskLogsFilters,
  logCategory: TaskLikeLogCategory,
  value: string
): TaskLogsFilters {
  if (logCategory === 'drawing') {
    return { ...filters, mjId: value }
  }
  return { ...filters, taskId: value }
}

export function TaskLogsFilterBar<TData>(props: TaskLogsFilterBarProps<TData>) {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const searchParams = route.useSearch()
  const { isAdminView: isAdmin } = useLogsViewScope(props.logCategory)
  const fetchingLogs = useIsFetching({ queryKey: ['logs'] })

  const organizationsQuery = useQuery({
    queryKey: [
      'organizations',
      'self',
      'task-log-filter',
      getAccountContextCacheKey(),
    ],
    queryFn: getOrganizations,
    enabled: props.logCategory === 'task' && !isAdmin,
    staleTime: 5 * 60 * 1000,
  })
  const organizations = useMemo(
    () => organizationsQuery.data?.data ?? [],
    [organizationsQuery.data]
  )

  const [filters, setFilters] = useState<TaskLogsFilters>(() => {
    const { start, end } = getDefaultTimeRange()
    return { startTime: start, endTime: end }
  })

  useEffect(() => {
    const { start, end } = getDefaultTimeRange()
    const baseFilters = {
      startTime: searchParams.startTime
        ? new Date(searchParams.startTime)
        : start,
      endTime: searchParams.endTime ? new Date(searchParams.endTime) : end,
      ...(searchParams.channel
        ? { channel: String(searchParams.channel) }
        : {}),
    }
    const next: TaskLogsFilters =
      props.logCategory === 'drawing'
        ? {
            ...baseFilters,
            ...(searchParams.filter ? { mjId: searchParams.filter } : {}),
          }
        : {
            ...baseFilters,
            ...(searchParams.filter ? { taskId: searchParams.filter } : {}),
            ...(searchParams.status
              ? { status: String(searchParams.status) }
              : {}),
            ...(searchParams.action
              ? { action: String(searchParams.action) }
              : {}),
            ...(searchParams.billingAccountType
              ? {
                  billingAccountType: String(searchParams.billingAccountType),
                }
              : {}),
            ...(searchParams.organizationId
              ? { organizationId: String(searchParams.organizationId) }
              : {}),
          }

    setFilters(next)
  }, [
    props.logCategory,
    searchParams.startTime,
    searchParams.endTime,
    searchParams.channel,
    searchParams.filter,
    searchParams.status,
    searchParams.action,
    searchParams.billingAccountType,
    searchParams.organizationId,
  ])

  const handleChange = useCallback(
    (field: keyof TaskLogFilters, value: Date | string | undefined) => {
      setFilters(
        (prev) =>
          ({ ...(prev as TaskLogFilters), [field]: value }) as TaskLogsFilters
      )
    },
    []
  )

  const handleApply = useCallback(() => {
    const filtersForParams =
      props.logCategory === 'task' &&
      !isAdmin &&
      (filters as TaskLogFilters).billingAccountType !== 'organization'
        ? { ...(filters as TaskLogFilters), organizationId: undefined }
        : filters
    const filterParams = buildSearchParams(filtersForParams, props.logCategory)
    navigate({
      to: '/usage-logs/$section',
      params: { section: props.logCategory },
      search: {
        ...filterParams,
        page: 1,
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [filters, isAdmin, navigate, props.logCategory, queryClient])

  const handleReset = useCallback(() => {
    const { start, end } = getDefaultTimeRange()
    setFilters({ startTime: start, endTime: end })

    navigate({
      to: '/usage-logs/$section',
      params: { section: props.logCategory },
      search: {
        page: 1,
        startTime: start.getTime(),
        endTime: end.getTime(),
      },
    })
    queryClient.invalidateQueries({ queryKey: ['logs'] })
  }, [navigate, props.logCategory, queryClient])

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') handleApply()
    },
    [handleApply]
  )

  const filterValue = getFilterValue(filters, props.logCategory)
  const taskFilters =
    props.logCategory === 'task' ? (filters as TaskLogFilters) : null
  const drawingFilters =
    props.logCategory === 'drawing' ? (filters as DrawingLogFilters) : null
  const placeholder =
    props.logCategory === 'drawing'
      ? t('Filter by MjProxy task ID')
      : t('Filter by task ID')
  const statusItems = useMemo(
    () => [
      { value: 'all', label: t('All statuses') },
      ...Object.entries(TASK_STATUS_MAPPINGS).map(([value, meta]) => ({
        value,
        label: t(meta.label),
      })),
    ],
    [t]
  )
  const actionItems = useMemo(
    () => [
      { value: 'all', label: t('All Types') },
      ...Object.entries(TASK_ACTION_MAPPINGS).map(([value, meta]) => ({
        value,
        label: t(meta.label),
      })),
    ],
    [t]
  )
  const billingAccountTypeItems = useMemo(
    () => [
      { value: 'all', label: t('All Billing Types') },
      { value: 'personal', label: t('Personal') },
      { value: 'organization', label: t('Organization') },
    ],
    [t]
  )
  const organizationItems = useMemo(
    () => [
      { value: 'all', label: t('All organizations') },
      ...organizations.map((organization) => ({
        value: String(organization.id),
        label: organization.name,
      })),
    ],
    [organizations, t]
  )
  const statusLabel =
    statusItems.find((item) => item.value === taskFilters?.status)?.label ??
    t('All statuses')
  const actionLabel =
    actionItems.find((item) => item.value === taskFilters?.action)?.label ??
    t('All Types')
  const billingAccountTypeValue = taskFilters?.billingAccountType || 'all'
  const billingAccountTypeLabel =
    billingAccountTypeItems.find(
      (item) => item.value === billingAccountTypeValue
    )?.label ?? t('All Billing Types')
  const organizationValue = taskFilters?.organizationId || 'all'
  const organizationLabel =
    organizationItems.find((item) => item.value === organizationValue)?.label ??
    t('All organizations')

  const hasAdvancedFilters =
    !!filterValue ||
    (isAdmin && !!taskFilters?.channel) ||
    (isAdmin && !!taskFilters?.organizationId)
  const advancedFilterCount = [
    filterValue,
    isAdmin ? taskFilters?.channel : undefined,
    isAdmin ? taskFilters?.organizationId : undefined,
  ].filter(Boolean).length
  const hasActiveFilters =
    hasAdvancedFilters ||
    !!taskFilters?.status ||
    !!taskFilters?.action ||
    !!taskFilters?.billingAccountType ||
    (!isAdmin && !!taskFilters?.organizationId)

  const dateRangeFilter = (
    <LogsFilterField className='sm:col-span-2'>
      <CompactDateTimeRangePicker
        start={filters.startTime}
        end={filters.endTime}
        onChange={({ start, end }) => {
          handleChange('startTime', start)
          handleChange('endTime', end)
        }}
      />
    </LogsFilterField>
  )
  const taskIdFilter = (
    <LogsFilterField>
      <LogsFilterInput
        aria-label={t('Task ID')}
        placeholder={placeholder}
        value={filterValue}
        onChange={(e) => {
          setFilters((prev) =>
            setFilterValue(prev, props.logCategory, e.target.value)
          )
        }}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  )
  const channelFilter = isAdmin ? (
    <LogsFilterField>
      <LogsFilterInput
        placeholder={t('Channel ID')}
        value={taskFilters?.channel ?? drawingFilters?.channel ?? ''}
        onChange={(e) => handleChange('channel', e.target.value)}
        onKeyDown={handleKeyDown}
      />
    </LogsFilterField>
  ) : null
  const organizationIdFilter =
    isAdmin && taskFilters ? (
      <LogsFilterField>
        <LogsFilterInput
          inputMode='numeric'
          placeholder={t('Organization ID')}
          value={taskFilters.organizationId || ''}
          onChange={(e) => handleChange('organizationId', e.target.value)}
          onKeyDown={handleKeyDown}
        />
      </LogsFilterField>
    ) : null
  const statusFilter = (
    <LogsFilterField>
      <Select
        items={statusItems}
        value={taskFilters?.status || 'all'}
        onValueChange={(value) =>
          handleChange(
            'status',
            value === null || value === 'all' ? undefined : value
          )
        }
      >
        <SelectTrigger>
          <SelectValue>{statusLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {statusItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const actionFilter = (
    <LogsFilterField>
      <Select
        items={actionItems}
        value={taskFilters?.action || 'all'}
        onValueChange={(value) =>
          handleChange(
            'action',
            value === null || value === 'all' ? undefined : value
          )
        }
      >
        <SelectTrigger>
          <SelectValue>{actionLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {actionItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const billingAccountTypeFilter = (
    <LogsFilterField>
      <Select
        items={billingAccountTypeItems}
        value={billingAccountTypeValue}
        onValueChange={(value) => {
          const next = value === null || value === 'all' ? undefined : value
          handleChange('billingAccountType', next)
          // The organization filter only means anything under organization
          // billing; clearing the type clears the organization with it.
          if (next !== 'organization') {
            handleChange('organizationId', undefined)
          }
        }}
      >
        <SelectTrigger>
          <SelectValue>{billingAccountTypeLabel}</SelectValue>
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {billingAccountTypeItems.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    </LogsFilterField>
  )
  const organizationFilter =
    !isAdmin &&
    organizations.length > 0 &&
    billingAccountTypeValue === 'organization' ? (
      <LogsFilterField>
        <Select
          items={organizationItems}
          value={organizationValue}
          onValueChange={(value) =>
            handleChange(
              'organizationId',
              value === null || value === 'all' ? undefined : value
            )
          }
        >
          <SelectTrigger>
            <SelectValue>{organizationLabel}</SelectValue>
          </SelectTrigger>
          <SelectContent alignItemWithTrigger={false}>
            <SelectGroup>
              {organizationItems.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectGroup>
          </SelectContent>
        </Select>
      </LogsFilterField>
    ) : null

  if (props.logCategory === 'drawing') {
    return (
      <LogsFilterToolbar
        table={props.table}
        primaryFilters={
          <>
            {dateRangeFilter}
            {taskIdFilter}
            {channelFilter}
          </>
        }
        mobilePinnedFilters={dateRangeFilter}
        mobileFilters={
          <>
            {taskIdFilter}
            {channelFilter}
          </>
        }
        mobileFilterCount={
          [filterValue, drawingFilters?.channel].filter(Boolean).length
        }
        hasActiveFilters={!!filterValue || !!drawingFilters?.channel}
        onSearch={handleApply}
        searchLoading={fetchingLogs > 0}
        onReset={handleReset}
      />
    )
  }

  return (
    <LogsFilterToolbar
      table={props.table}
      primaryFilters={
        <>
          {dateRangeFilter}
          {statusFilter}
          {actionFilter}
          {billingAccountTypeFilter}
          {organizationFilter}
        </>
      }
      advancedFilters={
        <>
          {taskIdFilter}
          {channelFilter}
          {organizationIdFilter}
        </>
      }
      mobilePinnedFilters={dateRangeFilter}
      mobileFilters={
        <>
          {statusFilter}
          {actionFilter}
          {billingAccountTypeFilter}
          {organizationFilter}
          {taskIdFilter}
          {channelFilter}
          {organizationIdFilter}
        </>
      }
      mobileFilterCount={
        advancedFilterCount +
        [
          taskFilters?.status,
          taskFilters?.action,
          taskFilters?.billingAccountType,
          !isAdmin ? taskFilters?.organizationId : undefined,
        ].filter(Boolean).length
      }
      hasAdvancedActiveFilters={hasAdvancedFilters}
      advancedFilterCount={advancedFilterCount}
      hasActiveFilters={hasActiveFilters}
      onSearch={handleApply}
      searchLoading={fetchingLogs > 0}
      onReset={handleReset}
    />
  )
}
