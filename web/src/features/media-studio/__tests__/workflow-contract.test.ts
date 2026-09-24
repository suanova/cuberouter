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
import { expect, test } from 'vitest'

import catalog from '../catalog.json'
import {
  draftSchema,
  initialDraft,
  imageRequest,
  templateDraft,
} from '../lib/workflow'
import type { Quality } from '../types'
import type { StudioTemplate } from '../workflow-types'

const reference = {
  id: 'photo',
  mime: 'image/png',
  url: 'data:image/png;base64,YQ==',
}
test('edit templates preserve references without uploading catalog example images', () => {
  const template = catalog.templates.find(
    (item) => item.id === 'pet-comic'
  ) as StudioTemplate
  const draft = templateDraft(
    template,
    { story: 'The cat makes tea', style: 'Watercolor' },
    {
      ...initialDraft,
      model: 'channel-model',
      references: [reference],
      parent_id: 'old',
    }
  )
  expect(draft.references).toEqual([reference])
  expect(draft.parent_id).toBe('old')
  expect(draft.prompt).toContain('The cat makes tea')
  expect(draft.model).toBe('channel-model')
})
test('create templates clear edit references and preserve provider-specific size', () => {
  const template = catalog.templates.find(
    (item) => item.id === 'chibi-animal'
  ) as StudioTemplate
  const draft = templateDraft(
    template,
    {},
    {
      ...initialDraft,
      size: '1024x1536',
      references: [reference],
      parent_id: 'old',
    }
  )
  expect(draft.references).toEqual([])
  expect(draft.parent_id).toBeUndefined()
  expect(draft.size).toBe('1024x1536')
})
test('image requests derive the diffusion step count from the selected quality tier', () => {
  const tiers: Array<[Quality, number]> = [
    ['fast', 20],
    ['standard', 30],
    ['high', 50],
  ]
  for (const [quality, steps] of tiers) {
    expect(
      imageRequest({
        ...initialDraft,
        model: 'image-model',
        prompt: 'Cat',
        quality,
      })
    ).toMatchObject({ quality, num_inference_steps: steps })
  }
})
test('image requests always carry the quality tier with the machine-native defaults', () => {
  expect(
    imageRequest({ ...initialDraft, model: 'image-model', prompt: 'Cat' })
  ).toEqual({
    model: 'image-model',
    prompt: 'Cat',
    n: 1,
    size: '1024x1024',
    quality: 'standard',
    num_inference_steps: 30,
    seed: 42,
    true_cfg_scale: 4,
  })
})
test('counts above four are rejected', () =>
  expect(
    draftSchema.safeParse({
      ...initialDraft,
      model: 'model',
      prompt: 'Cat',
      count: 5,
    }).success
  ).toBe(false))
test('edit requests without references are rejected', () =>
  expect(
    draftSchema.safeParse({
      ...initialDraft,
      model: 'model',
      prompt: 'Cat',
      mode: 'edit',
    }).success
  ).toBe(false))
