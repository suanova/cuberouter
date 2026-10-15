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
import { expect, test, vi } from 'vitest'

import {
  annotatedDraft,
  drawStrokes,
  editDraft,
  newEditSeed,
  referenceDimensions,
} from '../lib/qwen-edit'
import { qwenParameters } from '../lib/qwen-queue'
import { draftSchema, initialDraft } from '../lib/workflow'

const original = {
  id: 'original',
  url: 'data:image/png;base64,b3JpZ2luYWw=',
  mime: 'image/png',
}
const marked = {
  id: 'marked',
  url: 'data:image/png;base64,bWFya2Vk',
  mime: 'image/png',
}

test('a new edit advances a repeated maximum seed without exceeding the worker range', () => {
  const draw = vi
    .spyOn(crypto, 'getRandomValues')
    .mockImplementation((array) => {
      ;(array as Uint32Array)[0] = 4294967295
      return array
    })
  try {
    expect(newEditSeed(4294967295)).toBe(0)
  } finally {
    draw.mockRestore()
  }
})

test.each([
  [1280, 1280, '1280x1280'],
  [1536, 1024, '1536x1024'],
  [3000, 2000, '1664x1088'],
])(
  'a %i by %i uploaded reference defaults to supported dimensions %s',
  (width, height, expected) =>
    expect(referenceDimensions(width, height)).toBe(expected)
)
test('freehand strokes keep their endpoints open without filling', () => {
  const ctx = {
    beginPath: vi.fn(),
    moveTo: vi.fn(),
    lineTo: vi.fn(),
    closePath: vi.fn(),
    fill: vi.fn(),
    stroke: vi.fn(),
  }
  drawStrokes(
    ctx as unknown as CanvasRenderingContext2D,
    [
      {
        color: '#ffffff',
        points: [
          [0, 5],
          [7, 11],
          [15, 4],
        ],
      },
    ],
    512
  )
  expect(ctx.moveTo).toHaveBeenCalledWith(0, 5)
  expect(ctx.lineTo.mock.calls).toEqual([
    [7, 11],
    [15, 4],
  ])
  expect(ctx.closePath).not.toHaveBeenCalled()
  expect(ctx.fill).not.toHaveBeenCalled()
})
test('annotated edits submit marked bytes and the one instruction while preserving the original locally', () => {
  const result = annotatedDraft(
    { ...initialDraft, queued: true, model: 'qwen-image-2.1' },
    original,
    marked,
    [],
    'Add three divers in the white regions.'
  )
  expect(qwenParameters(result).images).toEqual(['bWFya2Vk'])
  expect(qwenParameters(result).prompt.match(/Add three divers/g)).toHaveLength(
    1
  )
  expect(qwenParameters(result).user_prompt).toBe(
    'Add three divers in the white regions.'
  )
  expect(result.annotation?.original).toEqual(original)
})
test('continuing an edit refreshes the seed and retains dimensions and other parameters', () => {
  const draft = {
    ...initialDraft,
    model: 'qwen-image-2.1',
    queued: true,
    size: '1120x736',
    cfg: 0,
    seed: 0,
    steps: 8,
  }
  const next = editDraft(draft, original, 'parent')
  expect(next.seed).not.toBe(draft.seed)
  expect(next).toMatchObject({
    size: '1120x736',
    cfg: 0,
    steps: 8,
    parent_id: 'parent',
    references: [original],
    mode: 'edit',
  })
  expect(qwenParameters(draft)).toMatchObject({ cfg: 0, seed: 0, steps: 8 })
})
test('Qwen accepts ten references and rejects an eleventh and unsupported dimensions', () => {
  const draft = {
    ...initialDraft,
    mode: 'edit' as const,
    model: 'qwen-image-2.1',
    queued: true,
    prompt: 'Combine',
    references: Array.from({ length: 10 }, () => original),
  }
  expect(draftSchema.safeParse(draft).success).toBe(true)
  expect(
    draftSchema.safeParse({
      ...draft,
      references: [...draft.references, original],
    }).success
  ).toBe(false)
  expect(draftSchema.safeParse({ ...draft, size: '1328x1328' }).success).toBe(
    false
  )
  expect(draftSchema.safeParse({ ...draft, queued: false }).success).toBe(false)
})
