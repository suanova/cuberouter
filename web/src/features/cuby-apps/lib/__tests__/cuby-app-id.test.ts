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

import { isValidCubyAppId } from '../cuby-app-id'

describe('cuby app id', () => {
  test.each(['ai-company', 'financebuddy', 'a1', '9lives', 'a'.repeat(40)])(
    'accepts %s, which Cuby accepts too',
    (appId) => {
      expect(isValidCubyAppId(appId)).toBe(true)
    }
  )

  test.each([
    ['', 'empty'],
    ['a', 'one character'],
    ['-ai', 'a leading hyphen'],
    ['AI-Company', 'upper case'],
    ['ai_company', 'an underscore'],
    ['ai company', 'a space inside'],
    ['a'.repeat(41), 'more than 40 characters'],
  ])('refuses %j: %s', (appId) => {
    expect(isValidCubyAppId(appId)).toBe(false)
  })

  test('ignores the spaces around a pasted id', () => {
    expect(isValidCubyAppId('  ai-company  ')).toBe(true)
  })
})
