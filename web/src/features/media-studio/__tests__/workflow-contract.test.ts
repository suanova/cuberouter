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
test('edit requests send the references as multipart image file parts', () => {
  // 编辑请求改为 multipart/form-data:每张参考图作为一个 image file part,不再
  // base64 内联进 JSON。
  const base = {
    ...initialDraft,
    mode: 'edit' as const,
    model: 'm',
    prompt: 'Cat',
  }
  const single = imageRequest(base, [reference.url])
  expect(single).toBeInstanceOf(FormData)
  const singleFiles = (single as FormData).getAll('image')
  expect(singleFiles).toHaveLength(1)
  expect(singleFiles[0]).toBeInstanceOf(File)

  const multiple = imageRequest(base, [
    reference.url,
    'data:image/png;base64,Yg==',
  ]) as FormData
  expect(multiple.getAll('image')).toHaveLength(2)

  // 没有参考图就不该发图片(会走 JSON 而不是 multipart)。
  expect(imageRequest(base, [])).not.toBeInstanceOf(FormData)
  expect(
    imageRequest({ ...initialDraft, model: 'm', prompt: 'Cat' }, [
      reference.url,
    ])
  ).not.toBeInstanceOf(FormData)
})
test('explicit machine settings override the tier and the machine defaults', () => {
  // 高级设置是覆盖而非新增字段：填了就原样下发，没填才回落到档位/默认值。
  expect(
    imageRequest({
      ...initialDraft,
      model: 'image-model',
      prompt: 'Cat',
      quality: 'fast',
      steps: 12,
      cfg: 6.5,
      seed: 7,
    })
  ).toMatchObject({
    num_inference_steps: 12,
    true_cfg_scale: 6.5,
    seed: 7,
  })
})
test('applying a template carries its reproducible machine settings', () => {
  const template = catalog.templates[0] as StudioTemplate
  const draft = templateDraft(template, {}, initialDraft)
  expect(draft.steps).toBe(template.steps)
  expect(draft.cfg).toBe(template.cfg)
  expect(draft.seed).toBe(template.seed)
  expect(imageRequest(draft)).toMatchObject({
    num_inference_steps: template.steps,
    true_cfg_scale: template.cfg,
    seed: template.seed,
  })
})
test('ten reference images are accepted and eleven are rejected', () => {
  const references = (count: number) =>
    Array.from({ length: count }, (_, index) => ({
      id: `ref-${index}`,
      mime: 'image/png',
      url: reference.url,
    }))
  const base = {
    ...initialDraft,
    mode: 'edit' as const,
    model: 'm',
    prompt: 'Cat',
  }
  expect(
    draftSchema.safeParse({ ...base, references: references(10) }).success
  ).toBe(true)
  expect(
    draftSchema.safeParse({ ...base, references: references(11) }).success
  ).toBe(false)
})
test('only ten references become image file parts in the edit request', () => {
  const urls = Array.from(
    { length: 10 },
    () => 'data:image/png;base64,iVBORw0KGgo='
  )
  const form = imageRequest(
    { ...initialDraft, mode: 'edit', model: 'm', prompt: 'Cat' },
    urls
  ) as FormData
  expect(form.getAll('image')).toHaveLength(10)
})
test('the rasterized mask is a file part rides only when a region was boxed', () => {
  // mask 与 image 是一套：没有参考图就没有可对齐的蒙版，文生图更不能带。
  const edit = {
    ...initialDraft,
    mode: 'edit' as const,
    model: 'm',
    prompt: 'Cat',
  }
  const mask = 'data:image/png;base64,TUFTSw=='
  const withMask = imageRequest(edit, [reference.url], mask) as FormData
  expect(withMask.getAll('image')).toHaveLength(1)
  expect(withMask.get('mask')).toBeInstanceOf(File)

  const withoutMask = imageRequest(edit, [reference.url]) as FormData
  expect(withoutMask.get('mask')).toBeNull()

  // 文生图不带蒙版,且保持 JSON。
  expect(
    imageRequest({ ...initialDraft, model: 'm', prompt: 'Cat' }, [], mask)
  ).not.toBeInstanceOf(FormData)
})
test('boxes outside the image or beyond the cap are rejected before sending', () => {
  // 归一化坐标只允许 0..1；框太多说明是误拖，挡住比让上游报错便宜。
  const boxes = (count: number) =>
    Array.from({ length: count }, () => ({
      x: 0.1,
      y: 0.1,
      width: 0.2,
      height: 0.2,
    }))
  const base = {
    ...initialDraft,
    mode: 'edit' as const,
    model: 'm',
    prompt: 'Cat',
    references: [reference],
  }
  expect(draftSchema.safeParse({ ...base, mask: boxes(20) }).success).toBe(true)
  expect(draftSchema.safeParse({ ...base, mask: boxes(21) }).success).toBe(
    false
  )
  expect(
    draftSchema.safeParse({
      ...base,
      mask: [{ x: -0.1, y: 0, width: 0.2, height: 0.2 }],
    }).success
  ).toBe(false)
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
