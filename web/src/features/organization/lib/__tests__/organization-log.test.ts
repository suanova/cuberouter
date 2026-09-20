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

import type { OrganizationLogRow } from '../../types'
import {
  organizationLogModelInfo,
  organizationLogResponsibleName,
  organizationLogTimeRangeParams,
  organizationLogTimeWindow,
  organizationLogTokenName,
  organizationLogTypeMeta,
  ORGANIZATION_LOG_TYPE,
  ORGANIZATION_LOG_TYPE_FILTER_OPTIONS,
} from '../organization-log'

const FALLBACK = {
  start: new Date('2026-09-20T00:00:00.000Z'),
  end: new Date('2026-09-20T21:00:00.000Z'),
}

function row(overrides: Partial<OrganizationLogRow> = {}): OrganizationLogRow {
  return {
    id: 1,
    user_id: 7,
    created_at: 1_700_000_000,
    type: 2,
    quota: 0,
    prompt_tokens: 0,
    completion_tokens: 0,
    use_time: 0,
    is_stream: false,
    ...overrides,
  }
}

describe('organizationLogTypeMeta', () => {
  test('every type the backend can store has its own label', () => {
    const labels = Object.values(ORGANIZATION_LOG_TYPE).map(
      (type) => organizationLogTypeMeta(type).labelKey
    )

    expect(new Set(labels).size).toBe(labels.length)
    expect(organizationLogTypeMeta(ORGANIZATION_LOG_TYPE.CONSUME).labelKey).toBe(
      'Consume'
    )
  })

  test('an absent or unknown type falls back to one label, not a crash', () => {
    expect(organizationLogTypeMeta(undefined)).toEqual(
      organizationLogTypeMeta(0)
    )
    expect(organizationLogTypeMeta(99)).toEqual(organizationLogTypeMeta(0))
  })

  test('the filter options cover every type, in backend order', () => {
    expect(ORGANIZATION_LOG_TYPE_FILTER_OPTIONS.map((o) => o.value)).toEqual([
      '1',
      '2',
      '3',
      '4',
      '5',
      '6',
      '7',
    ])
    // No "all" entry: an empty filter is how the UI says every type.
    expect(
      ORGANIZATION_LOG_TYPE_FILTER_OPTIONS.some((o) => o.value === '0')
    ).toBe(false)
  })
})

describe('organizationLogTimeWindow', () => {
  test('an empty search opens on the default window', () => {
    const window = organizationLogTimeWindow({}, FALLBACK)

    expect(window).toEqual({ ...FALLBACK, fromUrl: false })
  })

  test('both bounds spelled out in the URL are used as given', () => {
    const start = new Date('2026-09-01T08:00:00.000Z').getTime()
    const end = new Date('2026-09-02T08:00:00.000Z').getTime()

    expect(organizationLogTimeWindow({ startTime: start, endTime: end }, FALLBACK)).toEqual(
      { start: new Date(start), end: new Date(end), fromUrl: true }
    )
  })

  test('a half-written link gets the default for the bound it omits', () => {
    const start = new Date('2026-09-01T08:00:00.000Z').getTime()
    const window = organizationLogTimeWindow({ startTime: start }, FALLBACK)

    expect(window.start).toEqual(new Date(start))
    expect(window.end).toEqual(FALLBACK.end)
    expect(window.fromUrl).toBe(true)
  })

  test('a bound the URL cannot parse reads as absent, not as 1970', () => {
    const window = organizationLogTimeWindow(
      { startTime: Number.NaN, endTime: Number.POSITIVE_INFINITY },
      FALLBACK
    )

    expect(window.start).toEqual(FALLBACK.start)
    expect(window.end).toEqual(FALLBACK.end)
    expect(window.fromUrl).toBe(false)
  })
})

describe('organizationLogTimeRangeParams', () => {
  test('the window is sent as whole seconds', () => {
    expect(
      organizationLogTimeRangeParams(
        new Date('2026-09-20T10:00:00.750Z'),
        new Date('2026-09-20T11:30:59.999Z')
      )
    ).toEqual({
      start_timestamp: 1_789_898_400,
      end_timestamp: 1_789_903_859,
    })
  })

  test('a missing bound is left out entirely', () => {
    expect(organizationLogTimeRangeParams(undefined, undefined)).toEqual({})
    expect(
      organizationLogTimeRangeParams(new Date('2026-09-20T10:00:00.000Z'))
    ).toEqual({ start_timestamp: 1_789_898_400 })
  })

  test('an invalid date is dropped rather than sent as zero', () => {
    expect(
      organizationLogTimeRangeParams(new Date('nonsense'), new Date('also bad'))
    ).toEqual({})
  })
})

describe('organizationLogModelInfo', () => {
  test('a plain row reports just its model', () => {
    expect(organizationLogModelInfo({ model_name: 'gpt-4o' })).toEqual({
      name: 'gpt-4o',
      actualModel: undefined,
    })
  })

  test('a mapped row also reports the upstream model', () => {
    const info = organizationLogModelInfo({
      model_name: 'gpt-4o',
      other: JSON.stringify({
        is_model_mapped: true,
        upstream_model_name: 'gpt-4o-2024-11-20',
      }),
    })

    expect(info).toEqual({
      name: 'gpt-4o',
      actualModel: 'gpt-4o-2024-11-20',
    })
  })

  test('a flag without a replacement name is not a mapping', () => {
    expect(
      organizationLogModelInfo({
        model_name: 'gpt-4o',
        other: JSON.stringify({ is_model_mapped: true }),
      }).actualModel
    ).toBeUndefined()
  })

  test('an unparsable other field still leaves the model readable', () => {
    expect(
      organizationLogModelInfo({ model_name: 'gpt-4o', other: '{not json' })
    ).toEqual({ name: 'gpt-4o', actualModel: undefined })
  })

  test('a row with no model reads as empty, not as undefined text', () => {
    expect(organizationLogModelInfo({}).name).toBe('')
  })
})

describe('organizationLogResponsibleName', () => {
  test('the display name wins over the username', () => {
    expect(
      organizationLogResponsibleName(
        row({ responsible_display_name: 'Ada', responsible_username: 'ada' })
      )
    ).toBe('Ada')
  })

  test('the snapshot is used when the account is gone', () => {
    expect(
      organizationLogResponsibleName(
        row({ responsible_name: 'former member', responsible_user_id: 42 })
      )
    ).toBe('former member')
  })

  test('an id with no name anywhere still names the row', () => {
    expect(organizationLogResponsibleName(row({ responsible_user_id: 42 }))).toBe(
      '#42'
    )
  })

  test('a row with no holder at all reads as a dash', () => {
    expect(organizationLogResponsibleName(row({}))).toBe('-')
  })
})

describe('organizationLogTokenName', () => {
  test('the stored key name is used as-is', () => {
    expect(organizationLogTokenName(row({ token_name: 'ci' }))).toBe('ci')
  })

  test('a deleted key falls back to its id', () => {
    expect(organizationLogTokenName(row({ token_id: 12 }))).toBe('#12')
  })

  test('a row with neither reads as a dash', () => {
    expect(organizationLogTokenName(row({}))).toBe('-')
  })
})
