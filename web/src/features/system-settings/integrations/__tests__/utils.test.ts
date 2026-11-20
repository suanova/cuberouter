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
import { describe, expect, test, vi } from 'vitest'

import { applyOptionUpdates, type OptionUpdate } from '../utils'

const updates: OptionUpdate[] = [
  { key: 'FirstOption', value: 'first' },
  { key: 'SecondOption', value: 2 },
  { key: 'ThirdOption', value: true },
]

describe('applyOptionUpdates', () => {
  test('stops at the first failed update and reports its key and message', async () => {
    const apply = vi.fn(async (update: OptionUpdate) =>
      update.key === 'SecondOption'
        ? { success: false, message: 'invalid value' }
        : { success: true }
    )

    const failure = await applyOptionUpdates(updates, apply)

    expect(failure).toEqual({
      key: 'SecondOption',
      message: 'invalid value',
    })
    expect(apply).toHaveBeenCalledTimes(2)
    expect(apply.mock.calls.map(([update]) => update.key)).toEqual([
      'FirstOption',
      'SecondOption',
    ])
  })

  test('returns null when every update succeeds', async () => {
    const apply = vi.fn(async () => ({ success: true }))

    await expect(applyOptionUpdates(updates, apply)).resolves.toBeNull()
    expect(apply).toHaveBeenCalledTimes(updates.length)
  })
})
