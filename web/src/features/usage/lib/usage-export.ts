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
import { getCurrencyLabel } from '@/lib/currency'
import dayjs from '@/lib/dayjs'
import { writeExcelSheet, type ExcelSheet } from '@/lib/excel'
import { quotaUnitsToDollars } from '@/lib/format'

import type { SelfUsageExportData } from '../types'
import { usageTypeLabelKey } from './usage-type'

const TOKEN_FORMAT = '#,##0'
const MONEY_FORMAT = '#,##0.0000'

function tokenCell(value: number) {
  return { value, type: Number, format: TOKEN_FORMAT }
}

function moneyCell(value: number) {
  return { value, type: Number, format: MONEY_FORMAT }
}

function headerCell(value: string) {
  return { value, fontWeight: 'bold' as const }
}

/**
 * The spreadsheet mirrors the usage table. The backend has already aggregated
 * the rows at the requested bucket size, so this layer only formats columns.
 */
export function buildSelfUsageExportSheet(
  data: SelfUsageExportData,
  t: (key: string) => string
): ExcelSheet {
  const currency = getCurrencyLabel()
  const costHeader = `${t('Cost')} (${currency})`
  const timeHeader = data.granularity === 'hour' ? t('Time') : t('Date')

  return {
    rows: [
      [
        headerCell(timeHeader),
        headerCell(t('Type')),
        headerCell(t('Billing Account')),
        headerCell(t('API Key')),
        headerCell(t('Model')),
        headerCell(t('Requests')),
        headerCell(t('Success')),
        headerCell(t('Failed')),
        headerCell(t('Input Tokens')),
        headerCell(t('Output Tokens')),
        headerCell(t('Cache Tokens')),
        headerCell(costHeader),
      ],
      ...data.rows.map((row) => [
        row.date,
        t(usageTypeLabelKey(row.type)),
        row.billing_account_type === 'organization'
          ? row.organization_name || '-'
          : t('Personal'),
        row.token_name || '-',
        row.model_name || '-',
        tokenCell(row.request_count),
        tokenCell(row.success_count),
        tokenCell(row.failure_count),
        tokenCell(row.prompt_tokens),
        tokenCell(row.completion_tokens),
        tokenCell(row.cache_tokens),
        moneyCell(quotaUnitsToDollars(row.quota)),
      ]),
    ],
    columns: [
      { width: 18 },
      { width: 10 },
      { width: 18 },
      { width: 20 },
      { width: 22 },
      { width: 10 },
      { width: 10 },
      { width: 10 },
      { width: 14 },
      { width: 14 },
      { width: 16 },
      { width: 18 },
    ],
    filename: `usage-${data.granularity}-${dayjs().format('YYYY-MM-DD-HHmmss')}.xlsx`,
  }
}

/** Writes the sheet through the browser build, which triggers a download. */
export async function writeSelfUsageExportFile(
  sheet: ExcelSheet
): Promise<void> {
  await writeExcelSheet(sheet)
}
