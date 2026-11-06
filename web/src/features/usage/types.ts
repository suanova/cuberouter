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
export type UsageType =
  | 'text'
  | 'image'
  | 'video'
  | 'audio'
  | 'vector'
  | 'other'

export interface SelfUsageAccountStat {
  request_count: number
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  quota: number
  remaining_quota: number
}

export interface SelfUsageOrganizationStat extends SelfUsageAccountStat {
  id: number
  name: string
  member_quota_limit: number
  member_used_quota: number
  organization_quota: number
  organization_used_quota: number
}

export interface SelfUsageRow {
  date: string
  type: UsageType
  billing_account_type: 'personal' | 'organization'
  organization_id: number
  organization_name: string
  responsible_user_id?: number
  responsible_name?: string
  token_id: number
  token_name: string
  model_name: string
  request_count: number
  success_count: number
  failure_count: number
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  quota: number
}

export interface SelfUsageRows {
  page: number
  page_size: number
  total: number
  items: SelfUsageRow[]
}

export interface SelfUsageTotals {
  request_count: number
  success_count: number
  failure_count: number
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  quota: number
}

export interface SelfUsageChartPoint {
  date: string
  model_name: string
  request_count: number
  quota: number
}

export interface SelfUsageRange {
  start: string
  end: string
  start_timestamp: number
  end_timestamp: number
}

export interface SelfUsageData {
  backfill_complete: boolean
  range: SelfUsageRange
  overview: {
    personal: SelfUsageAccountStat
    organization: SelfUsageOrganizationStat | null
  }
  rows: SelfUsageRows
  totals: SelfUsageTotals
  charts: SelfUsageChartPoint[]
  filter_options: SelfUsageFilterOptions
}

export interface UsageUserOption {
  id: number
  name: string
}

export interface SelfUsageFilterOptions {
  models: string[]
  tokens: SelfUsageTokenOption[]
  users?: UsageUserOption[]
}

/** The fields the shared usage table reads; both personal and organization responses provide them. */
export interface UsageTableData {
  rows: SelfUsageRows
  totals: SelfUsageTotals
  filter_options: SelfUsageFilterOptions
}

export type UsageTableVariant = 'personal' | 'organization'

export interface SelfUsageTokenOption {
  id: number
  name: string
  organization_id: number
  organization_name: string
}

export interface SelfUsageEnvelope {
  success: boolean
  message?: string
  data?: SelfUsageData
}

export type UsageExportGranularity = 'day' | 'hour'

export interface SelfUsageExportData {
  granularity: UsageExportGranularity
  rows: SelfUsageRow[]
  total: number
  truncated: boolean
}

export interface SelfUsageExportEnvelope {
  success: boolean
  message?: string
  data?: SelfUsageExportData
}

export interface GetSelfUsageParams {
  start: string
  end: string
  model_name?: string
  token_name?: string
  token_id?: number
  billing_account_type?: string
  organization_id?: number
  overview_organization_id?: number
  /** "hour" asks the backend for hourly chart buckets; the table stays daily. */
  chart_granularity?: 'hour'
  p: number
  page_size: number
}

export interface ExportSelfUsageParams {
  start: string
  end: string
  granularity: UsageExportGranularity
  model_name?: string
  token_name?: string
  token_id?: number
  billing_account_type?: string
  organization_id?: number
}

export interface UsageOrganizationOption {
  id: number
  name: string
}
