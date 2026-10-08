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
import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { splitAllowedPorts } from '../allowed-ports'

describe('SSRF allowed ports', () => {
  test('serializes as a JSON string array so the backend []string field can decode it', () => {
    assert.equal(
      JSON.stringify(splitAllowedPorts('80,443,30000')),
      '["80","443","30000"]'
    )
  })

  test('keeps port range tokens that the backend expands into a span', () => {
    assert.deepEqual(splitAllowedPorts('80, 8000-9000'), ['80', '8000-9000'])
  })

  test('drops the blank entries a trailing or doubled comma produces', () => {
    assert.deepEqual(splitAllowedPorts('80, ,443,'), ['80', '443'])
  })
})
