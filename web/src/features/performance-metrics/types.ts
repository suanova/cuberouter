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
export type PerformanceSeriesPoint = {
  ts: number
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  /** Total requests in the bucket (additive; absent in older cached responses). */
  request_count?: number
  /** Latency percentiles in ms; -1 when the bucket has no latency histogram data. */
  p50_latency_ms?: number
  p95_latency_ms?: number
  p99_latency_ms?: number
  /** TTFT p95 in ms; -1 when the bucket has no TTFT samples. */
  p95_ttft_ms?: number
}

/** Per-channel breakdown of a group's buckets. Channel identity is internal
 * information: only admin-gated views may render it, never the public pages. */
export type PerformanceChannelSeries = {
  channel_id: number
  channel_name: string
  series: PerformanceSeriesPoint[]
}

export type PerformanceGroup = {
  group: string
  avg_ttft_ms: number
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  series: PerformanceSeriesPoint[]
  /** Per-channel series (additive; absent in older cached responses). */
  channels?: PerformanceChannelSeries[]
}

export type PerformanceMetricsData = {
  success: boolean
  message?: string
  data: {
    model_name: string
    series_schema?: string
    groups: PerformanceGroup[]
  }
}

export type PerfModelSummary = {
  model_name: string
  avg_latency_ms: number
  success_rate: number
  avg_tps: number
  recent_success_rates?: number[]
  request_count?: number
}

export type PerfSummaryAllData = {
  success: boolean
  message?: string
  data: {
    models: PerfModelSummary[]
  }
}

export type CapacityPoint = {
  /** Bucket start, Unix seconds. */
  ts: number
  /** Requests counted in the bucket (includes requests later rejected with 503). */
  attempts: number
  /** Requests rejected with HTTP 503 by the relay capacity check. */
  rejected_503: number
  /** Peak in-flight concurrency in the bucket (2s sampling approximation). */
  inflight_peak: number
}

export type CapacityMetricsData = {
  success: boolean
  message?: string
  data: {
    series: CapacityPoint[]
  }
}
