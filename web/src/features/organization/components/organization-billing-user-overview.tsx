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
import { Activity, Coins, Download, UserCheck, Users } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
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

import { listOrganizationBillingUserSummaries } from '../api'
import { organizationRoleMeta } from '../constants'
import { useOrganizationResourceSection } from '../hooks/use-organization-paged-query'
import {
  buildOrganizationBillingCsv,
  downloadOrganizationBillingCsv,
  organizationBillingCsvFilename,
  organizationBillingMonthRange,
  organizationBillingResponsibleName,
  organizationBillingUserOverviewStats,
  recentOrganizationBillingMonths,
  type OrganizationCsvColumn,
} from '../lib'
import type {
  OrganizationBillingUserSummaryItem,
  OrganizationBillingUserSummaryResponse,
} from '../types'
import { useOrganizationSectionRoute, useOrganizationSurface } from './organization-page-provider'
import { OrganizationSectionRefresh } from './organization-section'

type OrganizationBillingUserOverviewProps = {
  organizationId: number
  /** The caller reads every member's rows, not just their own. */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * What each member cost over a range of months.
 *
 * The range defaults to the current month on both sides, which is the question
 * a reader arrives with; widening it is two selects away. The endpoint answers
 * with the whole range at once, so this is one read per range rather than one
 * per month.
 */
export function OrganizationBillingUserOverview(
  props: OrganizationBillingUserOverviewProps
) {
  const { t } = useTranslation()
  const { search, navigate } = useOrganizationSectionRoute()
  const surface = useOrganizationSurface()

  const range = organizationBillingMonthRange(
    search.billingStartMonth,
    search.billingEndMonth
  )
  const months = recentOrganizationBillingMonths()

  const params = { start_month: range.start, end_month: range.end }
  const summaries = useOrganizationResourceSection<OrganizationBillingUserSummaryResponse>(
    {
      surface,
      organizationId: props.organizationId,
      resource: 'billing-user-summaries',
      params,
      query: () =>
        listOrganizationBillingUserSummaries(surface, props.organizationId, params),
      onForbidden: props.onForbidden,
    }
  )

  const items = summaries.data?.items ?? []
  const stats = organizationBillingUserOverviewStats(items)

  const setMonth = (key: 'billingStartMonth' | 'billingEndMonth', value: string) =>
    void navigate({ search: (previous) => ({ ...previous, [key]: value }) })

  const handleExport = () => {
    const columns: OrganizationCsvColumn<OrganizationBillingUserSummaryItem>[] = [
      {
        title: t('Member'),
        value: (row) => organizationBillingResponsibleName(row),
      },
    ]

    if (props.canViewWideData) {
      columns.push({
        title: t('Role'),
        value: (row) => {
          const meta = organizationRoleMeta(row.role)
          return meta ? t(meta.labelKey) : row.role || ''
        },
      })
    }

    columns.push(
      { title: t('Cost'), value: (row) => row.quota || 0 },
      { title: t('Requests'), value: (row) => row.request_count || 0 },
      { title: t('Prompt Tokens'), value: (row) => row.prompt_tokens || 0 },
      {
        title: t('Completion Tokens'),
        value: (row) => row.completion_tokens || 0,
      },
      { title: t('Keys'), value: (row) => row.token_count || 0 }
    )

    downloadOrganizationBillingCsv(
      buildOrganizationBillingCsv(columns, items),
      organizationBillingCsvFilename('user-overview')
    )
  }

  const monthItems = months.map((month) => ({ value: month, label: month }))

  return (
    <div className='flex flex-col gap-3'>
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
        <StatCard
          title={t('Total cost')}
          value={formatQuota(stats.totalQuota)}
          description={t('Charged to the organization in this range.')}
          icon={Coins}
          tone='accent-1'
          loading={summaries.isLoading}
        />
        <StatCard
          title={t('Requests')}
          value={formatNumber(stats.totalRequests)}
          description={t('Relay requests charged to this organization.')}
          icon={Activity}
          tone='accent-2'
          loading={summaries.isLoading}
        />
        <StatCard
          title={t('Active members')}
          value={formatNumber(stats.activeUsers)}
          description={t('Members with cost or traffic in this range.')}
          icon={Users}
          tone='accent-3'
          loading={summaries.isLoading}
        />
        <StatCard
          title={t('Top spender')}
          value={
            stats.topUser
              ? `${organizationBillingResponsibleName(stats.topUser)} · ${formatQuota(stats.topUser.quota)}`
              : '-'
          }
          description={t('The member charged the most in this range.')}
          icon={UserCheck}
          tone='accent-1'
          loading={summaries.isLoading}
        />
      </div>

      <Card size='sm'>
        <CardHeader className='border-border/60 border-b'>
          <CardTitle>
            {t('Members')} · {range.start} → {range.end}
          </CardTitle>
          <CardAction className='flex flex-wrap items-center gap-2'>
            <Select
              items={monthItems}
              value={range.start}
              onValueChange={(value) =>
                setMonth('billingStartMonth', String(value))
              }
            >
              <SelectTrigger size='sm' aria-label={t('From month')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {monthItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <Select
              items={monthItems}
              value={range.end}
              onValueChange={(value) => setMonth('billingEndMonth', String(value))}
            >
              <SelectTrigger size='sm' aria-label={t('To month')}>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {monthItems.map((item) => (
                  <SelectItem key={item.value} value={item.value}>
                    {item.label}
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
                : t('No member was charged in this range.')}
            </div>
          ) : (
            <Table className='text-xs [&_td]:text-xs [&_th]:text-xs'>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('Member')}</TableHead>
                  {props.canViewWideData && <TableHead>{t('Role')}</TableHead>}
                  <TableHead className='text-right'>{t('Cost')}</TableHead>
                  <TableHead className='text-right'>{t('Requests')}</TableHead>
                  <TableHead className='text-right'>{t('Prompt Tokens')}</TableHead>
                  <TableHead className='text-right'>
                    {t('Completion Tokens')}
                  </TableHead>
                  <TableHead className='text-right'>{t('Keys')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {items.map((row) => {
                  const role = organizationRoleMeta(row.role)
                  return (
                    <TableRow key={row.responsible_user_id}>
                      <TableCell className='font-medium whitespace-nowrap'>
                        {organizationBillingResponsibleName(row)}
                      </TableCell>
                      {props.canViewWideData && (
                        <TableCell>
                          {role ? (
                            <StatusBadge variant={role.variant} copyable={false}>
                              {t(role.labelKey)}
                            </StatusBadge>
                          ) : (
                            (row.role ?? '-')
                          )}
                        </TableCell>
                      )}
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
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
