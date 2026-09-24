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
import { useTranslation } from 'react-i18next'

import { StatusBadge } from '@/components/status-badge'
import { Progress } from '@/components/ui/progress'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatQuota } from '@/lib/format'
import { cn } from '@/lib/utils'

type UserQuotaCellProps = {
  used: number
  remaining: number
  total: number
  unlimited?: boolean
}

function getQuotaProgressColor(percentage: number): string {
  if (percentage <= 10) return '[&_[data-slot=progress-indicator]]:bg-rose-500'
  if (percentage <= 30) return '[&_[data-slot=progress-indicator]]:bg-amber-500'
  return '[&_[data-slot=progress-indicator]]:bg-emerald-500'
}

// Port from develop 51b3f79/#86, 2c55d2e, 9df4a57: the cell now renders the
// effective-subscription token quota basis (remain/total/unlimited) instead
// of the wallet quota. `total` must be the subscription total (remain is
// clamped per subscription, so used + remaining is not the total).
export function UserQuotaCell(props: UserQuotaCellProps) {
  const { t } = useTranslation()
  const { used, remaining, total, unlimited } = props
  const percentage = total > 0 ? (remaining / total) * 100 : 0
  const formattedRemaining = formatQuota(remaining)
  const formattedTotal = formatQuota(total)

  if (!unlimited && total === 0 && used === 0) {
    return (
      <StatusBadge
        label={t('No Quota')}
        variant='neutral'
        copyable={false}
        className='-ml-1.5'
      />
    )
  }

  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <div className='w-full min-w-0 cursor-help space-y-1.5 overflow-hidden' />
        }
      >
        <div className='grid min-w-0 grid-cols-2 gap-x-4 text-xs'>
          <span className='min-w-0 truncate font-medium tabular-nums'>
            {unlimited ? t('Unlimited') : formattedRemaining}
          </span>
          <span className='text-muted-foreground min-w-0 truncate text-right tabular-nums'>
            {formattedTotal}
          </span>
        </div>
        <Progress
          value={percentage}
          className={cn('h-1.5', getQuotaProgressColor(percentage))}
        />
      </TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1 text-xs'>
          <div>
            {t('Used:')} {formatQuota(used)}
          </div>
          <div>
            {t('Remaining:')} {unlimited ? t('Unlimited') : formattedRemaining}
          </div>
          <div>
            {t('Total:')} {formattedTotal}
          </div>
          <div>
            {t('Percentage:')} {percentage.toFixed(1)}%
          </div>
        </div>
      </TooltipContent>
    </Tooltip>
  )
}
