/*
Copyright (C) 2023-2026 QuantumNous

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
import { expect, describe, test } from 'vitest'

import { buildExportPayload } from '../export-utils'

describe('buildExportPayload', () => {
  test('selected ids win over the current filter', () => {
    expect(buildExportPayload([1, 2], { keyword: 'alice', group: 'vip' })).toEqual({
      ids: [1, 2],
    })
  })

  test('falls back to the keyword/group filter', () => {
    expect(buildExportPayload([], { keyword: 'alice', group: 'vip' })).toEqual({
      keyword: 'alice',
      group: 'vip',
    })
  })

  test('omits empty filter fields (empty payload = export all)', () => {
    expect(buildExportPayload([], { keyword: '', group: '' })).toEqual({})
  })
})
