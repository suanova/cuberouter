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

import { StaticDataTable } from '@/components/data-table'
import { cn } from '@/lib/utils'

import { formatVideoPrice, getOffPeakWindowLabel } from '../lib/video-price'
import type { OffPeakWindow, VideoPriceTable } from '../types'

export interface VideoPriceTableProps {
  table: VideoPriceTable
  /** Global off-peak window from the pricing response; note hidden when absent. */
  offPeakWindow?: OffPeakWindow
  className?: string
  tableClassName?: string
}

const headerCellClass =
  'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase'

/**
 * Per-second video price table for a model, showing the admin-configured
 * ¥/s values verbatim (no coefficients), with the global off-peak window
 * note beside the table. Renders nothing when the table has no rows.
 */
export function VideoPriceTable(props: VideoPriceTableProps) {
  const { t } = useTranslation()
  const rows = props.table?.rows ?? []
  if (rows.length === 0) return null

  const windowLabel = getOffPeakWindowLabel(props.offPeakWindow)

  return (
    <div
      className={cn(
        'flex flex-wrap items-start gap-x-4 gap-y-1.5',
        props.className
      )}
    >
      <StaticDataTable
        className='min-w-0 flex-1 overflow-hidden rounded-lg border'
        tableClassName={props.tableClassName ?? 'text-sm'}
        headerRowClassName='hover:bg-transparent'
        data={rows}
        getRowKey={(row, index) => row.resolution || `row-${index}`}
        columns={[
          {
            id: 'resolution',
            header: t('Resolution'),
            className: headerCellClass,
            cellClassName: 'py-2 font-medium',
            cell: (row) => row.resolution,
          },
          {
            id: 'normal',
            header: t('Video price (¥/s)'),
            className: `${headerCellClass} text-right`,
            cellClassName: 'py-2 text-right font-mono tabular-nums',
            cell: (row) => formatVideoPrice(row.normal_price),
          },
          {
            id: 'off-peak',
            header: t('Off-peak price (¥/s)'),
            className: `${headerCellClass} text-right`,
            cellClassName: 'py-2 text-right font-mono tabular-nums',
            cell: (row) => formatVideoPrice(row.off_peak_price),
          },
        ]}
      />
      {windowLabel && (
        <p className='text-muted-foreground/70 mt-1 max-w-56 text-[11px] leading-relaxed'>
          {t('Off-peak window: {{start}} - {{end}}', {
            start: windowLabel.start,
            end: windowLabel.crossesMidnight
              ? `${t('Next day')} ${windowLabel.end}`
              : windowLabel.end,
          })}
        </p>
      )}
    </div>
  )
}
