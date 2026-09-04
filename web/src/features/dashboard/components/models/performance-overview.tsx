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
import { Gauge, HeartPulse, Timer } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge, type IconBadgeTone } from '@/components/ui/icon-badge'
import { Skeleton } from '@/components/ui/skeleton'
import { getPerfMetricsSummary } from '@/features/performance-metrics/api'
import {
  formatLatency,
  formatThroughput,
  formatUptimePct,
  getSuccessRateDotClass,
  getSuccessRateTextClass,
} from '@/features/performance-metrics/lib/format'
import type { PerfModelSummary } from '@/features/performance-metrics/types'
import { cn } from '@/lib/utils'

const PERFORMANCE_WINDOW_HOURS = 24
const TOP_MODEL_LIMIT = 5

type WeightedMetric = 'avg_latency_ms' | 'avg_tps' | 'success_rate'

type PerformanceSummary = {
  totalRequests: number
  avgLatencyMs: number
  avgTps: number
  successRate: number
}

function simpleAverage(
  rows: PerfModelSummary[],
  metric: WeightedMetric,
  isValid: (value: number) => boolean
): number {
  let total = 0
  let count = 0

  for (const row of rows) {
    const value = Number(row[metric])
    if (!isValid(value)) continue
    total += value
    count++
  }

  return count > 0 ? total / count : Number.NaN
}

function buildPerformanceSummary(rows: PerfModelSummary[]): PerformanceSummary {
  return {
    totalRequests: rows.length,
    avgLatencyMs: Math.round(
      simpleAverage(
        rows,
        'avg_latency_ms',
        (value) => Number.isFinite(value) && value > 0
      )
    ),
    avgTps: simpleAverage(
      rows,
      'avg_tps',
      (value) => Number.isFinite(value) && value > 0
    ),
    successRate: simpleAverage(rows, 'success_rate', Number.isFinite),
  }
}

export function PerformanceOverview() {
  const { t } = useTranslation()
  const metricsQuery = useQuery({
    queryKey: ['perf-metrics-summary', PERFORMANCE_WINDOW_HOURS],
    queryFn: () => getPerfMetricsSummary(PERFORMANCE_WINDOW_HOURS),
    staleTime: 60 * 1000,
    retry: false,
  })

  const models = useMemo(
    () => metricsQuery.data?.data.models ?? [],
    [metricsQuery.data]
  )
  const summary = useMemo(() => buildPerformanceSummary(models), [models])
  const topModels = useMemo(() => models.slice(0, TOP_MODEL_LIMIT), [models])
  const overflowModels = useMemo(() => models.slice(TOP_MODEL_LIMIT), [models])
  const overflowCount = overflowModels.length
  const loading = metricsQuery.isLoading
  const hasData = models.length > 0
  // +N 溢出展开的局部 UI 状态：模型列表由服务端按 request_count 排序，此处
  // 保持接收顺序即可；chip 仅在 overflowCount > 0 时渲染，列表刷新后无溢出时
  // 展开态无可见影响，故无需随查询重置。展开时剩余徽章续接同一 wrap（三元
  // 两侧均为已 memo 的数组，不产生新引用）。
  const [modelsExpanded, setModelsExpanded] = useState(false)
  const visibleModels = modelsExpanded ? models : topModels

  if (!loading && !hasData) {
    return (
      <div className='text-muted-foreground overflow-hidden rounded-lg border px-4 py-3 text-center text-xs'>
        {t('No performance data available')}
      </div>
    )
  }

  return (
    <div className='overflow-hidden rounded-lg border'>
      <div className='flex flex-wrap items-center gap-x-5 gap-y-2.5 px-4 py-2.5 sm:px-5 sm:py-3'>
        {/* Title */}
        <div className='flex items-center gap-1.5'>
          <IconBadge tone='success' size='xs'>
            <HeartPulse />
          </IconBadge>
          <span className='text-xs font-semibold whitespace-nowrap'>
            {t('Performance health')}
          </span>
        </div>

        {/* Separator */}
        <div className='bg-border hidden h-4 w-px sm:block' />

        {/* 3 KPI inline metrics */}
        {loading ? (
          <div className='flex flex-wrap items-center gap-x-5 gap-y-2'>
            {['success', 'latency', 'throughput'].map((key) => (
              <div key={key} className='flex items-center gap-1.5'>
                <Skeleton className='h-3 w-14' />
                <Skeleton className='h-4 w-16' />
              </div>
            ))}
          </div>
        ) : (
          <div className='flex flex-wrap items-center gap-x-5 gap-y-2'>
            <InlineMetric
              icon={HeartPulse}
              label={t('Success rate')}
              value={formatUptimePct(summary.successRate)}
              valueClassName={getSuccessRateTextClass(summary.successRate)}
              tone='success'
            />
            <InlineMetric
              icon={Timer}
              label={t('Average latency')}
              value={formatLatency(summary.avgLatencyMs)}
              tone='warning'
            />
            <InlineMetric
              icon={Gauge}
              label={t('Throughput')}
              value={formatThroughput(summary.avgTps)}
              tone='info'
            />
          </div>
        )}

        {/* Separator */}
        <div className='bg-border hidden h-4 w-px lg:block' />

        {/* Top models inline badges */}
        {!loading && hasData && (
          <div className='flex flex-wrap items-center gap-1.5'>
            {visibleModels.map((model) => (
              <ModelBadge key={model.model_name} model={model} />
            ))}
            {overflowCount > 0 && (
              <button
                type='button'
                aria-expanded={modelsExpanded}
                onClick={() => setModelsExpanded((open) => !open)}
                className='bg-muted/50 text-muted-foreground hover:bg-muted inline-flex items-center rounded-full px-2.5 py-1 font-mono text-[11px] transition-colors'
              >
                {t('+{{count}} Models', { count: overflowCount })}
              </button>
            )}
          </div>
        )}
      </div>
    </div>
  )
}

function InlineMetric(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  valueClassName?: string
  tone: IconBadgeTone
}) {
  const Icon = props.icon

  return (
    <div className='flex items-center gap-1.5'>
      <IconBadge tone={props.tone} size='xs'>
        <Icon />
      </IconBadge>
      <span className='text-muted-foreground text-[11px]'>{props.label}</span>
      <span
        className={cn(
          'font-mono text-xs font-semibold tabular-nums',
          props.valueClassName
        )}
      >
        {props.value}
      </span>
    </div>
  )
}

function ModelBadge(props: { model: PerfModelSummary }) {
  const model = props.model

  return (
    <span className='bg-muted/50 inline-flex items-center gap-1.5 rounded-full px-2.5 py-1'>
      <span className='max-w-[10rem] truncate font-mono text-[11px]'>
        {model.model_name}
      </span>
      <span
        className={cn(
          'size-1.5 rounded-full',
          getSuccessRateDotClass(model.success_rate)
        )}
        aria-hidden='true'
      />
      <span
        className={cn(
          'font-mono text-[11px] font-semibold tabular-nums',
          getSuccessRateTextClass(model.success_rate)
        )}
      >
        {formatUptimePct(model.success_rate)}
      </span>
    </span>
  )
}
