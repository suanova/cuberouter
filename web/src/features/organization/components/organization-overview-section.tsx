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
import { Activity, Coins, Gauge, Wallet } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Bar, BarChart, CartesianGrid, Line, LineChart, XAxis, YAxis } from 'recharts'

import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { StatCard } from '@/features/dashboard/components/ui/stat-card'
import { CompactDateTimeRangePicker } from '@/features/usage-logs/components/compact-date-time-range-picker'
import { CHART_COLORS } from '@/lib/colors'
import dayjs from '@/lib/dayjs'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'

import { getOrganizationQuotaData } from '../api'
import { useOrganizationResourceSection } from '../hooks/use-organization-paged-query'
import type { Organization, OrganizationQuotaDataRow } from '../types'
import { useOrganizationSurface } from './organization-page-provider'
import { OrganizationSection, OrganizationSectionEmpty } from './organization-section'

/**
 * The backend rejects a window wider than this with `success: false`.
 * service/organization.go enforces the limit on the quota-data endpoint.
 */
const MAX_WINDOW_DAYS = 30

/** How far back the section looks before the operator narrows it. */
const DEFAULT_WINDOW_DAYS = 7

type OrganizationOverviewSectionProps = {
  organization: Organization
  /** The caller may read the organization's aggregated usage. */
  canViewUsage: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * Where an organization stands: its quota, its traffic, and which models the
 * traffic went to.
 *
 * Every number here belongs to the organization, not to whoever is looking at
 * it, which is why the read is gated on `can_view_organization_usage` rather
 * than being shown to all members.
 */
export function OrganizationOverviewSection(
  props: OrganizationOverviewSectionProps
) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const [range, setRange] = useState(() => defaultWindow())

  // A window the operator dragged past the limit is clamped rather than
  // rejected: the backend would answer `success: false`, which reads as an
  // empty dashboard and hides the fact that a narrower window would work.
  const window = clampWindow(range)
  const params = {
    start_timestamp: window.start
      ? Math.floor(window.start.getTime() / 1000)
      : undefined,
    end_timestamp: window.end ? Math.floor(window.end.getTime() / 1000) : undefined,
  }

  const usage = useOrganizationResourceSection<OrganizationQuotaDataRow[]>({
    surface,
    organizationId: props.organization.id,
    resource: 'quota-data',
    params,
    query: () => getOrganizationQuotaData(surface, props.organization.id, params),
    enabled: props.canViewUsage,
    onForbidden: props.onForbidden,
  })

  const rows = usage.data ?? []

  if (!props.canViewUsage) {
    return (
      <OrganizationSectionEmpty
        icon='overview'
        title={t('Overview')}
        message={t(
          'Only an organization owner or administrator can see organization-wide usage.'
        )}
      />
    )
  }

  return (
    <OrganizationSection
      icon='overview'
      title={t('Overview')}
      description={t('Quota, traffic and model usage for the whole organization.')}
      actions={
        <CompactDateTimeRangePicker
          start={range.start}
          end={range.end}
          onChange={setRange}
        />
      }
      scroll
    >
      <div className='flex flex-col gap-3'>
        <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-4'>
          <StatCard
            title={t('Remaining quota')}
            value={formatQuota(props.organization.quota)}
            description={t('Shared by every API key in this organization.')}
            icon={Wallet}
            tone='accent-1'
            loading={usage.isLoading}
          />
          <StatCard
            title={t('Used Quota')}
            value={formatQuota(props.organization.used_quota)}
            description={t('Consumed since the organization was created.')}
            icon={Coins}
            tone='accent-2'
            loading={usage.isLoading}
          />
          <StatCard
            title={t('Requests')}
            value={formatNumber(props.organization.request_count)}
            description={t('Relay requests charged to this organization.')}
            icon={Activity}
            tone='accent-3'
            loading={usage.isLoading}
          />
          <StatCard
            title={t('Created')}
            value={formatTimestamp(props.organization.created_at)}
            description={t('When the organization was created.')}
            icon={Gauge}
            tone='accent-1'
            loading={usage.isLoading}
          />
        </div>

        <div className='grid grid-cols-1 gap-3 xl:grid-cols-2'>
          <ModelCallChart rows={rows} isLoading={usage.isLoading} />
          <ModelQuotaChart rows={rows} isLoading={usage.isLoading} />
        </div>
      </div>
    </OrganizationSection>
  )
}

/**
 * One bucket per model, biggest first.
 *
 * The window is aggregated client-side because the endpoint answers with the
 * raw per-request rows: a chart of them is a sum, and doing the sum here means
 * the two charts below share one fetch.
 */
function aggregateByModel(rows: OrganizationQuotaDataRow[]) {
  const byModel = new Map<string, { model: string; count: number; quota: number }>()
  for (const row of rows) {
    const model = row.model_name || '-'
    const entry = byModel.get(model) ?? { model, count: 0, quota: 0 }
    entry.count += row.count || 0
    entry.quota += row.quota || 0
    byModel.set(model, entry)
  }
  return [...byModel.values()].sort((a, b) => b.count - a.count)
}

function ModelCallChart(props: { rows: OrganizationQuotaDataRow[]; isLoading: boolean }) {
  const { t } = useTranslation()
  const data = useMemo(() => aggregateByModel(props.rows).slice(0, 10), [props.rows])
  const config = useMemo<ChartConfig>(
    () => ({ count: { label: t('Requests'), color: CHART_COLORS[0] } }),
    [t]
  )

  return (
    <Card size='sm'>
      <CardHeader className='border-border/60 border-b'>
        <CardTitle>{t('Requests by Model')}</CardTitle>
      </CardHeader>
      <CardContent>
        {data.length === 0 ? (
          <ChartPlaceholder isLoading={props.isLoading} />
        ) : (
          <ChartContainer config={config} className='aspect-auto h-64 w-full'>
            <BarChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: 8 }}>
              <CartesianGrid vertical={false} />
              <XAxis dataKey='model' tickLine={false} axisLine={false} />
              <YAxis tickLine={false} axisLine={false} width={48} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Bar dataKey='count' fill='var(--color-count)' radius={[4, 4, 0, 0]} />
            </BarChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}

function ModelQuotaChart(props: { rows: OrganizationQuotaDataRow[]; isLoading: boolean }) {
  const { t } = useTranslation()
  const data = useMemo(
    () =>
      aggregateByModel(props.rows)
        .sort((a, b) => b.quota - a.quota)
        .slice(0, 10)
        .map((row) => ({ model: row.model, quota: row.quota })),
    [props.rows]
  )
  const config = useMemo<ChartConfig>(
    () => ({ quota: { label: t('Quota'), color: CHART_COLORS[1] } }),
    [t]
  )

  return (
    <Card size='sm'>
      <CardHeader className='border-border/60 border-b'>
        <CardTitle>{t('Quota by Model')}</CardTitle>
      </CardHeader>
      <CardContent>
        {data.length === 0 ? (
          <ChartPlaceholder isLoading={props.isLoading} />
        ) : (
          <ChartContainer config={config} className='aspect-auto h-64 w-full'>
            <LineChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: 8 }}>
              <CartesianGrid vertical={false} />
              <XAxis dataKey='model' tickLine={false} axisLine={false} />
              <YAxis tickLine={false} axisLine={false} width={64} />
              <ChartTooltip content={<ChartTooltipContent />} />
              <Line
                dataKey='quota'
                type='monotone'
                stroke='var(--color-quota)'
                strokeWidth={2}
                dot={false}
              />
            </LineChart>
          </ChartContainer>
        )}
      </CardContent>
    </Card>
  )
}

function ChartPlaceholder(props: { isLoading: boolean }) {
  const { t } = useTranslation()
  return (
    <div className='text-muted-foreground flex h-64 items-center justify-center rounded-md border border-dashed text-xs'>
      {props.isLoading ? t('Loading...') : t('No usage in this period.')}
    </div>
  )
}

function defaultWindow(): { start?: Date; end?: Date } {
  return { start: dayjs().subtract(DEFAULT_WINDOW_DAYS, 'day').toDate(), end: new Date() }
}

/**
 * Narrows a range to the window the endpoint accepts, measured back from its
 * end so a deliberately chosen end time is honoured.
 */
function clampWindow(range: { start?: Date; end?: Date }): {
  start?: Date
  end?: Date
} {
  const end = range.end ?? new Date()
  if (!range.start) return { start: undefined, end }
  const earliest = dayjs(end).subtract(MAX_WINDOW_DAYS, 'day').toDate()
  return { start: range.start < earliest ? earliest : range.start, end }
}
