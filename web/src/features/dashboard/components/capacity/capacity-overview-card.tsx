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
import { useMemo, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { CartesianGrid, Line, LineChart, XAxis } from 'recharts'

import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import {
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
  type ChartConfig,
} from '@/components/ui/chart'
import { Skeleton } from '@/components/ui/skeleton'
import { getCapacityMetrics } from '@/features/performance-metrics/api'
import type { CapacityPoint } from '@/features/performance-metrics/types'
import { CHART_COLORS } from '@/lib/colors'
import dayjs from '@/lib/dayjs'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

const CAPACITY_WINDOW_HOURS = 24

type CapacityChartPoint = {
  tsMs: number
  rps: number
  inflight_peak: number
  rejected_503: number
  rejected_429: number
}

type CapacityMetricKey = 'rps' | 'inflight_peak' | 'rejected_503' | 'rejected_429'

// RPS per bucket is derived from the bucket's attempt count divided by the
// bucket width, which is inferred from the neighboring timestamps (the final
// bucket reuses the previous interval). Fewer than two points yields no
// interval, so the caller shows the empty state instead.
function toChartPoints(series: CapacityPoint[]): CapacityChartPoint[] {
  return series.map((point, index) => {
    const next = series[index + 1]
    const previous = series[index - 1]
    let seconds = 0
    if (next) {
      seconds = next.ts - point.ts
    } else if (previous) {
      seconds = point.ts - previous.ts
    }
    const rps =
      seconds > 0 ? Math.round((point.attempts / seconds) * 100) / 100 : 0
    return {
      tsMs: point.ts * 1000,
      rps,
      inflight_peak: point.inflight_peak,
      rejected_503: point.rejected_503,
      // 旧缓存响应不含 rejected_429 字段，缺失按 0 处理。
      rejected_429: point.rejected_429 ?? 0,
    }
  })
}

function formatTimeTick(value: unknown): string {
  const ms = typeof value === 'number' ? value : Number(value)
  return dayjs(Number.isFinite(ms) ? ms : 0).format('MM-DD HH:mm')
}

function formatAxisTick(value: unknown): string {
  const ms = typeof value === 'number' ? value : Number(value)
  return dayjs(Number.isFinite(ms) ? ms : 0).format('HH:mm')
}

export function CapacityOverviewCard() {
  const { t } = useTranslation()
  const user = useAuthStore((state) => state.auth.user)

  const capacityQuery = useQuery({
    queryKey: ['capacity-metrics', CAPACITY_WINDOW_HOURS],
    queryFn: () => getCapacityMetrics(CAPACITY_WINDOW_HOURS),
    staleTime: 60 * 1000,
    retry: false,
  })

  const chartData = useMemo(
    () => toChartPoints(capacityQuery.data?.data.series ?? []),
    [capacityQuery.data]
  )

  if (user == null || user.role < ROLE.ADMIN) return null

  let content: ReactNode
  if (capacityQuery.isLoading) {
    content = <CapacityChartSkeleton />
  } else if (chartData.length > 1) {
    content = (
      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        <CapacityMiniChart
          label={t('RPS')}
          dataKey='rps'
          color={CHART_COLORS[0]}
          data={chartData}
        />
        <CapacityMiniChart
          label={t('Inflight peak')}
          dataKey='inflight_peak'
          color={CHART_COLORS[1]}
          data={chartData}
        />
        <CapacityMiniChart
          label={t('Rejected 503')}
          dataKey='rejected_503'
          color='#ef4444'
          data={chartData}
        />
        <CapacityMiniChart
          label={t('Rate limit 429')}
          dataKey='rejected_429'
          color='#f97316'
          data={chartData}
        />
      </div>
    )
  } else {
    content = (
      <div className='text-muted-foreground flex h-36 items-center justify-center rounded-md border border-dashed text-xs'>
        {t('Not enough capacity data yet')}
      </div>
    )
  }

  return (
    <Card size='sm' className='h-full'>
      <CardHeader className='border-border/60 border-b'>
        <CardTitle>{t('Capacity (24h)')}</CardTitle>
      </CardHeader>
      <CardContent>{content}</CardContent>
    </Card>
  )
}

function CapacityMiniChart(props: {
  label: string
  dataKey: CapacityMetricKey
  color: string
  data: CapacityChartPoint[]
}) {
  const config = useMemo<ChartConfig>(
    () => ({
      [props.dataKey]: { label: props.label, color: props.color },
    }),
    [props.dataKey, props.label, props.color]
  )

  return (
    <div className='flex min-w-0 flex-col gap-1.5'>
      <span className='text-muted-foreground text-[11px] font-medium tracking-wide'>
        {props.label}
      </span>
      <ChartContainer
        config={config}
        className='aspect-auto h-28 w-full sm:h-32'
      >
        <LineChart
          data={props.data}
          margin={{ top: 4, right: 4, bottom: 0, left: 4 }}
        >
          <CartesianGrid vertical={false} />
          <XAxis
            dataKey='tsMs'
            type='number'
            domain={['dataMin', 'dataMax']}
            tickLine={false}
            axisLine={false}
            tickMargin={6}
            minTickGap={48}
            tickFormatter={formatAxisTick}
          />
          <ChartTooltip
            content={<ChartTooltipContent labelFormatter={formatTimeTick} />}
          />
          <Line
            dataKey={props.dataKey}
            type='monotone'
            stroke={props.color}
            strokeWidth={1.75}
            dot={false}
            activeDot={{ r: 3 }}
            isAnimationActive={false}
          />
        </LineChart>
      </ChartContainer>
    </div>
  )
}

function CapacityChartSkeleton() {
  return (
    <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
      {[0, 1, 2, 3].map((key) => (
        <div key={key} className='flex flex-col gap-1.5'>
          <Skeleton className='h-3.5 w-20' />
          <Skeleton className='h-28 rounded-md sm:h-32' />
        </div>
      ))}
    </div>
  )
}
