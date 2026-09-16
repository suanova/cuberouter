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
import { useBillingCurrency } from '@/lib/currency'
import { cn } from '@/lib/utils'

import { formatVideoPriceMoney } from '../lib/video-price'
import type { ImagePriceTable } from '../types'

export interface ImagePriceTableProps {
  table: ImagePriceTable
  className?: string
  tableClassName?: string
}

const headerCellClass =
  'text-muted-foreground py-2 text-[10px] font-medium tracking-wider uppercase'

/**
 * Per-image price table for a model, showing the stored USD/image values
 * converted to the site display currency (symbol in the header). Renders
 * nothing when the table has no rows.
 */
export function ImagePriceTable(props: ImagePriceTableProps) {
  const { t } = useTranslation()
  const symbol = useBillingCurrency().symbol
  const rows = props.table?.rows ?? []
  if (rows.length === 0) return null

  return (
    <StaticDataTable
      className={cn(
        'min-w-0 overflow-hidden rounded-lg border',
        props.className
      )}
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
          id: 'price',
          header: t('Image price ({{symbol}}/image)', { symbol }),
          className: `${headerCellClass} text-right`,
          cellClassName: 'py-2 text-right font-mono tabular-nums',
          cell: (row) =>
            formatVideoPriceMoney(row.price, { showSymbol: false }),
        },
      ]}
    />
  )
}
