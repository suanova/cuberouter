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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { UsageDateRangePicker } from '@/features/usage/components/usage-date-range-picker'
import type { UsageFilterValues } from '@/features/usage/components/usage-filter-bar'
import { UsageTable } from '@/features/usage/components/usage-table'
import {
  usageDateParams,
  usageWindowFromSearch,
} from '@/features/usage/lib/time-window'
import {
  buildSelfUsageExportSheet,
  writeSelfUsageExportFile,
} from '@/features/usage/lib/usage-export'
import type { UsageExportGranularity } from '@/features/usage/types'

import { exportOrganizationUsage, getOrganizationUsage } from '../api'
import { useOrganizationResourceSection } from '../hooks/use-organization-paged-query'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationUsageData } from '../types'
import {
  useOrganizationSectionRoute,
  useOrganizationSurface,
} from './organization-page-provider'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
} from './organization-section'

type OrganizationUsageSectionProps = {
  organizationId: number
  /** The caller may read the organization's usage. */
  canView: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * The organization's consumption, one row per day and member.
 *
 * The shape matches the personal usage table: text/fee logs, tasks and
 * Midjourney are merged server-side into daily buckets, and the same row is
 * reused by both pages. The organization adds the responsible member, because
 * "who spent this" is the question this page exists to answer.
 */
export function OrganizationUsageSection(props: OrganizationUsageSectionProps) {
  const { t } = useTranslation()
  const { search, navigate } = useOrganizationSectionRoute()
  const surface = useOrganizationSurface()

  const window = usageWindowFromSearch({
    start: search.usageStart,
    end: search.usageEnd,
  })
  const dateParams = usageDateParams(window)
  const page = search.page ?? 1
  const pageSize = search.pageSize ?? ORGANIZATION_DEFAULT_PAGE_SIZE
  const filters: UsageFilterValues = {
    model: search.usageModel || undefined,
    tokenId: Number(search.usageTokenId) || undefined,
    userId: Number(search.usageUserId) || undefined,
  }

  const listParams = {
    p: page,
    page_size: pageSize,
    ...dateParams,
    ...(filters.model ? { model_name: filters.model } : {}),
    ...(filters.tokenId ? { token_id: filters.tokenId } : {}),
    ...(filters.userId ? { responsible_user_id: filters.userId } : {}),
  }

  const usage = useOrganizationResourceSection<OrganizationUsageData>({
    surface,
    organizationId: props.organizationId,
    resource: 'usage',
    params: listParams,
    query: () =>
      getOrganizationUsage(surface, props.organizationId, listParams),
    enabled: props.canView,
    onForbidden: props.onForbidden,
  })

  const [exporting, setExporting] = useState<UsageExportGranularity | null>(
    null
  )

  const handleExport = async (granularity: UsageExportGranularity) => {
    setExporting(granularity)
    try {
      const response = await exportOrganizationUsage(
        surface,
        props.organizationId,
        {
          ...dateParams,
          granularity,
          ...(filters.model ? { model_name: filters.model } : {}),
          ...(filters.tokenId ? { token_id: filters.tokenId } : {}),
          ...(filters.userId ? { responsible_user_id: filters.userId } : {}),
        }
      )
      if (!response.success || !response.data) {
        toast.error(response.message || t('The export could not be prepared.'))
        return
      }
      if (response.data.rows.length === 0) {
        toast.info(t('No usage in this period.'))
        return
      }
      await writeSelfUsageExportFile(
        buildSelfUsageExportSheet(response.data, t, {
          variant: 'organization',
        })
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

  if (!props.canView) {
    return (
      <OrganizationSectionEmpty
        icon='usage'
        title={t('Usage')}
        message={t(
          'Only an organization owner or administrator can see organization-wide usage.'
        )}
      />
    )
  }

  const backfillPending = usage.data ? !usage.data.backfill_complete : false

  return (
    <OrganizationSection
      icon='usage'
      title={t('Usage')}
      description={t('Per-day usage and charges for the organization.')}
      count={usage.data?.rows.total}
    >
      <div className='flex h-full min-h-0 flex-col gap-3'>
        <div className='flex flex-wrap items-center gap-3'>
          <UsageDateRangePicker
            start={window.start}
            end={window.end}
            onChange={(range) => {
              const next = usageDateParams(range)
              navigate({
                search: (previous) => ({
                  ...previous,
                  usageStart: next.start,
                  usageEnd: next.end,
                  page: undefined,
                }),
              })
            }}
          />
        </div>
        {backfillPending ? (
          <div
            role='status'
            className='rounded-lg border border-amber-500/40 bg-amber-500/5 px-4 py-2 text-sm text-amber-700 dark:text-amber-400'
          >
            {t('History is still being prepared.')}
          </div>
        ) : (
          <div className='min-h-0 flex-1 rounded-lg border'>
            {usage.data ? (
              <UsageTable
                data={usage.data}
                loading={usage.isLoading}
                filtering={usage.isFetching}
                organizations={[]}
                variant='organization'
                initialFilters={filters}
                page={page}
                pageSize={pageSize}
                exporting={exporting}
                onExport={handleExport}
                onFiltersChange={(values) =>
                  navigate({
                    search: (previous) => ({
                      ...previous,
                      usageModel: values.model ?? '',
                      usageTokenId: values.tokenId
                        ? String(values.tokenId)
                        : '',
                      usageUserId: values.userId ? String(values.userId) : '',
                      page: undefined,
                    }),
                  })
                }
                onPageChange={(nextPage, nextPageSize) =>
                  navigate({
                    search: (previous) => ({
                      ...previous,
                      page: nextPage,
                      pageSize: nextPageSize,
                    }),
                  })
                }
              />
            ) : (
              <div className='bg-muted/20 h-48 animate-pulse rounded-lg' />
            )}
          </div>
        )}
      </div>
    </OrganizationSection>
  )
}
