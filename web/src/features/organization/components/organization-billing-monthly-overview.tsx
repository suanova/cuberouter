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
import { Activity, CalendarDays, Coins, Download, TrendingUp } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { formatNumber, formatQuota } from '@/lib/format'

import { listOrganizationBillingMonthlySummaries } from '../api'
import { useOrganizationResourceSection } from '../hooks/use-organization-paged-query'
import {
  buildOrganizationBillingCsv,
  currentOrganizationBillingMonth,
  downloadOrganizationBillingCsv,
  normalizeOrganizationBillingMonths,
  organizationBillingCsvFilename,
  organizationBillingMonthlyOverviewStats,
  ORGANIZATION_BILLING_MONTH_COUNTS,
  type OrganizationCsvColumn,
} from '../lib'
import type {
  OrganizationBillingMonthlySummaryItem,
  OrganizationBillingMonthlySummaryResponse,
} from '../types'
import { OrganizationSectionRefresh } from './organization-section'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

type OrganizationBillingMonthlyOverviewProps = {
  organizationId: number
  /** The caller reads every member's rows, not just their own. */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * What the organization cost, month by month.
 *
 * The look-back starts at the organization's creation month, so a short-lived
 * organization answers with fewer rows than were asked for — which is why the
 * average below is taken over the rows that came back rather than over the
 * count that was requested.
 */
export function OrganizationBillingMonthlyOverview(
  props: OrganizationBillingMonthlyOverviewProps
) {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()

  const months = normalizeOrganizationBillingMonths(search.billingMonths)

  const params = { months }
  const summaries = useOrganizationResourceSection<OrganizationBillingMonthlySummaryResponse>(
    {
      organizationId: props.organizationId,
      resource: 'billing-monthly-summaries',
      params,
      query: () =>
        listOrganizationBillingMonthlySummaries(props.organizationId, params),
      onForbidden: props.onForbidden,
    }
  )

  const items = summaries.data?.items ?? []
  const stats = organizationBillingMonthlyOverviewStats(
    items,
    currentOrganizationBillingMonth()
  )

  const handleExport = () => {
    const columns: OrganizationCsvColumn<OrganizationBillingMonthlySummaryItem>[] =
      [
        { title: t('Month'), value: (row) => row.month || '' },
        { title: t('Cost'), value: (row) => row.quota || 0 },
        { title: t('Requests'), value: (row) => row.request_count || 0 },
        { title: t('Prompt tokens'), value: (row) => row.prompt_tokens || 0 },
        {
          title: t('Completion tokens'),
          value: (row) => row.completion_tokens || 0,
        },
        { title: t('Keys'), value: (row) => row.token_count || 0 },
        // The endpoint answers every organization with this count; only a
        // caller who may see the members behind it gets the column, so an
        // exported file says no more than the screen does.
        ...(props.canViewWideData
          ? [
              {
                title: t('Members charged'),
                value: (row: OrganizationBillingMonthlySummaryItem) =>
                  row.responsible_user_count || 0,
              },
            ]
          : []),
      ]

    downloadOrganizationBillingCsv(
      buildOrganizationBillingCsv(columns, items),
      organizationBillingCsvFilename('monthly-overview')
    )
  }

  return (
    <div className='flex flex-col gap-3'>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <StatCard
          title={t('Total cost')}
          value={formatQuota(stats.totalQuota)}
          description={t('Charged to the organization over these months.')}
          icon={Coins}
          tone='accent-1'
          loading={summaries.isLoading}
        />
        <StatCard
          title={t('Average cost')}
          value={formatQuota(stats.averageQuota)}
          description={t('Per month with any activity.')}
          icon={TrendingUp}
          tone='accent-2'
          loading={summaries.isLoading}
        />
        <StatCard
          title={t('Peak month')}
          value={
            stats.peak ? `${stats.peak.month} · ${formatQuota(stats.peak.quota)}` : '-'
          }
          description={t('The most expensive month in this range.')}
          icon={Activity}
          tone='accent-3'
          loading={summaries.isLoading}
        />
        <StatCard
          title={t('This month')}
          value={stats.current ? formatQuota(stats.current.quota) : '-'}
          description={t('Charged since the first of the month.')}
          icon={CalendarDays}
          tone='accent-1'
          loading={summaries.isLoading}
        />
      </div>

      <Card size='sm'>
        <CardHeader className='border-border/60 border-b'>
          <CardTitle>{t('Monthly cost')}</CardTitle>
          <CardAction className='flex flex-wrap items-center gap-2'>
            <Select
              value={String(months)}
              onValueChange={(value) =>
                void navigate({
                  search: (previous) => ({
                    ...previous,
                    billingMonths: normalizeOrganizationBillingMonths(value),
                  }),
                })
              }
            >
              <SelectTrigger size='sm' aria-label={t('Months to show')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {ORGANIZATION_BILLING_MONTH_COUNTS.map((count) => (
                  <SelectItem key={count} value={String(count)}>
                    {t('Last {{count}} months', { count })}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Button
              type='button'
              variant='outline'
              size='sm'
              onClick={handleExport}
              disabled={items.length === 0}
            >
              <Download className='size-4' />
              {t('Export')}
            </Button>
            <OrganizationSectionRefresh
              onClick={summaries.refetch}
              isFetching={summaries.isFetching}
            />
          </CardAction>
        </CardHeader>
        <CardContent>
          {items.length === 0 ? (
            <div className='text-muted-foreground flex h-32 items-center justify-center rounded-md border border-dashed text-xs'>
              {summaries.isLoading
                ? t('Loading...')
                : t('No cost in these months.')}
            </div>
          ) : (
            <Table className='text-xs [&_td]:text-xs [&_th]:text-xs'>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Month')}</TableHead>
                  <TableHead className='text-right'>{t('Cost')}</TableHead>
                  <TableHead className='text-right'>{t('Requests')}</TableHead>
                  <TableHead className='text-right'>{t('Prompt tokens')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Completion tokens')}
                  </TableHead>
                  <TableHead className='text-right'>{t('Keys')}</TableHead>
                  {props.canViewWideData && (
                    <TableHead className='text-right'>
                      {t('Members charged')}
                    </TableHead>
                  )}
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((row) => (
                  <TableRow key={row.month}>
                    <TableCell className='font-medium whitespace-nowrap'>
                      {row.month || '-'}
                    </TableCell>
                    <TableCell className='text-right font-medium'>
                      {formatQuota(row.quota || 0)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.request_count || 0)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.prompt_tokens || 0)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.completion_tokens || 0)}
                    </TableCell>
                    <TableCell className='text-right'>
                      {formatNumber(row.token_count || 0)}
                    </TableCell>
                    {props.canViewWideData && (
                      <TableCell className='text-right'>
                        {formatNumber(row.responsible_user_count || 0)}
                      </TableCell>
                    )}
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
