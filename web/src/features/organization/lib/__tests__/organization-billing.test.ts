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

import type {
  OrganizationBillingMonthlySummaryItem,
  OrganizationBillingRecord,
  OrganizationBillingUserSummaryItem,
} from '../../types'
import {
  buildOrganizationBillingCsv,
  csvCell,
  currentOrganizationBillingMonth,
  DEFAULT_ORGANIZATION_BILLING_MONTHS,
  DEFAULT_ORGANIZATION_BILLING_PANEL,
  findOrganizationBillingMonth,
  formatOrganizationBillingExportTime,
  isOrganizationBillingMonth,
  normalizeOrganizationBillingMonth,
  normalizeOrganizationBillingMonths,
  normalizeOrganizationBillingPanel,
  organizationBillingCsvFilename,
  organizationBillingDetailFilters,
  organizationBillingExportTimestamp,
  organizationBillingLedgerDelta,
  organizationBillingMonthRange,
  organizationBillingMonthlyOverviewStats,
  organizationBillingRecordTypeMeta,
  organizationBillingResponsibleName,
  organizationBillingUserOverviewStats,
  recentOrganizationBillingMonths,
} from '../organization-billing'

/** A local Date, so the expectations do not depend on the test timezone. */
const LOCAL_DATE = new Date(2026, 8, 20, 13, 5, 9)

function record(
  overrides: Partial<OrganizationBillingRecord> = {}
): OrganizationBillingRecord {
  return {
    id: 1,
    organization_id: 7,
    session_id: 0,
    record_key: 'k',
    quota_delta: 0,
    used_quota_delta: 0,
    usage_quota: 0,
    quota_before: 0,
    quota_after: 0,
    used_quota_before: 0,
    used_quota_after: 0,
    created_at: 1_700_000_000,
    ledger_quota_delta: 0,
    ...overrides,
  }
}

function userSummary(
  overrides: Partial<OrganizationBillingUserSummaryItem> = {}
): OrganizationBillingUserSummaryItem {
  return {
    responsible_user_id: 1,
    quota: 0,
    request_count: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    token_count: 0,
    ...overrides,
  }
}

function monthlySummary(
  overrides: Partial<OrganizationBillingMonthlySummaryItem> = {}
): OrganizationBillingMonthlySummaryItem {
  return {
    month: '2026-09',
    month_start: 0,
    month_end: 0,
    quota: 0,
    request_count: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    token_count: 0,
    responsible_user_count: 0,
    ...overrides,
  }
}

describe('panels', () => {
  test('a known panel name is kept', () => {
    expect(normalizeOrganizationBillingPanel('monthly')).toBe('monthly')
    expect(normalizeOrganizationBillingPanel('details')).toBe('details')
    expect(normalizeOrganizationBillingPanel('user')).toBe('user')
  })

  test('anything else selects the default panel', () => {
    expect(normalizeOrganizationBillingPanel(undefined)).toBe(
      DEFAULT_ORGANIZATION_BILLING_PANEL
    )
    expect(normalizeOrganizationBillingPanel('')).toBe(
      DEFAULT_ORGANIZATION_BILLING_PANEL
    )
    expect(normalizeOrganizationBillingPanel('usage')).toBe(
      DEFAULT_ORGANIZATION_BILLING_PANEL
    )
    expect(normalizeOrganizationBillingPanel(3)).toBe(
      DEFAULT_ORGANIZATION_BILLING_PANEL
    )
  })
})

describe('months', () => {
  test('only the backend spelling is a month', () => {
    expect(isOrganizationBillingMonth('2026-09')).toBe(true)
    expect(isOrganizationBillingMonth('2026-12')).toBe(true)
    // The endpoint parses with `2006-01`, so these are all refused by it too.
    expect(isOrganizationBillingMonth('2026-13')).toBe(false)
    expect(isOrganizationBillingMonth('2026-00')).toBe(false)
    expect(isOrganizationBillingMonth('2026-9')).toBe(false)
    expect(isOrganizationBillingMonth('26-09')).toBe(false)
    expect(isOrganizationBillingMonth('2026-09-01')).toBe(false)
    expect(isOrganizationBillingMonth('')).toBe(false)
    expect(isOrganizationBillingMonth(202609)).toBe(false)
    expect(isOrganizationBillingMonth(undefined)).toBe(false)
  })

  test('the current month is zero padded', () => {
    expect(currentOrganizationBillingMonth(new Date(2026, 0, 5))).toBe('2026-01')
    expect(currentOrganizationBillingMonth(new Date(2026, 11, 31))).toBe('2026-12')
  })

  test('the recent months are the current one and the eleven before it', () => {
    const months = recentOrganizationBillingMonths(
      DEFAULT_ORGANIZATION_BILLING_MONTHS,
      LOCAL_DATE
    )
    expect(months).toHaveLength(12)
    expect(months[0]).toBe('2026-09')
    expect(months[1]).toBe('2026-08')
    expect(months[11]).toBe('2025-10')
  })

  test('a month shorter than the one before it does not skip a month', () => {
    // 31 March minus 30 days would land back in March.
    const months = recentOrganizationBillingMonths(4, new Date(2026, 2, 31))
    expect(months).toEqual(['2026-03', '2026-02', '2026-01', '2025-12'])
  })

  test('a month from a link is kept only when it is one', () => {
    expect(normalizeOrganizationBillingMonth('2025-02', '2026-09')).toBe('2025-02')
    expect(normalizeOrganizationBillingMonth('2025-13', '2026-09')).toBe('2026-09')
    expect(normalizeOrganizationBillingMonth(undefined, '2026-09')).toBe('2026-09')
    expect(normalizeOrganizationBillingMonth('', '2026-09')).toBe('2026-09')
  })

  test('a range keeps the two months it was given', () => {
    expect(
      organizationBillingMonthRange('2026-01', '2026-09', LOCAL_DATE)
    ).toEqual({ start: '2026-01', end: '2026-09' })
  })

  test('the same month twice is a single-month range', () => {
    // The endpoint's end bound is the next month's start, so start < end holds.
    expect(
      organizationBillingMonthRange('2026-09', '2026-09', LOCAL_DATE)
    ).toEqual({ start: '2026-09', end: '2026-09' })
  })

  test('a range entered backwards is swapped rather than refused', () => {
    expect(
      organizationBillingMonthRange('2026-09', '2026-03', LOCAL_DATE)
    ).toEqual({ start: '2026-03', end: '2026-09' })
  })

  test('one bound is filled in from the other', () => {
    expect(
      organizationBillingMonthRange('2026-03', '', LOCAL_DATE)
    ).toEqual({ start: '2026-03', end: '2026-03' })
    expect(
      organizationBillingMonthRange('', '2026-03', LOCAL_DATE)
    ).toEqual({ start: '2026-03', end: '2026-03' })
  })

  test('an absent range is the current month', () => {
    expect(organizationBillingMonthRange('', '', LOCAL_DATE)).toEqual({
      start: '2026-09',
      end: '2026-09',
    })
    expect(organizationBillingMonthRange(undefined, 'nonsense', LOCAL_DATE)).toEqual(
      { start: '2026-09', end: '2026-09' }
    )
  })

  test('a month count is clamped to the range the endpoint answers for', () => {
    expect(normalizeOrganizationBillingMonths(6)).toBe(6)
    expect(normalizeOrganizationBillingMonths('24')).toBe(24)
    expect(normalizeOrganizationBillingMonths(36)).toBe(36)
    expect(normalizeOrganizationBillingMonths(120)).toBe(36)
    expect(normalizeOrganizationBillingMonths(0)).toBe(
      DEFAULT_ORGANIZATION_BILLING_MONTHS
    )
    expect(normalizeOrganizationBillingMonths(-3)).toBe(
      DEFAULT_ORGANIZATION_BILLING_MONTHS
    )
    expect(normalizeOrganizationBillingMonths('')).toBe(
      DEFAULT_ORGANIZATION_BILLING_MONTHS
    )
    expect(normalizeOrganizationBillingMonths('abc')).toBe(
      DEFAULT_ORGANIZATION_BILLING_MONTHS
    )
  })

  test('a month is found by name, or not at all', () => {
    const items = [monthlySummary({ month: '2026-09' }), monthlySummary({ month: '2026-08' })]
    expect(findOrganizationBillingMonth(items, '2026-08')?.month).toBe('2026-08')
    expect(findOrganizationBillingMonth(items, '2026-07')).toBeNull()
    expect(findOrganizationBillingMonth([], '2026-09')).toBeNull()
  })
})

describe('record types', () => {
  const translate = (key: string) => `t:${key}`

  test('each of the four ledger types has its own label and colour', () => {
    const seen = new Set<string>()
    for (const type of ['pre_consume', 'settle', 'refund', 'adjustment']) {
      const meta = organizationBillingRecordTypeMeta(type, translate)
      expect(meta.label).toMatch(/^t:/)
      expect(meta.label).not.toBe('t:')
      seen.add(meta.label)
      seen.add(meta.variant)
    }
    // Four labels and four colours, none shared.
    expect(seen.size).toBe(8)
  })

  test('an unknown type is shown as itself, not hidden', () => {
    expect(organizationBillingRecordTypeMeta('topup', translate)).toEqual({
      label: 'topup',
      variant: 'grey',
    })
  })

  test('a missing type reads as a dash', () => {
    expect(organizationBillingRecordTypeMeta(undefined, translate)).toEqual({
      label: '-',
      variant: 'grey',
    })
    expect(organizationBillingRecordTypeMeta('', translate)).toEqual({
      label: '-',
      variant: 'grey',
    })
  })
})

describe('records', () => {
  test('the ledger delta is the one the endpoint resolved', () => {
    expect(
      organizationBillingLedgerDelta(
        record({ ledger_quota_delta: -500, quota_delta: -500, usage_quota: 900 })
      )
    ).toBe(-500)
    // The other deltas are not a substitute: for a settlement the ledger delta
    // is the usage, and for a pre-consume it is the reservation.
    expect(
      organizationBillingLedgerDelta(
        record({ ledger_quota_delta: 900, used_quota_delta: 400, usage_quota: 900 })
      )
    ).toBe(900)
  })

  test('a missing ledger delta is zero rather than NaN', () => {
    const bare = record()
    delete (bare as { ledger_quota_delta?: number }).ledger_quota_delta
    expect(organizationBillingLedgerDelta(bare)).toBe(0)
  })

  test('the responsible name prefers the display name, then the username', () => {
    expect(
      organizationBillingResponsibleName({
        responsible_display_name: 'Ada Lovelace',
        responsible_username: 'ada',
        responsible_user_id: 4,
      })
    ).toBe('Ada Lovelace')
    expect(
      organizationBillingResponsibleName({
        responsible_display_name: '   ',
        responsible_username: 'ada',
        responsible_user_id: 4,
      })
    ).toBe('ada')
  })

  test('a deleted user still reads as an id, not as nobody', () => {
    expect(organizationBillingResponsibleName({ responsible_user_id: 12 })).toBe(
      '#12'
    )
    expect(organizationBillingResponsibleName({})).toBe('-')
  })
})

describe('per-member overview stats', () => {
  test('the totals are the sums over the members', () => {
    const stats = organizationBillingUserOverviewStats([
      userSummary({ responsible_user_id: 1, quota: 300, request_count: 2 }),
      userSummary({ responsible_user_id: 2, quota: 100, request_count: 5 }),
    ])
    expect(stats.totalQuota).toBe(400)
    expect(stats.totalRequests).toBe(7)
    expect(stats.activeUsers).toBe(2)
    expect(stats.topUser?.responsible_user_id).toBe(1)
  })

  test('a member with no cost and no traffic is not active', () => {
    const stats = organizationBillingUserOverviewStats([
      userSummary({ responsible_user_id: 1, quota: 0, request_count: 0 }),
      userSummary({ responsible_user_id: 2, quota: 0, request_count: 1 }),
    ])
    expect(stats.activeUsers).toBe(1)
  })

  test('a member who only holds keys is listed but not active', () => {
    const stats = organizationBillingUserOverviewStats([
      userSummary({ responsible_user_id: 9, token_count: 3 }),
    ])
    expect(stats.activeUsers).toBe(0)
    expect(stats.totalQuota).toBe(0)
    // Nobody spent anything, so there is no top spender to name; the panel
    // shows a dash rather than crediting a member with zero cost.
    expect(stats.topUser).toBeNull()
  })

  test('the top user is the one with the highest cost, not the most requests', () => {
    const stats = organizationBillingUserOverviewStats([
      userSummary({ responsible_user_id: 1, quota: 100, request_count: 99 }),
      userSummary({ responsible_user_id: 2, quota: 400, request_count: 1 }),
      userSummary({ responsible_user_id: 3, quota: 0, request_count: 500 }),
    ])
    expect(stats.topUser?.responsible_user_id).toBe(2)
  })

  test('an empty list has no top user', () => {
    const stats = organizationBillingUserOverviewStats([])
    expect(stats).toEqual({
      totalQuota: 0,
      totalRequests: 0,
      activeUsers: 0,
      topUser: null,
    })
  })
})

describe('monthly overview stats', () => {
  test('the average is over the months that have rows', () => {
    const stats = organizationBillingMonthlyOverviewStats(
      [
        monthlySummary({ month: '2026-09', quota: 200 }),
        monthlySummary({ month: '2026-08', quota: 100 }),
      ],
      '2026-09'
    )
    expect(stats.totalQuota).toBe(300)
    expect(stats.averageQuota).toBe(150)
    expect(stats.peak?.month).toBe('2026-09')
    expect(stats.current?.quota).toBe(200)
  })

  test('the current month is absent when it has no rows', () => {
    const stats = organizationBillingMonthlyOverviewStats(
      [monthlySummary({ month: '2026-08', quota: 100 })],
      '2026-09'
    )
    expect(stats.current).toBeNull()
    expect(stats.peak?.month).toBe('2026-08')
  })

  test('an empty list has no peak and no average', () => {
    expect(organizationBillingMonthlyOverviewStats([], '2026-09')).toEqual({
      totalQuota: 0,
      averageQuota: 0,
      peak: null,
      current: null,
    })
  })
})

describe('detail filters', () => {
  test('keywords are trimmed and blanks dropped', () => {
    expect(
      organizationBillingDetailFilters(
        {
          month: '2026-09',
          token_name: '  prod-key  ',
          model_name: 'gpt-4o',
          group: '   ',
          request_id: '',
        },
        { canViewWideData: false }
      )
    ).toEqual({
      month: '2026-09',
      token_name: 'prod-key',
      model_name: 'gpt-4o',
    })
  })

  test('a month that is not a month is dropped rather than sent', () => {
    expect(
      organizationBillingDetailFilters(
        { month: '2026-13' },
        { canViewWideData: false }
      )
    ).toEqual({})
  })

  test('the responsible filter needs wide-data capability', () => {
    const values = { responsible_name: 'ada' }
    expect(
      organizationBillingDetailFilters(values, { canViewWideData: false })
    ).toEqual({})
    expect(
      organizationBillingDetailFilters(values, { canViewWideData: true })
    ).toEqual({ responsible_name: 'ada' })
  })

  test('the month is not sent alongside a timestamp window', () => {
    // The endpoint ignores `month` once either bound is set, so the filter set
    // has no field for a bound — this pins that absence.
    expect(
      Object.keys(
        organizationBillingDetailFilters(
          { month: '2026-09', start_timestamp: 1, end_timestamp: 2 } as never,
          { canViewWideData: true }
        )
      )
    ).toEqual(['month'])
  })
})

describe('csv', () => {
  test('a plain cell is left alone', () => {
    expect(csvCell('prod-key')).toBe('prod-key')
    expect(csvCell(42)).toBe('42')
    expect(csvCell(-1.5)).toBe('-1.5')
  })

  test('an absent value is an empty cell, never the word null', () => {
    expect(csvCell(null)).toBe('')
    expect(csvCell(undefined)).toBe('')
    expect(csvCell('')).toBe('')
  })

  test('a cell that contains a separator or a quote is quoted', () => {
    expect(csvCell('a,b')).toBe('"a,b"')
    expect(csvCell('line\nbreak')).toBe('"line\nbreak"')
    expect(csvCell('carriage\rreturn')).toBe('"carriage\rreturn"')
  })

  test('a quote inside a quoted cell is doubled', () => {
    expect(csvCell('say "hi"')).toBe('"say ""hi"""')
    expect(csvCell('a,"b"')).toBe('"a,""b"""')
  })

  test('a document starts with a byte-order mark and a header', () => {
    const csv = buildOrganizationBillingCsv(
      [{ title: 'Month', value: (row: { month: string }) => row.month }],
      []
    )
    expect(csv).toBe('\uFEFFMonth')
    expect(csv.charCodeAt(0)).toBe(0xfeff)
  })

  test('rows follow the header in order', () => {
    const csv = buildOrganizationBillingCsv(
      [
        { title: 'Month', value: (row: { month: string }) => row.month },
        { title: 'Cost', value: (row: { cost: number }) => row.cost },
      ],
      [
        { month: '2026-09', cost: 2 },
        { month: '2026-08', cost: 1 },
      ]
    )
    expect(csv).toBe('\uFEFFMonth,Cost\n2026-09,2\n2026-08,1')
  })

  test('a title with a comma is escaped like any other cell', () => {
    const csv = buildOrganizationBillingCsv(
      [{ title: 'Cost, USD', value: () => 'x' }],
      []
    )
    expect(csv).toBe('\uFEFF"Cost, USD"')
  })

  test('an exported name containing a comma survives the round trip', () => {
    const csv = buildOrganizationBillingCsv(
      [{ title: 'User', value: (row: { name: string }) => row.name }],
      [{ name: 'Lovelace, Ada' }]
    )
    expect(csv).toBe('\uFEFFUser\n"Lovelace, Ada"')
  })
})

describe('export naming', () => {
  test('the timestamp is filesystem-safe and zero padded', () => {
    expect(organizationBillingExportTimestamp(LOCAL_DATE)).toBe(
      '2026-09-20-130509'
    )
    expect(
      organizationBillingExportTimestamp(new Date(2026, 0, 2, 3, 4, 5))
    ).toBe('2026-01-02-030405')
  })

  test('the filename names the panel it came from', () => {
    expect(organizationBillingCsvFilename('details', LOCAL_DATE)).toBe(
      'organization-billing-details-2026-09-20-130509.csv'
    )
    expect(organizationBillingCsvFilename('monthly-overview', LOCAL_DATE)).toBe(
      'organization-billing-monthly-overview-2026-09-20-130509.csv'
    )
  })

  test('an exported time is the local wall clock to the second', () => {
    expect(formatOrganizationBillingExportTime(LOCAL_DATE.getTime() / 1000)).toBe(
      '2026-09-20 13:05:09'
    )
  })

  test('an absent or unparsable time is blank, not 1970', () => {
    expect(formatOrganizationBillingExportTime(undefined)).toBe('')
    expect(formatOrganizationBillingExportTime(0)).toBe('')
    expect(formatOrganizationBillingExportTime(-1)).toBe('')
    expect(formatOrganizationBillingExportTime(Number.NaN)).toBe('')
  })
})
