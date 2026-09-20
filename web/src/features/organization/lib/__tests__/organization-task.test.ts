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
import { describe, expect, test } from 'vitest'

import {
  TASK_ACTION_MAPPINGS,
  TASK_STATUS_MAPPINGS,
} from '@/features/usage-logs/constants'

import type {
  OrganizationMidjourneyTaskRow,
  OrganizationTaskRow,
} from '../../types'
import {
  DEFAULT_ORGANIZATION_TASK_PANEL,
  insertColumnsAfter,
  normalizeOrganizationTaskPanel,
  organizationMidjourneyTaskToLog,
  organizationTaskResponsibleName,
  organizationTaskTimeRangeParams,
  organizationTaskToLog,
  organizationTaskTokenName,
  ORGANIZATION_TASK_ACTION_OPTIONS,
  ORGANIZATION_TASK_PANELS,
  ORGANIZATION_TASK_STATUS_OPTIONS,
} from '../organization-task'

function taskRow(
  overrides: Partial<OrganizationTaskRow> = {}
): OrganizationTaskRow {
  return {
    id: 1,
    created_at: 1_700_000_000,
    updated_at: 1_700_000_100,
    task_id: 'task-1',
    platform: 'kling',
    user_id: 7,
    quota: 0,
    status: 'SUCCESS',
    ...overrides,
  }
}

function midjourneyRow(
  overrides: Partial<OrganizationMidjourneyTaskRow> = {}
): OrganizationMidjourneyTaskRow {
  return {
    id: 1,
    user_id: 7,
    quota: 0,
    ...overrides,
  }
}

describe('normalizeOrganizationTaskPanel', () => {
  test('both panels are recognised', () => {
    for (const panel of ORGANIZATION_TASK_PANELS) {
      expect(normalizeOrganizationTaskPanel(panel)).toBe(panel)
    }
  })

  test('anything else opens the default panel rather than nothing', () => {
    expect(normalizeOrganizationTaskPanel('drawings')).toBe(
      DEFAULT_ORGANIZATION_TASK_PANEL
    )
    expect(normalizeOrganizationTaskPanel(undefined)).toBe(
      DEFAULT_ORGANIZATION_TASK_PANEL
    )
    expect(normalizeOrganizationTaskPanel(7)).toBe(
      DEFAULT_ORGANIZATION_TASK_PANEL
    )
  })
})

describe('organizationTaskToLog', () => {
  test('the fields the row omits are filled for the shared columns', () => {
    const log = organizationTaskToLog(taskRow())

    expect(log.action).toBe('')
    expect(log.group).toBe('')
    expect(log.channel_id).toBe(0)
    expect(log.submit_time).toBe(0)
  })

  test('values the row carries are kept', () => {
    const log = organizationTaskToLog(
      taskRow({
        action: 'GENERATE',
        group: 'vip',
        channel_id: 3,
        submit_time: 99,
      })
    )

    expect(log).toMatchObject({
      action: 'GENERATE',
      group: 'vip',
      channel_id: 3,
      submit_time: 99,
    })
  })

  test('the task itself survives the adapter untouched', () => {
    const log = organizationTaskToLog(
      taskRow({ task_id: 'task-9', platform: 'suno', quota: 500 })
    )

    expect(log).toMatchObject({
      task_id: 'task-9',
      platform: 'suno',
      quota: 500,
      status: 'SUCCESS',
    })
  })
})

describe('organizationMidjourneyTaskToLog', () => {
  test('every field the drawing columns read is filled', () => {
    const log = organizationMidjourneyTaskToLog(midjourneyRow())

    expect(log).toMatchObject({
      code: 0,
      mj_id: '',
      action: '',
      channel_id: 0,
      submit_time: 0,
      progress: '',
      prompt: '',
      status: '',
    })
  })

  test('a populated row is passed through', () => {
    const log = organizationMidjourneyTaskToLog(
      midjourneyRow({
        code: 1,
        mj_id: 'mj-1',
        action: 'IMAGINE',
        channel_id: 4,
        submit_time: 1_700_000_000_000,
        progress: '100%',
        prompt: 'a cat',
        status: 'SUCCESS',
      })
    )

    expect(log).toMatchObject({
      code: 1,
      mj_id: 'mj-1',
      action: 'IMAGINE',
      channel_id: 4,
      // Milliseconds, as the drawing table reads them.
      submit_time: 1_700_000_000_000,
      prompt: 'a cat',
    })
  })
})

describe('organizationTaskResponsibleName', () => {
  test('the responsible snapshot wins over the creator snapshot', () => {
    expect(
      organizationTaskResponsibleName({
        responsible_name: 'Ada',
        creator_name: 'Grace',
      })
    ).toBe('Ada')
  })

  test('a row with only a creator falls back to it', () => {
    expect(organizationTaskResponsibleName({ creator_name: 'Grace' })).toBe(
      'Grace'
    )
  })

  test('an id stands in when neither name was stored', () => {
    expect(organizationTaskResponsibleName({ responsible_user_id: 42 })).toBe(
      '#42'
    )
    expect(organizationTaskResponsibleName({ creator_user_id: 42 })).toBe('#42')
    expect(organizationTaskResponsibleName({ user_id: 42 })).toBe('#42')
  })

  test('a row with nobody attached reads as a dash', () => {
    expect(organizationTaskResponsibleName({})).toBe('-')
  })

  test('username is never consulted, because the endpoint never fills it', () => {
    // `model.Task.Username` is `gorm:"-"`: the personal dashboard writes it
    // while reading, and the organization endpoint returns the rows as found,
    // so on an organization row it is empty however the type reads.
    expect(
      organizationTaskResponsibleName({
        username: 'ghost',
        responsible_user_id: 42,
      } as { username: string; responsible_user_id: number })
    ).toBe('#42')
  })
})

describe('organizationTaskTokenName', () => {
  test('the stored key name is used as-is', () => {
    expect(organizationTaskTokenName({ token_name: 'ci' })).toBe('ci')
  })

  test('a deleted key falls back to its id', () => {
    expect(organizationTaskTokenName({ token_id: 12 })).toBe('#12')
  })

  test('a row with neither reads as a dash', () => {
    expect(organizationTaskTokenName({})).toBe('-')
  })
})

describe('task filter options', () => {
  test('the statuses are the ones the table can badge', () => {
    expect(ORGANIZATION_TASK_STATUS_OPTIONS.map((o) => o.value).sort()).toEqual(
      Object.keys(TASK_STATUS_MAPPINGS).sort()
    )
    expect(
      ORGANIZATION_TASK_STATUS_OPTIONS.find((o) => o.value === 'SUCCESS')?.label
    ).toBe('Success')
  })

  test('the actions are the ones the table can badge', () => {
    expect(ORGANIZATION_TASK_ACTION_OPTIONS.map((o) => o.value).sort()).toEqual(
      Object.keys(TASK_ACTION_MAPPINGS).sort()
    )
  })

  test('no option is offered twice', () => {
    const values = ORGANIZATION_TASK_STATUS_OPTIONS.map((o) => o.value)
    expect(new Set(values).size).toBe(values.length)
  })
})

describe('organizationTaskTimeRangeParams', () => {
  const start = new Date('2026-09-20T10:00:00.750Z')
  const end = new Date('2026-09-20T11:30:59.999Z')

  test('the async task table is sent whole seconds', () => {
    expect(organizationTaskTimeRangeParams(start, end, 'seconds')).toEqual({
      start_timestamp: 1_789_898_400,
      end_timestamp: 1_789_903_859,
    })
  })

  test('the Midjourney table is sent milliseconds', () => {
    expect(organizationTaskTimeRangeParams(start, end, 'milliseconds')).toEqual(
      {
        start_timestamp: start.getTime(),
        end_timestamp: end.getTime(),
      }
    )
  })

  test('a missing bound is left out rather than sent as zero', () => {
    // Zero is what the endpoint reads as "no bound", but the async service
    // skips the clause on `> 0` alone — so sending it would be indistinguishable
    // from not sending it, and omitting it says what is meant.
    expect(
      organizationTaskTimeRangeParams(undefined, undefined, 'seconds')
    ).toEqual({})
    expect(
      organizationTaskTimeRangeParams(start, undefined, 'seconds')
    ).toEqual({ start_timestamp: 1_789_898_400 })
  })

  test('an invalid date is dropped rather than sent as NaN', () => {
    expect(
      organizationTaskTimeRangeParams(
        new Date('nonsense'),
        new Date('also nonsense'),
        'milliseconds'
      )
    ).toEqual({})
  })
})

/**
 * How TanStack identifies a column: the id it declares, or the accessor key it
 * reads, which is the id it derives when none was declared. A group column has
 * only the first, which is why a union member has no `accessorKey` to read.
 */
function columnId<T>(column: ColumnDef<T>): string | undefined {
  const { id, accessorKey } = column as { id?: string; accessorKey?: string }
  return id ?? accessorKey
}

describe('insertColumnsAfter', () => {
  type Row = { task_id: string; status: string }

  const shared: ColumnDef<Row>[] = [
    { accessorKey: 'task_id', header: 'Task ID' },
    { id: 'status', header: 'Status' },
    { accessorKey: 'fail_reason', header: 'Details' },
  ]
  const extra: ColumnDef<Row>[] = [{ id: 'cost', header: 'Cost' }]

  test('the inserted columns land directly after the named column', () => {
    expect(
      insertColumnsAfter(shared, 'task_id', extra).map((column) =>
        columnId(column)
      )
    ).toEqual(['task_id', 'cost', 'status', 'fail_reason'])
  })

  test('a column named by its explicit id is found too', () => {
    expect(
      insertColumnsAfter(shared, 'status', extra).map((column) =>
        columnId(column)
      )
    ).toEqual(['task_id', 'status', 'cost', 'fail_reason'])
  })

  test('an unknown anchor appends rather than dropping the columns', () => {
    expect(
      insertColumnsAfter(shared, 'nowhere', extra).map((column) =>
        columnId(column)
      )
    ).toEqual(['task_id', 'status', 'fail_reason', 'cost'])
  })

  test('the input is not mutated', () => {
    const before = shared.map(columnId)
    insertColumnsAfter(shared, 'task_id', extra)

    expect(shared.map(columnId)).toEqual(before)
  })

  test('a wider row type is accepted for the inserted columns', () => {
    type Wide = Row & { token_name?: string }
    const wide: ColumnDef<Wide>[] = [
      {
        id: 'token_name',
        header: 'Key',
        cell: ({ row }) => row.original.token_name,
      },
    ]

    expect(
      insertColumnsAfter(shared, 'task_id', wide).map((column) =>
        columnId(column)
      )
    ).toEqual(['task_id', 'token_name', 'status', 'fail_reason'])
  })
})
