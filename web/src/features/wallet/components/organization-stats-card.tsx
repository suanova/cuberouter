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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { IconBadge } from '@/components/ui/icon-badge'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatQuota } from '@/lib/format'
import {
  ACCOUNT_CONTEXT_TYPE,
  useAccountContextStore,
} from '@/stores/account-context-store'

import type { WalletOrganization } from '../types'

interface OrganizationStatsCardProps {
  organizations: WalletOrganization[]
}

export function OrganizationStatsCard(props: OrganizationStatsCardProps) {
  const { t } = useTranslation()
  const [pickedId, setPickedId] = useState<number | null>(null)
  const contextOrganizationId = useAccountContextStore((state) =>
    state.accountContext.current?.type === ACCOUNT_CONTEXT_TYPE.ORGANIZATION
      ? state.accountContext.current.id
      : null
  )

  if (props.organizations.length === 0) {
    return null
  }

  const selected =
    props.organizations.find((organization) => organization.id === pickedId) ??
    props.organizations.find(
      (organization) => organization.id === contextOrganizationId
    ) ??
    props.organizations[0]
  // A member without a configured limit draws from the organization's pool,
  // so the pool total is the ceiling shown for them.
  const quotaLimit =
    selected.member_quota_limit > 0
      ? selected.member_quota_limit
      : selected.quota
  const items = props.organizations.map((organization) => ({
    value: String(organization.id),
    label: organization.name,
  }))

  return (
    <div className='rounded-lg border'>
      <div className='flex items-center gap-1.5 px-2.5 py-2.5 sm:gap-2.5 sm:px-5 sm:py-4'>
        <IconBadge tone='chart-4' size='stat'>
          <Building2 />
        </IconBadge>
        <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
          {t('Organization')}
        </div>
        <Select
          items={items}
          value={String(selected.id)}
          onValueChange={(next) =>
            setPickedId(next === null ? null : Number(next))
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

      <div className='grid grid-cols-2 divide-x border-t'>
        <div className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4'>
          <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
            {t('Organization Quota')}
          </div>
          <div className='text-foreground mt-1.5 font-mono text-sm font-bold tracking-tight break-all tabular-nums sm:mt-2.5 sm:text-2xl'>
            {formatQuota(quotaLimit)}
          </div>
        </div>
        <div className='min-w-0 px-2.5 py-2.5 sm:px-5 sm:py-4'>
          <div className='text-muted-foreground truncate text-[11px] font-medium tracking-wider uppercase sm:text-xs'>
            {t('Used')}
          </div>
          <div className='text-foreground mt-1.5 font-mono text-sm font-bold tracking-tight break-all tabular-nums sm:mt-2.5 sm:text-2xl'>
            {formatQuota(selected.member_used_quota)}
          </div>
        </div>
      </div>
    </div>
  )
}
