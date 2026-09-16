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

import { cn } from '@/lib/utils'

import { ASPECT_RATIO_ORDER, ASPECT_RATIOS } from '../constants'
import type { AspectRatio } from '../types'

interface RatioGridProps {
  value: AspectRatio
  onChange: (ratio: AspectRatio) => void
  disabled?: boolean
}

export function RatioGrid({
  value,
  onChange,
  disabled = false,
}: RatioGridProps) {
  const { t } = useTranslation()

  return (
    <div>
      <div
        role='radiogroup'
        aria-label={t('Image aspect ratio')}
        className='grid grid-cols-4 gap-1.5'
      >
        {ASPECT_RATIO_ORDER.map((ratio) => {
          const selected = value === ratio
          return (
            <button
              key={ratio}
              type='button'
              role='radio'
              aria-checked={selected}
              disabled={disabled}
              onClick={() => onChange(ratio)}
              className={cn(
                'rounded-md border px-1 py-1.5 text-xs font-medium transition-colors',
                'focus-visible:outline-2 focus-visible:outline-ring disabled:cursor-not-allowed disabled:opacity-50',
                selected
                  ? 'border-primary bg-primary/10 text-primary'
                  : 'border-border bg-background hover:bg-muted',
              )}
            >
              {ratio}
            </button>
          )
        })}
      </div>
      <p className='mt-1.5 text-[11px] text-muted-foreground'>
        {(() => {
          const [width, height] = ASPECT_RATIOS[value]
          return `${width} × ${height}`
        })()}
      </p>
    </div>
  )
}
