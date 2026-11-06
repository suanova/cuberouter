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
import { lazy, Suspense, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { Button } from '@/components/ui/button'
import { getOrganizations } from '@/features/organization/api'
import {
  ACCOUNT_CONTEXT_TYPE,
  useAccountContextStore,
} from '@/stores/account-context-store'

import { exportSelfUsage, getSelfUsage } from './api'
import { UsageDateRangePicker } from './components/usage-date-range-picker'
import type { UsageFilterValues } from './components/usage-filter-bar'
import { UsageOverview } from './components/usage-overview'
import { UsageTable } from './components/usage-table'
import { resolveOverviewOrganizationId } from './lib/organization-selection'
import { usageRefetchInterval } from './lib/polling'
import { usageDateParams, usageWindowFromSearch } from './lib/time-window'
import {
  buildSelfUsageExportSheet,
  writeSelfUsageExportFile,
} from './lib/usage-export'
import type {
  GetSelfUsageParams,
  UsageExportGranularity,
  UsageOrganizationOption,
} from './types'

const LazyUsageCharts = lazy(() =>
  import('./components/usage-charts').then((module) => ({
    default: module.UsageCharts,
  }))
)

export interface UsageSearch {
  start?: string
  end?: string
  model?: string
  token?: string
  tokenId?: string
  billingAccountType?: string
  organizationId?: string
  overviewOrganizationId?: string
  page?: number
  pageSize?: number
}

interface UsagePageProps {
  search: UsageSearch
  onSearchChange: (next: Partial<UsageSearch>) => void
}

export function UsagePage(props: UsagePageProps) {
  const { t } = useTranslation()
  const window = usageWindowFromSearch(props.search)
  const dateParams = usageDateParams(window)
  const singleDay = dateParams.start === dateParams.end

  const organizationsQuery = useQuery({
    queryKey: ['usage-organizations'],
    queryFn: async (): Promise<UsageOrganizationOption[]> => {
      const result = await getOrganizations()
      return (result.data ?? []).map((organization) => ({
        id: organization.id,
        name: organization.name,
      }))
    },
  })
  const organizations = organizationsQuery.data ?? []
  const currentContextOrganizationId = useAccountContextStore((state) =>
    state.accountContext.current?.type === ACCOUNT_CONTEXT_TYPE.ORGANIZATION
      ? state.accountContext.current.id
      : null
  )
  const requestedOrganizationId = props.search.overviewOrganizationId
    ? Number(props.search.overviewOrganizationId)
    : null
  // URL 里的组织可能已被删除或用户已退出；校验后再用，避免后端 403。
  const selectedOrganizationId =
    requestedOrganizationId !== null &&
    organizations.some(
      (organization) => organization.id === requestedOrganizationId
    )
      ? requestedOrganizationId
      : resolveOverviewOrganizationId(
          organizations,
          currentContextOrganizationId
        )

  const params: GetSelfUsageParams = {
    start: dateParams.start,
    end: dateParams.end,
    p: props.search.page ?? 1,
    page_size: props.search.pageSize ?? 20,
    ...(props.search.model ? { model_name: props.search.model } : {}),
    ...(props.search.token ? { token_name: props.search.token } : {}),
    ...(props.search.tokenId ? { token_id: Number(props.search.tokenId) } : {}),
    ...(props.search.billingAccountType
      ? { billing_account_type: props.search.billingAccountType }
      : {}),
    ...(props.search.organizationId
      ? { organization_id: Number(props.search.organizationId) }
      : {}),
    ...(selectedOrganizationId
      ? { overview_organization_id: selectedOrganizationId }
      : {}),
    ...(singleDay ? { chart_granularity: 'hour' as const } : {}),
  }

  const usageQuery = useQuery({
    queryKey: ['self-usage', params],
    queryFn: () => getSelfUsage(params),
    placeholderData: (previousData) => previousData,
    // 回填完成前页面只显示提示；轮询到完成即可自动放出数据。
    refetchInterval: (query) =>
      usageRefetchInterval(query.state.data?.data?.backfill_complete),
  })
  const data = usageQuery.data?.data
  const failed = usageQuery.isError || usageQuery.data?.success === false
  // 回填完成前旧任务日志会被当成 text，行数与金额都不准；按设计（§7.3）
  // 只显示说明，不暴露任何用量数字。
  const backfillPending = data ? !data.backfill_complete : false

  const [exporting, setExporting] = useState<UsageExportGranularity | null>(
    null
  )

  const handleExport = async (granularity: UsageExportGranularity) => {
    setExporting(granularity)
    try {
      const response = await exportSelfUsage({
        start: dateParams.start,
        end: dateParams.end,
        granularity,
        ...(props.search.model ? { model_name: props.search.model } : {}),
        ...(props.search.token ? { token_name: props.search.token } : {}),
        ...(props.search.tokenId
          ? { token_id: Number(props.search.tokenId) }
          : {}),
        ...(props.search.billingAccountType
          ? { billing_account_type: props.search.billingAccountType }
          : {}),
        ...(props.search.organizationId
          ? { organization_id: Number(props.search.organizationId) }
          : {}),
      })
      if (!response.success || !response.data) {
        toast.error(response.message || t('The export could not be prepared.'))
        return
      }
      if (response.data.rows.length === 0) {
        toast.info(t('No usage in this period.'))
        return
      }
      await writeSelfUsageExportFile(
        buildSelfUsageExportSheet(response.data, t)
      )
      if (response.data.truncated) {
        toast.warning(
          t('Exported the first {{exported}} of {{total}} records.', {
            exported: response.data.rows.length,
            total: response.data.total,
          })
        )
      }
    } catch {
      toast.error(t('The export could not be prepared.'))
    } finally {
      setExporting(null)
    }
  }

  const updateSearch = (next: Partial<UsageSearch>) => {
    props.onSearchChange(next)
  }
  const initialFilters: UsageFilterValues = {
    model: props.search.model,
    token: props.search.token,
    tokenId: props.search.tokenId ? Number(props.search.tokenId) : undefined,
    billingAccountType: props.search.billingAccountType,
    organizationId: props.search.organizationId
      ? Number(props.search.organizationId)
      : undefined,
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
          <div className='flex flex-wrap items-center gap-3'>
            <UsageDateRangePicker
              start={window.start}
              end={window.end}
              onChange={(range) => {
                const next = usageDateParams(range)
                updateSearch({ start: next.start, end: next.end, page: 1 })
              }}
            />
          </div>
          {failed ? (
            <div
              role='alert'
              className='border-destructive/40 bg-destructive/5 text-destructive flex items-center justify-between gap-3 rounded-lg border px-4 py-2 text-sm'
            >
              {t('Failed to load')}
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() => void usageQuery.refetch()}
              >
                {t('Retry')}
              </Button>
            </div>
          ) : null}
          {!failed && backfillPending ? (
            <div
              role='status'
              className='rounded-lg border border-amber-500/40 bg-amber-500/5 px-4 py-2 text-sm text-amber-700 dark:text-amber-400'
            >
              {t('History is still being prepared.')}
            </div>
          ) : null}
          {!failed && !backfillPending ? (
            <>
              <UsageOverview
                data={data}
                loading={usageQuery.isLoading}
                organizations={organizations}
                selectedOrganizationId={selectedOrganizationId}
                onSelectOrganization={(id) =>
                  updateSearch({ overviewOrganizationId: String(id), page: 1 })
                }
              />
              <Suspense
                fallback={
                  <div className='bg-muted/20 h-96 animate-pulse rounded-lg border' />
                }
              >
                <LazyUsageCharts
                  points={data?.charts ?? []}
                  loading={usageQuery.isLoading}
                  singleDay={singleDay}
                />
              </Suspense>
              <div className='rounded-lg border'>
                {data ? (
                  <UsageTable
                    data={data}
                    loading={usageQuery.isLoading}
                    filtering={usageQuery.isFetching}
                    organizations={organizations}
                    initialFilters={initialFilters}
                    page={props.search.page ?? 1}
                    pageSize={props.search.pageSize ?? 20}
                    exporting={exporting}
                    onExport={handleExport}
                    onFiltersChange={(filters) =>
                      updateSearch({
                        model: filters.model ?? '',
                        token: filters.token ?? '',
                        tokenId: filters.tokenId ? String(filters.tokenId) : '',
                        billingAccountType: filters.billingAccountType ?? '',
                        organizationId: filters.organizationId
                          ? String(filters.organizationId)
                          : '',
                        page: 1,
                      })
                    }
                    onPageChange={(page, pageSize) =>
                      updateSearch({ page, pageSize })
                    }
                  />
                ) : (
                  <div className='bg-muted/20 h-48 animate-pulse rounded-lg' />
                )}
              </div>
            </>
          ) : null}
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
