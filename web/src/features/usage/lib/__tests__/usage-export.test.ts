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
import { describe, expect, test } from 'vitest'

import { getCurrencyLabel } from '@/lib/currency'
import { quotaUnitsToDollars } from '@/lib/format'

import type {
  SelfUsageExportData,
  SelfUsageRow,
  UsageExportGranularity,
} from '../../types'
import { buildSelfUsageExportSheet } from '../usage-export'

function cellValue(cell: unknown): unknown {
  if (cell && typeof cell === 'object' && 'value' in cell) {
    return (cell as { value?: unknown }).value
  }
  return cell
}

function row(overrides: Partial<SelfUsageRow> = {}): SelfUsageRow {
  return {
    date: '2026-11-03',
    type: 'text',
    billing_account_type: 'personal',
    organization_id: 0,
    organization_name: '',
    token_id: 1,
    token_name: 'key-1',
    model_name: 'gpt-4o',
    request_count: 3,
    success_count: 3,
    failure_count: 0,
    prompt_tokens: 1000,
    completion_tokens: 500,
    cache_tokens: 300,
    quota: 800_000,
    ...overrides,
  }
}

function exportData(
  granularity: UsageExportGranularity,
  rows: SelfUsageRow[] = [row()]
): SelfUsageExportData {
  return { granularity, rows, total: rows.length, truncated: false }
}

function translate(key: string): string {
  return key
}

describe('buildSelfUsageExportSheet', () => {
  test('exports the table columns with a daily time label', () => {
    const currency = getCurrencyLabel()
    const sheet = buildSelfUsageExportSheet(exportData('day'), translate)

    expect(sheet.rows[0].map(cellValue)).toEqual([
      'Date',
      'Type',
      'Billing Account',
      'API Key',
      'Model',
      'Requests',
      'Success',
      'Failed',
      'Input Tokens',
      'Output Tokens',
      'Cache Tokens',
      `Cost (${currency})`,
    ])
    expect(sheet.rows[1].map(cellValue)).toEqual([
      '2026-11-03',
      'Text',
      'Personal',
      'key-1',
      'gpt-4o',
      3,
      3,
      0,
      1000,
      500,
      300,
      quotaUnitsToDollars(800_000),
    ])
    expect(sheet.filename).toMatch(/^usage-day-\d{4}-\d{2}-\d{2}-\d{6}\.xlsx$/)
  })

  test('exports hourly rows with the organization name and time header', () => {
    const currency = getCurrencyLabel()
    const sheet = buildSelfUsageExportSheet(
      exportData('hour', [
        row({
          date: '2026-11-03 10:00',
          type: 'video',
          billing_account_type: 'organization',
          organization_name: 'Acme',
          token_name: 'org-key',
          model_name: 'wan3.0-video',
          request_count: 1,
          success_count: 1,
          prompt_tokens: 0,
          completion_tokens: 0,
          cache_tokens: 0,
          quota: 120_000,
        }),
      ]),
      translate
    )

    expect(sheet.rows[0].map(cellValue)[0]).toBe('Time')
    expect(sheet.rows[1].map(cellValue)).toEqual([
      '2026-11-03 10:00',
      'Video',
      'Acme',
      'org-key',
      'wan3.0-video',
      1,
      1,
      0,
      0,
      0,
      0,
      quotaUnitsToDollars(120_000),
    ])
    expect(sheet.filename).toMatch(/^usage-hour-\d{4}-\d{2}-\d{2}-\d{6}\.xlsx$/)
    expect(sheet.rows[0].map(cellValue)[11]).toBe(`Cost (${currency})`)
  })

  test('exports organization rows with the responsible member and no billing account', () => {
    const currency = getCurrencyLabel()
    const sheet = buildSelfUsageExportSheet(
      exportData('day', [
        row({
          billing_account_type: 'organization',
          organization_name: 'Acme',
          responsible_user_id: 101,
          responsible_name: 'Alice Chen',
        }),
      ]),
      translate,
      { variant: 'organization' }
    )

    expect(sheet.rows[0].map(cellValue)).toEqual([
      'Date',
      'User',
      'Type',
      'API Key',
      'Model',
      'Requests',
      'Success',
      'Failed',
      'Input Tokens',
      'Output Tokens',
      'Cache Tokens',
      `Cost (${currency})`,
    ])
    expect(sheet.rows[1].map(cellValue)).toEqual([
      '2026-11-03',
      'Alice Chen',
      'Text',
      'key-1',
      'gpt-4o',
      3,
      3,
      0,
      1000,
      500,
      300,
      quotaUnitsToDollars(800_000),
    ])
    expect(sheet.filename).toMatch(
      /^organization-usage-day-\d{4}-\d{2}-\d{2}-\d{6}\.xlsx$/
    )
  })

  test('exports the same success and failure numbers the table shows', () => {
    const sheet = buildSelfUsageExportSheet(
      exportData('day', [
        row({ request_count: 2, success_count: 1, failure_count: 1 }),
        row({ request_count: 1, success_count: 0, failure_count: 0 }),
      ]),
      translate
    )

    // With failures the table reports success_count; without failures it
    // reports request_count, including a request still in progress.
    expect(sheet.rows[1].map(cellValue)[6]).toBe(1)
    expect(sheet.rows[1].map(cellValue)[7]).toBe(1)
    expect(sheet.rows[2].map(cellValue)[6]).toBe(1)
    expect(sheet.rows[2].map(cellValue)[7]).toBe(0)
  })
})
