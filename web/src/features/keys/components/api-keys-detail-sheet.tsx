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
import { Clock, Info, WalletCards } from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import {
  SideDrawerSection,
  SideDrawerSectionHeader,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { formatQuota, formatTimestamp } from '@/lib/format'

import { API_KEY_STATUSES } from '../constants'
import type { ApiKey } from '../types'

function DetailRow(props: { label: string; children: ReactNode }) {
  return (
    <div className='flex items-start justify-between gap-4 text-sm'>
      <span className='text-muted-foreground shrink-0'>{props.label}</span>
      <div className='min-w-0 text-right break-words'>{props.children}</div>
    </div>
  )
}

export function ApiKeyDetailSheet(props: {
  open: boolean
  onOpenChange: (open: boolean) => void
  apiKey: ApiKey | null
}) {
  const { t } = useTranslation()
  const { apiKey } = props

  if (!apiKey) return null

  const isOrganizationKey = apiKey.scope_type === 'organization'
  const statusConfig = API_KEY_STATUSES[apiKey.status]
  const modelsEnabled = apiKey.model_limits_enabled
  const models = modelsEnabled
    ? (apiKey.model_limits ?? '')
        .split(',')
        .map((model) => model.trim())
        .filter(Boolean)
    : []
  const allowIps = apiKey.allow_ips?.trim()

  let modelsContent: ReactNode
  if (!modelsEnabled) {
    modelsContent = t('Unlimited')
  } else if (models.length === 0) {
    modelsContent = t('No models allowed')
  } else {
    modelsContent = (
      <div className='flex flex-wrap justify-end gap-1'>
        {models.map((model) => (
          <Badge
            key={model}
            variant='secondary'
            className='font-mono text-[11px]'
          >
            {model}
          </Badge>
        ))}
      </div>
    )
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent
        className={sideDrawerContentClassName('max-w-none sm:!max-w-[520px]')}
      >
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('API Key Details')}</SheetTitle>
          <SheetDescription className='flex items-center gap-2'>
            <span className='text-foreground truncate font-medium'>
              {apiKey.name}
            </span>
            {statusConfig ? (
              <StatusBadge
                label={t(statusConfig.label)}
                variant={statusConfig.variant}
                copyable={false}
              />
            ) : null}
          </SheetDescription>
        </SheetHeader>

        <div className={sideDrawerFormClassName('gap-5')}>
          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('Basic Information')}
              icon={<Info className='size-4' />}
              iconTone='info'
            />
            <DetailRow label={t('Billing Account')}>
              {isOrganizationKey
                ? apiKey.organization_name || '-'
                : t('Personal')}
            </DetailRow>
            <DetailRow label={t('Models')}>{modelsContent}</DetailRow>
            {isOrganizationKey ? (
              <DetailRow label={t('Visibility')}>
                {apiKey.visibility === 'public' ? t('Public') : t('Private')}
              </DetailRow>
            ) : (
              <DetailRow label={t('IP Restriction')}>
                {allowIps ? (
                  <span className='break-all whitespace-pre-wrap'>
                    {allowIps}
                  </span>
                ) : (
                  t('No restriction')
                )}
              </DetailRow>
            )}
          </SideDrawerSection>

          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('Quota')}
              icon={<WalletCards className='size-4' />}
              iconTone='warning'
            />
            {apiKey.unlimited_quota ? (
              <>
                <DetailRow label={t('Quota')}>{t('Unlimited')}</DetailRow>
                {isOrganizationKey ? (
                  <p className='text-muted-foreground text-xs leading-5'>
                    {t(
                      'Depends on the quota actually allocated to you by the organization.'
                    )}
                  </p>
                ) : null}
              </>
            ) : (
              <>
                <DetailRow label={t('Remaining quota')}>
                  {formatQuota(apiKey.remain_quota)}
                </DetailRow>
                <DetailRow label={t('Used Quota')}>
                  {formatQuota(apiKey.used_quota)}
                </DetailRow>
              </>
            )}
          </SideDrawerSection>

          <SideDrawerSection>
            <SideDrawerSectionHeader
              title={t('Time')}
              icon={<Clock className='size-4' />}
              iconTone='neutral'
            />
            <DetailRow label={t('Created')}>
              {formatTimestamp(apiKey.created_time)}
            </DetailRow>
            <DetailRow label={t('Last Used')}>
              {formatTimestamp(apiKey.accessed_time)}
            </DetailRow>
            <DetailRow label={t('Expires')}>
              {apiKey.expired_time === -1
                ? t('Never')
                : formatTimestamp(apiKey.expired_time)}
            </DetailRow>
          </SideDrawerSection>
        </div>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose
            render={<Button variant='outline' className='w-full sm:w-auto' />}
          >
            {t('Close')}
          </SheetClose>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
