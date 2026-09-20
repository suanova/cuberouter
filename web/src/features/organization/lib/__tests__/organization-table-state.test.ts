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

import { organizationColumnFilterValue } from '../organization-table-state'

describe('organizationColumnFilterValue', () => {
  test('a chosen string is returned', () => {
    expect(
      organizationColumnFilterValue(
        [{ id: 'model_name', value: 'gpt-4o' }],
        'model_name'
      )
    ).toBe('gpt-4o')
  })

  test('a multi-select contributes its first entry', () => {
    expect(
      organizationColumnFilterValue(
        [{ id: 'group', value: ['prod', 'staging'] }],
        'group'
      )
    ).toBe('prod')
  })

  test('a cleared filter reads as absent, not as an empty string', () => {
    // The distinction matters: `?model=` and no `model` at all are the same
    // request, and a caller checking `=== undefined` has to see both that way.
    expect(
      organizationColumnFilterValue([{ id: 'model_name', value: '' }], 'model_name')
    ).toBeUndefined()
    expect(
      organizationColumnFilterValue([{ id: 'group', value: [] }], 'group')
    ).toBeUndefined()
    expect(
      organizationColumnFilterValue([{ id: 'group', value: [''] }], 'group')
    ).toBeUndefined()
  })

  test('a filter that was never set reads as absent', () => {
    expect(organizationColumnFilterValue([], 'model_name')).toBeUndefined()
    expect(
      organizationColumnFilterValue(
        [{ id: 'token_name', value: 'prod' }],
        'model_name'
      )
    ).toBeUndefined()
  })

  test('a non-string value reads as absent rather than being stringified', () => {
    expect(
      organizationColumnFilterValue([{ id: 'quota', value: 12 }], 'quota')
    ).toBeUndefined()
    expect(
      organizationColumnFilterValue([{ id: 'quota', value: null }], 'quota')
    ).toBeUndefined()
  })
})
