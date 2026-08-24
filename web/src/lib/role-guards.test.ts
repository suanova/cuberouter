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
import { describe, expect, it } from 'vitest'

import { isAdmin, isOps } from './role-guards'

describe('role guards', () => {
  it('isOps accepts ops, admin and root', () => {
    expect(isOps(5)).toBe(true)
    expect(isOps(10)).toBe(true)
    expect(isOps(100)).toBe(true)
  })

  it('isOps rejects common and guest', () => {
    expect(isOps(1)).toBe(false)
    expect(isOps(0)).toBe(false)
    expect(isOps(undefined)).toBe(false)
  })

  it('isAdmin accepts admin and root only', () => {
    expect(isAdmin(10)).toBe(true)
    expect(isAdmin(100)).toBe(true)
    expect(isAdmin(5)).toBe(false)
    expect(isAdmin(1)).toBe(false)
  })
})
