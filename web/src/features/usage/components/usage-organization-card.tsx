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
import { Building2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatLogQuota, formatNumber, formatQuota } from '@/lib/format'

import type {
  SelfUsageOrganizationStat,
  UsageOrganizationOption,
} from '../types'

interface UsageOrganizationCardProps {
  organizations: UsageOrganizationOption[]
  selectedId: number | null
  onSelect: (id: number) => void
  overview?: SelfUsageOrganizationStat | null
  loading: boolean
}

export function UsageOrganizationCard(props: UsageOrganizationCardProps) {
  const { t } = useTranslation()
  if (props.organizations.length === 0) return null

  const overview = props.overview
  const items = props.organizations.map((organization) => ({
    value: String(organization.id),
    label: organization.name,
  }))
  const breakdown = overview
    ? `${t('Member remaining')}: ${formatQuota(
        Math.max(0, overview.member_quota_limit - overview.member_used_quota)
      )}\n${t('Organization balance')}: ${formatQuota(
        Math.max(
          0,
          overview.organization_quota - overview.organization_used_quota
        )
      )}`
    : undefined

  return (
    <div className='rounded-lg border'>
      <div className='flex items-center gap-1.5 px-2.5 py-2.5 sm:gap-2.5 sm:px-5 sm:py-4'>
        <IconBadge tone='chart-2' size='stat'>
          <Building2 />
        </IconBadge>
        <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
          {t('Organization')}
        </div>
        <Select
          items={items}
          value={props.selectedId === null ? null : String(props.selectedId)}
          onValueChange={(next) =>
            props.onSelect(next === null ? 0 : Number(next))
          }
        >
          <SelectTrigger
            className='ml-auto max-w-48'
            size='sm'
            aria-label={t('Select organization')}
          >
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {items.map((item) => (
              <SelectItem key={item.value} value={item.value}>
                {item.label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className='grid grid-cols-2 divide-x divide-y border-t sm:grid-cols-3'>
        <UsageMetric label={t('Cost')} value={formatLogQuota(overview?.quota ?? 0)} />
        <UsageMetric
          label={t('Requests')}
          value={formatNumber(overview?.request_count ?? 0)}
        />
        <UsageMetric
          label={t('Input Tokens')}
          value={formatNumber(overview?.prompt_tokens ?? 0)}
        />
        <UsageMetric
          label={t('Output Tokens')}
          value={formatNumber(overview?.completion_tokens ?? 0)}
        />
        <UsageMetric
          label={t('Cache Tokens')}
          value={formatNumber(overview?.cache_tokens ?? 0)}
        />
        <UsageMetric
          label={t('My available quota')}
          value={formatQuota(overview?.remaining_quota ?? 0)}
          title={breakdown}
        />
      </div>
    </div>
  )
}

function UsageMetric(props: {
  label: string
  value: string
  title?: string
}) {
  return (
    <div className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4' title={props.title}>
      <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
        {props.label}
      </div>
      <div className='text-foreground mt-1.5 font-mono text-sm font-bold tracking-tight break-all tabular-nums sm:mt-2.5 sm:text-xl'>
        {props.value}
      </div>
    </div>
  )
}
