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

export function PerformanceHealthPanel() {
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

  const summary = useMemo(() => {
    return {
      avgLatencyMs: Math.round(
        simpleAverage(
          models,
          'avg_latency_ms',
          (v) => Number.isFinite(v) && v > 0
        )
      ),
      avgTps: simpleAverage(
        models,
        'avg_tps',
        (v) => Number.isFinite(v) && v > 0
      ),
      successRate: simpleAverage(models, 'success_rate', Number.isFinite),
    }
  }, [models])

  const topModels = useMemo(() => models.slice(0, TOP_MODEL_LIMIT), [models])
  const overflowModels = useMemo(() => models.slice(TOP_MODEL_LIMIT), [models])
  const overflowCount = overflowModels.length
  const loading = metricsQuery.isLoading
  const hasData = models.length > 0
  // +N 溢出展开的局部 UI 状态：模型列表由服务端按 request_count 排序，此处
  // 保持接收顺序即可；chip 仅在 overflowCount > 0 时渲染，列表刷新后无溢出时
  // 展开态无可见影响，故无需随查询重置。展开时剩余模型续接同一网格（三元
  // 两侧均为已 memo 的数组，不产生新引用）。
  const [modelsExpanded, setModelsExpanded] = useState(false)
  const visibleModels = modelsExpanded ? models : topModels

  return (
    <section className='bg-card h-full overflow-hidden rounded-2xl border shadow-xs'>
      <div className='flex items-center gap-2 border-b px-4 py-3 sm:px-5'>
        <IconBadge tone='success' size='sm'>
          <HeartPulse />
        </IconBadge>
        <h3 className='text-sm font-semibold'>{t('Performance health')}</h3>
        <span className='text-muted-foreground ml-auto text-xs'>
          {t('Performance metrics for the last 24 hours')}
        </span>
      </div>

      <div className='space-y-3 p-4 sm:p-5'>
        <div className='grid grid-cols-3 gap-2'>
          <MetricCell
            icon={HeartPulse}
            label={t('Success rate')}
            value={formatUptimePct(summary.successRate)}
            loading={loading}
            valueClassName={getSuccessRateTextClass(summary.successRate)}
            tone='success'
          />
          <MetricCell
            icon={Timer}
            label={t('Average latency')}
            value={formatLatency(summary.avgLatencyMs)}
            loading={loading}
            tone='warning'
          />
          <MetricCell
            icon={Gauge}
            label={t('Throughput')}
            value={formatThroughput(summary.avgTps)}
            loading={loading}
            tone='info'
          />
        </div>

        {loading ? (
          <div className='space-y-1'>
            {['success', 'latency', 'throughput'].map((key) => (
              <Skeleton key={key} className='h-5 w-full rounded' />
            ))}
          </div>
        ) : (
          hasData && (
            <div>
              <span className='text-muted-foreground mb-1 block text-[11px] font-medium'>
                {t('Top models by traffic')}
              </span>
              <div className='grid grid-cols-1 gap-x-4 sm:grid-cols-2'>
                {visibleModels.map((model) => (
                  <div
                    key={model.model_name}
                    className='flex items-center justify-between gap-2 rounded px-1.5 py-1'
                  >
                    <span className='min-w-0 flex-1 truncate font-mono text-[11px]'>
                      {model.model_name}
                    </span>
                    <span className='inline-flex shrink-0 items-center gap-1'>
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
                  </div>
                ))}
              </div>
              {overflowCount > 0 && (
                <button
                  type='button'
                  aria-expanded={modelsExpanded}
                  onClick={() => setModelsExpanded((open) => !open)}
                  className='bg-muted/50 text-muted-foreground hover:bg-muted mt-1 inline-flex items-center rounded-full px-2.5 py-1 font-mono text-[11px] transition-colors'
                >
                  {t('+{{count}} Models', { count: overflowCount })}
                </button>
              )}
            </div>
          )
        )}
      </div>
    </section>
  )
}

function MetricCell(props: {
  icon: React.ComponentType<{ className?: string }>
  label: string
  value: string
  loading: boolean
  valueClassName?: string
  tone: IconBadgeTone
}) {
  const Icon = props.icon
  return (
    <div className='bg-muted/40 rounded-xl px-3 py-2.5'>
      <div className='text-muted-foreground flex items-center gap-1.5 text-[11px] font-medium'>
        <IconBadge tone={props.tone} size='xs'>
          <Icon />
        </IconBadge>
        <span className='truncate'>{props.label}</span>
      </div>
      {props.loading ? (
        <Skeleton className='mt-1.5 h-5 w-16' />
      ) : (
        <div
          className={cn(
            'mt-1.5 font-mono text-sm font-semibold tabular-nums',
            props.valueClassName
          )}
        >
          {props.value}
        </div>
      )}
    </div>
  )
}
