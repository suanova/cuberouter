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
import type { ColumnDef } from '@tanstack/react-table'

import { formatLogQuota, formatNumber } from '@/lib/format'

import { usageFailureValue, usageSuccessValue } from '../lib/usage-row'
import { usageTypeLabelKey } from '../lib/usage-type'
import type { SelfUsageRow, UsageTableVariant } from '../types'

function tokenText(value: number): string {
  return value > 0 ? formatNumber(value) : '-'
}

export function usageSuccessText(row: SelfUsageRow): string {
  return formatNumber(usageSuccessValue(row))
}

export function usageFailureText(row: SelfUsageRow): string {
  const value = usageFailureValue(row)
  return value > 0 ? formatNumber(value) : '-'
}

export function buildUsageColumns(
  t: (key: string) => string,
  variant: UsageTableVariant = 'personal'
): ColumnDef<SelfUsageRow>[] {
  const columns: ColumnDef<SelfUsageRow>[] = [
    {
      accessorKey: 'date',
      header: () => <span className='whitespace-nowrap'>{t('Date')}</span>,
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <span className='whitespace-nowrap tabular-nums'>
          {row.original.date}
        </span>
      ),
    },
  ]

  if (variant === 'organization') {
    columns.push({
      accessorKey: 'responsible_name',
      header: () => <span className='whitespace-nowrap'>{t('User')}</span>,
      enableSorting: false,
      size: 150,
      cell: ({ row }) => (
        <span className='block max-w-44 truncate text-sm'>
          {row.original.responsible_name || '-'}
        </span>
      ),
    })
  }

  columns.push({
    accessorKey: 'type',
    header: () => <span className='whitespace-nowrap'>{t('Type')}</span>,
    enableSorting: false,
    size: 90,
    cell: ({ row }) => (
      <span className='whitespace-nowrap'>
        {t(usageTypeLabelKey(row.original.type))}
      </span>
    ),
  })

  if (variant === 'personal') {
    columns.push({
      accessorKey: 'billing_account_type',
      header: () => (
        <span className='whitespace-nowrap'>{t('Billing Account')}</span>
      ),
      enableSorting: false,
      size: 150,
      cell: ({ row }) => (
        <span className='block max-w-44 truncate text-sm'>
          {row.original.billing_account_type === 'organization'
            ? row.original.organization_name || '-'
            : t('Personal')}
        </span>
      ),
    })
  }

  columns.push(
    {
      accessorKey: 'token_name',
      header: () => <span className='whitespace-nowrap'>{t('API Key')}</span>,
      enableSorting: false,
      size: 150,
      cell: ({ row }) => (
        <span className='block max-w-44 truncate font-mono text-xs'>
          {row.original.token_name || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'model_name',
      header: () => <span className='whitespace-nowrap'>{t('Model')}</span>,
      enableSorting: false,
      size: 180,
      cell: ({ row }) => (
        <span className='block max-w-44 truncate font-mono text-xs'>
          {row.original.model_name || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'request_count',
      header: () => <span className='whitespace-nowrap'>{t('Requests')}</span>,
      enableSorting: false,
      size: 100,
      cell: ({ row }) => (
        <span className='tabular-nums'>
          {formatNumber(row.original.request_count)}
        </span>
      ),
    },
    {
      accessorKey: 'success_count',
      header: () => <span className='whitespace-nowrap'>{t('Success')}</span>,
      enableSorting: false,
      size: 90,
      cell: ({ row }) => (
        <span className='tabular-nums'>{usageSuccessText(row.original)}</span>
      ),
    },
    {
      accessorKey: 'failure_count',
      header: () => <span className='whitespace-nowrap'>{t('Failed')}</span>,
      enableSorting: false,
      size: 90,
      cell: ({ row }) => (
        <span className='tabular-nums'>{usageFailureText(row.original)}</span>
      ),
    },
    {
      accessorKey: 'prompt_tokens',
      header: () => (
        <span className='whitespace-nowrap'>{t('Input Tokens')}</span>
      ),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <span className='tabular-nums'>
          {tokenText(row.original.prompt_tokens)}
        </span>
      ),
    },
    {
      accessorKey: 'completion_tokens',
      header: () => (
        <span className='whitespace-nowrap'>{t('Output Tokens')}</span>
      ),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <span className='tabular-nums'>
          {tokenText(row.original.completion_tokens)}
        </span>
      ),
    },
    {
      accessorKey: 'cache_tokens',
      header: () => (
        <span className='whitespace-nowrap'>{t('Cache Tokens')}</span>
      ),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <span className='tabular-nums'>
          {tokenText(row.original.cache_tokens)}
        </span>
      ),
    },
    {
      accessorKey: 'quota',
      header: () => <span className='whitespace-nowrap'>{t('Cost')}</span>,
      enableSorting: false,
      size: 120,
      cell: ({ row }) => (
        <span className='tabular-nums'>
          {formatLogQuota(row.original.quota)}
        </span>
      ),
    }
  )

  return columns
}
