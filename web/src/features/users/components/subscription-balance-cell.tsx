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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatCurrencyUSD, formatQuota } from '@/lib/format'

type SubscriptionBalanceCellProps = {
  // Plan-price-converted remaining value (user.subscription_remain_value, USD)
  remainValue: number
  remaining: number
  used: number
  total: number
  unlimited?: boolean
}

// Port from develop 2c55d2e, 9df4a57: shows the effective-subscription
// remaining value converted at the current plan price (USD), with the
// remaining percentage as a secondary value. '-' when the user has no
// active subscription, 'Unlimited' when any active subscription is
// amount_total<=0. The tooltip lists the raw subscription token breakdown.
export function SubscriptionBalanceCell(props: SubscriptionBalanceCellProps) {
  const { t } = useTranslation()
  const { remainValue, remaining, used, total } = props
  const unlimited = !!props.unlimited
  const hasActive = total > 0 || used > 0 || unlimited

  if (!hasActive) {
    return <span className='text-muted-foreground text-xs'>-</span>
  }

  if (unlimited) {
    return (
      <StatusBadge
        label={t('Unlimited')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  const percentage = total > 0 ? (remaining / total) * 100 : 0

  return (
    <Tooltip>
      <TooltipTrigger
        render={<div className='w-full min-w-0 cursor-help overflow-hidden' />}
      >
        <div className='flex min-w-0 items-center gap-1 whitespace-nowrap text-xs tabular-nums'>
          <span className='min-w-0 truncate font-medium'>
            {formatCurrencyUSD(remainValue)}
          </span>
          <span className='text-muted-foreground text-[10px]'>
            {percentage.toFixed(1)}%
          </span>
        </div>
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1 text-xs'>
          <div>
            {t('Remaining:')} {formatQuota(remaining)} (
            {percentage.toFixed(1)}%)
          </div>
          <div>
            {t('Used:')} {formatQuota(used)}
          </div>
          <div>
            {t('Total:')} {formatQuota(total)}
          </div>
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
