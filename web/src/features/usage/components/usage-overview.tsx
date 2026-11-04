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
import { UserRound } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import { formatLogQuota, formatNumber, formatQuota } from '@/lib/format'

import type { SelfUsageData, UsageOrganizationOption } from '../types'
import { UsageOrganizationCard } from './usage-organization-card'

interface UsageOverviewProps {
  data?: SelfUsageData
  loading: boolean
  organizations: UsageOrganizationOption[]
  selectedOrganizationId: number | null
  onSelectOrganization: (id: number) => void
}

export function UsageOverview(props: UsageOverviewProps) {
  const { t } = useTranslation()
  const personal = props.data?.overview.personal
  const pending = props.loading && !props.data

  return (
    <div className='grid gap-4 xl:grid-cols-2'>
      <div className='rounded-lg border'>
        <div className='flex items-center gap-1.5 px-2.5 py-2.5 sm:gap-2.5 sm:px-5 sm:py-4'>
          <IconBadge tone='chart-1' size='stat'>
            <UserRound />
          </IconBadge>
          <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
            {t('Personal')}
          </div>
        </div>
        <div className='grid grid-cols-2 divide-x divide-y border-t sm:grid-cols-4'>
          <UsageMetric
            label={t('Cost')}
            value={pending ? '—' : formatLogQuota(personal?.quota ?? 0)}
          />
          <UsageMetric
            label={t('Requests')}
            value={pending ? '—' : formatNumber(personal?.request_count ?? 0)}
          />
          <UsageMetric
            label={t('Tokens')}
            value={
              pending
                ? '—'
                : formatNumber(
                    (personal?.prompt_tokens ?? 0) +
                      (personal?.completion_tokens ?? 0)
                  )
            }
          />
          <UsageMetric
            label={t('Available quota')}
            value={pending ? '—' : formatQuota(personal?.remaining_quota ?? 0)}
          />
        </div>
      </div>
      <UsageOrganizationCard
        organizations={props.organizations}
        selectedId={props.selectedOrganizationId}
        onSelect={props.onSelectOrganization}
        overview={props.data?.overview.organization ?? null}
        loading={props.loading}
      />
    </div>
  )
}

function UsageMetric(props: { label: string; value: string; title?: string }) {
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
