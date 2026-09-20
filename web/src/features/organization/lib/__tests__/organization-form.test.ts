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

import {
  isOrganizationSlugConfirmed,
  organizationFormSchema,
  transformOrganizationFormToPayload,
} from '../organization-form'

describe('organizationFormSchema', () => {
  test('accepts a name at the backend limit and rejects one over it', () => {
    // The update endpoint enforces 64 runes; enforcing it on create too keeps a
    // long name from becoming unsavable after the fact.
    expect(
      organizationFormSchema.safeParse({ name: 'a'.repeat(64) }).success
    ).toBe(true)
    expect(
      organizationFormSchema.safeParse({ name: 'a'.repeat(65) }).success
    ).toBe(false)
  })

  test('counts by code point, so emoji do not count double', () => {
    // jsdom/zod would otherwise measure UTF-16 code units, where each emoji is 2
    // and a 64-emoji name (64 runes, accepted by the backend) would be rejected.
    const name = '😀'.repeat(64)
    expect(name.length).toBe(128)
    expect(organizationFormSchema.safeParse({ name }).success).toBe(true)
    expect(
      organizationFormSchema.safeParse({ name: '😀'.repeat(65) }).success
    ).toBe(false)
  })

  test('trims the name and rejects a whitespace-only one', () => {
    const parsed = organizationFormSchema.parse({ name: '  Design Group  ' })
    expect(parsed.name).toBe('Design Group')
    expect(organizationFormSchema.safeParse({ name: '   ' }).success).toBe(false)
  })

  test('rejects a description longer than the column', () => {
    expect(
      organizationFormSchema.safeParse({
        name: 'Org',
        description: 'x'.repeat(512),
      }).success
    ).toBe(true)
    expect(
      organizationFormSchema.safeParse({
        name: 'Org',
        description: 'x'.repeat(513),
      }).success
    ).toBe(false)
  })
})

describe('transformOrganizationFormToPayload', () => {
  test('omits group and reason when creating', () => {
    // Create always starts on the default group; sending one would be ignored.
    const payload = transformOrganizationFormToPayload({
      name: 'Design',
      description: 'Design group',
      group: 'vip',
      reason: 'because',
    })
    expect(payload).toEqual({ name: 'Design', description: 'Design group' })
  })

  test('sends group and reason when updating with group rights', () => {
    const payload = transformOrganizationFormToPayload(
      {
        name: 'Design',
        description: 'Design group',
        group: 'vip',
        reason: 'moving tiers',
      },
      { organizationId: 7, includeGroup: true }
    )
    expect(payload).toEqual({
      name: 'Design',
      description: 'Design group',
      group: 'vip',
      reason: 'moving tiers',
    })
  })

  test('omits group without group rights and never sends an empty reason', () => {
    const payload = transformOrganizationFormToPayload(
      { name: 'Design', description: '', group: 'vip', reason: '  ' },
      { organizationId: 7, includeGroup: false }
    )
    expect(payload).toEqual({ name: 'Design', description: '' })
  })

  test('defaults a cleared group back to default rather than sending empty', () => {
    const payload = transformOrganizationFormToPayload(
      { name: 'Design', group: '   ' },
      { organizationId: 7, includeGroup: true }
    )
    expect(payload.group).toBe('default')
  })
})

describe('isOrganizationSlugConfirmed', () => {
  test('requires the exact organization slug', () => {
    expect(isOrganizationSlugConfirmed('metastone', 'metastone')).toBe(true)
    expect(isOrganizationSlugConfirmed('metastone', 'MetaStone')).toBe(false)
    expect(isOrganizationSlugConfirmed('metastone', 'metastone-x')).toBe(false)
    // Surrounding whitespace is trimmed on both sides, as the backend does.
    expect(isOrganizationSlugConfirmed('metastone', '  metastone  ')).toBe(true)
    expect(isOrganizationSlugConfirmed('  metastone  ', 'metastone')).toBe(true)
  })

  test('a missing slug never confirms, not even with an empty input', () => {
    expect(isOrganizationSlugConfirmed('', '')).toBe(false)
    expect(isOrganizationSlugConfirmed(undefined, '')).toBe(false)
    expect(isOrganizationSlugConfirmed('metastone', '')).toBe(false)
    expect(isOrganizationSlugConfirmed('metastone', '   ')).toBe(false)
  })
})
