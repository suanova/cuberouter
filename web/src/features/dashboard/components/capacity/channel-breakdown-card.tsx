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
import { useQuery } from '@tanstack/react-query'
import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

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
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import {
  getPerfMetrics,
  getPerfMetricsSummary,
} from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { PerformanceGroup } from '@/features/performance-metrics/types'
import { ROLE } from '@/lib/roles'
import { cn } from '@/lib/utils'
import { useAuthStore } from '@/stores/auth-store'

const CHANNEL_WINDOW_HOURS = 24

type ChannelRow = {
  key: string
  group: string
  channelId: number
  channelName: string
  /** null when the cached response predates per-bucket request counts. */
  requests: number | null
  avgLatencyMs: number
  p95LatencyMs: number
  successRate: number
  avgTps: number
}

// Each channel is aggregated over its series buckets. -1 percentile values
// (the backend's no-data sentinel) and empty buckets (0 latency / TPS) are
// skipped; the format helpers render a dash when nothing was collected.
function average(values: number[]): number {
  if (values.length === 0) return Number.NaN
  return values.reduce((sum, value) => sum + value, 0) / values.length
}

function toChannelRows(groups: PerformanceGroup[]): ChannelRow[] {
  const rows: ChannelRow[] = []
  for (const group of groups) {
    for (const channel of group.channels ?? []) {
      let totalRequests = 0
      let hasRequestCounts = false
      const latencies: number[] = []
      const p95Latencies: number[] = []
      const successRates: number[] = []
      const tpsValues: number[] = []

      for (const point of channel.series) {
        if (point.request_count !== undefined) {
          hasRequestCounts = true
          totalRequests += point.request_count
        }
        if (point.avg_latency_ms > 0) latencies.push(point.avg_latency_ms)
        if (point.p95_latency_ms !== undefined && point.p95_latency_ms >= 0) {
          p95Latencies.push(point.p95_latency_ms)
        }
        if (Number.isFinite(point.success_rate)) {
          successRates.push(point.success_rate)
        }
        if (Number.isFinite(point.avg_tps) && point.avg_tps > 0) {
          tpsValues.push(point.avg_tps)
        }
      }

      rows.push({
        key: `${group.group}::${channel.channel_id}`,
        group: group.group,
        channelId: channel.channel_id,
        channelName: channel.channel_name,
        requests: hasRequestCounts ? totalRequests : null,
        avgLatencyMs: average(latencies),
        p95LatencyMs: average(p95Latencies),
        successRate: average(successRates),
        avgTps: average(tpsValues),
      })
    }
  }
  return rows
}

export function ChannelBreakdownCard() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)
  const [selectedModel, setSelectedModel] = useState<string | null>(null)

  const summaryQuery = useQuery({
    queryKey: ['perf-metrics-summary', CHANNEL_WINDOW_HOURS],
    queryFn: () => getPerfMetricsSummary(CHANNEL_WINDOW_HOURS),
    staleTime: 60 * 1000,
    retry: false,
  })

  const models = useMemo(
    () => summaryQuery.data?.data.models ?? [],
    [summaryQuery.data]
  )

  // Default to the first listed model; re-sync when the list changes and the
  // current selection no longer exists.
  useEffect(() => {
    if (models.length === 0) {
      if (selectedModel !== null) setSelectedModel(null)
      return
    }
    if (
      selectedModel === null ||
      !models.some((item) => item.model_name === selectedModel)
    ) {
      setSelectedModel(models[0].model_name)
    }
  }, [models, selectedModel])

  const model = selectedModel ?? ''
  const detailQuery = useQuery({
    queryKey: ['perf-metrics', model],
    queryFn: () => getPerfMetrics(model, CHANNEL_WINDOW_HOURS),
    enabled: model !== '',
    staleTime: 60 * 1000,
    retry: false,
  })

  const rows = useMemo(
    () => toChannelRows(detailQuery.data?.data.groups ?? []),
    [detailQuery.data]
  )
  const modelOptions = useMemo(
    () =>
      models.map((item) => ({
        value: item.model_name,
        label: item.model_name,
      })),
    [models]
  )

  if (user == null || user.role < ROLE.ADMIN) return null

  const summaryLoading = summaryQuery.isLoading
  const detailLoading = model !== '' && detailQuery.isLoading
  const noData = model === '' || (!detailLoading && rows.length === 0)

  let modelPicker: ReactNode
  if (summaryLoading) {
    modelPicker = <Skeleton className='h-7 w-44' />
  } else if (models.length > 0) {
    modelPicker = (
      <Select
        items={modelOptions}
        value={model}
        onValueChange={(value) => setSelectedModel(value)}
      >
        <SelectTrigger size='sm' className='max-w-56'>
          <SelectValue placeholder={t('Model')} />
        </SelectTrigger>
        <SelectContent alignItemWithTrigger={false}>
          <SelectGroup>
            {models.map((item) => (
              <SelectItem key={item.model_name} value={item.model_name}>
                {item.model_name}
              </SelectItem>
            ))}
          </SelectGroup>
        </SelectContent>
      </Select>
    )
  } else {
    modelPicker = null
  }

  let tableContent: ReactNode
  if (summaryLoading || detailLoading) {
    tableContent = <ChannelTableSkeleton />
  } else if (noData) {
    tableContent = (
      <div className='text-muted-foreground flex h-36 items-center justify-center rounded-md border border-dashed text-xs'>
        {t('No channel data for this model')}
      </div>
    )
  } else {
    tableContent = (
      <Table className='text-xs [&_td]:text-xs [&_th]:text-xs'>
        <TableHeader>
          <TableRow>
            <TableHead>{t('Group')}</TableHead>
            <TableHead>{t('Channel')}</TableHead>
            <TableHead className='text-right'>{t('Requests')}</TableHead>
            <TableHead className='text-right'>{t('Avg Latency')}</TableHead>
            <TableHead className='text-right'>{t('P95 Latency')}</TableHead>
            <TableHead className='text-right'>{t('Success Rate')}</TableHead>
            <TableHead className='text-right'>{t('TPS')}</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row) => (
            <TableRow key={row.key}>
              <TableCell>
                <span className='block max-w-32 truncate' title={row.group}>
                  {row.group}
                </span>
              </TableCell>
              <TableCell>
                <span
                  className={cn(
                    'block max-w-44 truncate font-mono',
                    row.channelName === '' && 'text-muted-foreground'
                  )}
                  title={row.channelName || undefined}
                >
                  {row.channelName !== ''
                    ? row.channelName
                    : `#${row.channelId}`}
                </span>
              </TableCell>
              <TableCell className='text-right'>
                {row.requests !== null ? row.requests.toLocaleString() : '—'}
              </TableCell>
              <TableCell className='text-right'>
                {formatLatency(row.avgLatencyMs)}
              </TableCell>
              <TableCell className='text-right'>
                {formatLatency(row.p95LatencyMs)}
              </TableCell>
              <TableCell
                className={cn(
                  'text-right',
                  getSuccessRateTextClass(row.successRate)
                )}
              >
                {formatUptimePct(row.successRate)}
              </TableCell>
              <TableCell className='text-right'>
                {formatThroughput(row.avgTps)}
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    )
  }

  return (
    <Card size='sm' className='h-full'>
      <CardHeader className='border-border/60 border-b'>
        <CardTitle>{t('Channel Breakdown')}</CardTitle>
        <CardAction>{modelPicker}</CardAction>
      </CardHeader>
      <CardContent>{tableContent}</CardContent>
    </Card>
  )
}

function ChannelTableSkeleton() {
  return (
    <div className='space-y-2 py-1'>
      {[0, 1, 2, 3, 4].map((key) => (
        <Skeleton key={key} className='h-8 w-full rounded-md' />
      ))}
    </div>
  )
}
