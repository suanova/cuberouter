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

import { taskActionMapper } from '../lib/mappers'

describe('task action labels', () => {
  test('maps backend canonical video actions to display labels', () => {
    const labels = {
      image_to_video: 'Image to Video',
      text_to_video: 'Text to Video',
      first_tail_to_video: 'First/Last Frame to Video',
      reference_to_video: 'Reference Video',
      remix: 'Video Remix',
    }

    for (const [action, label] of Object.entries(labels)) {
      assert.equal(taskActionMapper.getLabel(action), label)
    }
  })

  test('keeps legacy camelCase video actions mapped', () => {
    assert.equal(taskActionMapper.getLabel('generate'), 'Image to Video')
    assert.equal(taskActionMapper.getLabel('textGenerate'), 'Text to Video')
  })
})
