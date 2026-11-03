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

import { resolveOverviewOrganizationId } from '../organization-selection'

describe('usage organization selection', () => {
  test('prefers the current context, falls back to the first org, then null', () => {
    expect(resolveOverviewOrganizationId([{ id: 1 }, { id: 2 }], 2)).toBe(2)
    expect(resolveOverviewOrganizationId([{ id: 1 }, { id: 2 }], 99)).toBe(1)
    expect(resolveOverviewOrganizationId([], null)).toBeNull()
  })
})
