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
export type UsageType = 'text' | 'image' | 'video' | 'audio' | 'vector' | 'other'

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

export interface SelfUsageTotals {
  request_count: number
  success_count: number
  failure_count: number
  prompt_tokens: number
  completion_tokens: number
  cache_tokens: number
  quota: number
}

export interface SelfUsageData {
  backfill_complete: boolean
  range: {
    start: string
    end: string
    start_timestamp: number
    end_timestamp: number
  }
  overview: {
    personal: SelfUsageAccountStat
    organization: SelfUsageOrganizationStat | null
  }
  rows: {
    page: number
    page_size: number
    total: number
    items: SelfUsageRow[]
  }
  totals: SelfUsageTotals
}

export interface SelfUsageEnvelope {
  success: boolean
  message?: string
  data?: SelfUsageData
}

export interface GetSelfUsageParams {
  start: string
  end: string
  model_name?: string
  token_name?: string
  billing_account_type?: string
  organization_id?: number
  overview_organization_id?: number
  p: number
  page_size: number
}

export interface UsageOrganizationOption {
  id: number
  name: string
}
