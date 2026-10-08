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
  CHANNEL_TYPE_CUBE_STACK,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'

describe('CubeStack channel', () => {
  test('registers selection metadata and upstream model discovery', () => {
    const option = CHANNEL_TYPE_OPTIONS.find(
      (item) => item.value === CHANNEL_TYPE_CUBE_STACK
    )

    expect(option).toEqual({
      value: CHANNEL_TYPE_CUBE_STACK,
      label: 'CubeStack',
    })
    // MODEL_FETCHABLE_TYPES gates the "Fetch from Upstream" button in the
    // channel drawer (add and edit modes) and the row action on the channel
    // list; CubeStack (SGLang) exposes GET /v1/models.
    expect(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_CUBE_STACK)).toBe(true)
  })
})
