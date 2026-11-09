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
import { describe, expect, it } from 'vitest'

import {
  buildDshChannelChoices,
  mergeDshOrganizationSelection,
  normalizeDshOrganizationIds,
} from '../dsh-agent-access'

describe('normalizeDshOrganizationIds', () => {
  it('keeps positive integer IDs from numbers and numeric strings', () => {
    expect(normalizeDshOrganizationIds([3, '1', '2'])).toEqual([1, 2, 3])
  })

  it('drops zero, negative, fractional and non-numeric values', () => {
    expect(normalizeDshOrganizationIds([0, -1, 1.5, 'abc', '', null])).toEqual(
      []
    )
  })

  it('deduplicates IDs so the saved JSON array is stable', () => {
    expect(JSON.stringify(normalizeDshOrganizationIds(['2', 2, 1]))).toBe(
      '[1,2]'
    )
  })
})

describe('mergeDshOrganizationSelection', () => {
  it('appends a newly bound organization without disturbing the others', () => {
    expect(mergeDshOrganizationSelection(['3', '1'], 12)).toEqual(['3', '1', '12'])
  })

  it('keeps the selection unchanged for an organization that is already selected', () => {
    expect(mergeDshOrganizationSelection(['12'], 12)).toEqual(['12'])
  })

  it('ignores invalid organization IDs', () => {
    expect(mergeDshOrganizationSelection(['1'], 0)).toEqual(['1'])
    expect(mergeDshOrganizationSelection(['1'], 1.5)).toEqual(['1'])
  })
})

describe('buildDshChannelChoices', () => {
  const channels = [
    { id: 34, name: 'org-12', enabled: true, boundOrganizationId: 0 },
    { id: 35, name: 'stopped', enabled: false, boundOrganizationId: 0 },
    { id: 36, name: 'other', enabled: true, boundOrganizationId: 13 },
  ]

  it('always offers "unbound" first and lists every DSH channel', () => {
    const choices = buildDshChannelChoices(channels, 12, 0)

    expect(choices.map((choice) => choice.id)).toEqual([0, 34, 35, 36])
    expect(choices[0]).toMatchObject({ disabled: false, reason: null })
  })

  it('disables stopped channels and channels owned by another organization', () => {
    const choices = buildDshChannelChoices(channels, 12, 0)

    expect(choices[2]).toMatchObject({ id: 35, disabled: true, reason: 'disabled' })
    expect(choices[3]).toMatchObject({
      id: 36,
      disabled: true,
      reason: 'bound_elsewhere',
    })
  })

  it('keeps the current organization channel selectable even when it is stopped', () => {
    const choices = buildDshChannelChoices(channels, 12, 35)

    expect(choices[2]).toMatchObject({ id: 35, disabled: false, reason: null })
  })

  it('surfaces a deleted binding as a missing placeholder', () => {
    const choices = buildDshChannelChoices(channels, 12, 99)

    expect(choices[1]).toEqual({
      id: 99,
      name: '',
      disabled: false,
      reason: 'missing',
    })
  })
})
