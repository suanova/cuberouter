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

import { normalizeDshOrganizationIds } from '../dsh-agent-access'

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
