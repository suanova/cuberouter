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
import { beforeEach, describe, expect, test, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/lib/api', () => ({
  api: { get, post },
}))

import { generateImages, getStudioModels } from '../api'
import { GENERATION_TIMEOUT_MS } from '../constants'
import type { GenerationRequestBody } from '../lib/request-builder'

function samplePayload(): GenerationRequestBody {
  return {
    model: 'qwen-image-2512',
    prompt: 'a cat',
    n: 1,
    size: '1328x1328',
    seed: 42,
    num_inference_steps: 30,
    true_cfg_scale: 4,
    quality: 'standard',
  }
}

function pricingResponse(pricings: unknown[]) {
  return {
    data: {
      success: true,
      data: { pricings },
    },
  }
}

describe('getStudioModels', () => {
  beforeEach(() => {
    get.mockReset()
  })

  test('classifies models by declared tags and ignores endpoint types', async () => {
    get.mockResolvedValue(
      pricingResponse([
        {
          model_name: 'qwen-image-2512',
          tags: 'text-to-image,hot',
          supported_endpoint_types: ['image-generation', 'openai'],
        },
        {
          // 回归锚点：它带着 image-generation 端点类型（按名字推断的遗留结果），
          // 但只声明了 image-to-image。旧的端点类型过滤会把它混进文生图列表，
          // 用户在那里选它就会拿到上游 404。
          model_name: 'qwen-image-edit-2511',
          tags: 'image-to-image',
          supported_endpoint_types: ['image-generation', 'openai'],
        },
        {
          model_name: 'both-modes-model',
          tags: 'text-to-image,image-to-image',
        },
        // 标签大小写与首尾空白不敏感；模型名保持小写，避免断言依赖 locale 的排序规则。
        { model_name: 'case-tag-model', tags: ' Text-To-Image ' },
        {
          model_name: 'no-tags-model',
          supported_endpoint_types: ['image-generation'],
        },
        { model_name: 'chat-model', tags: 'chat' },
        { model_name: 'qwen-image-2512', tags: 'text-to-image' },
      ])
    )

    await expect(getStudioModels()).resolves.toEqual({
      textToImage: ['both-modes-model', 'case-tag-model', 'qwen-image-2512'],
      imageToImage: ['both-modes-model', 'qwen-image-edit-2511'],
    })
    expect(get).toHaveBeenCalledWith('/api/pricing')
  })

  test('returns empty catalogs when no model carries an image label', async () => {
    get.mockResolvedValue(
      pricingResponse([
        { model_name: 'gpt-4o', supported_endpoint_types: ['openai'] },
      ])
    )

    await expect(getStudioModels()).resolves.toEqual({
      textToImage: [],
      imageToImage: [],
    })
  })

  test('returns empty catalogs when the pricing payload is unusable', async () => {
    get.mockResolvedValue({ data: { success: false, data: null } })

    await expect(getStudioModels()).resolves.toEqual({
      textToImage: [],
      imageToImage: [],
    })
  })
})

describe('generateImages', () => {
  beforeEach(() => {
    post.mockReset()
  })

  test('resolves b64_json-only upstream payloads as data URLs', async () => {
    const b64 = 'iVBORw0KGgoAAAANSUhEUg'
    post.mockResolvedValue({ data: { created: 7, data: [{ b64_json: b64 }] } })

    const result = await generateImages(samplePayload())

    expect(result.images).toEqual([{ url: `data:image/png;base64,${b64}` }])
    expect(result.created).toBe(7)
    expect(post).toHaveBeenCalledWith(
      '/pg/images/generations',
      expect.anything(),
      expect.objectContaining({ timeout: GENERATION_TIMEOUT_MS })
    )
  })

  test('keeps url-only upstream items unchanged', async () => {
    post.mockResolvedValue({
      data: { created: 8, data: [{ url: 'https://cdn.example/img.png' }] },
    })

    const result = await generateImages(samplePayload())

    expect(result.images).toEqual([{ url: 'https://cdn.example/img.png' }])
    expect(result.created).toBe(8)
  })

  test('rejects with empty_response when a 2xx body carries no usable image', async () => {
    post.mockResolvedValue({
      data: { created: 9, data: [{ revised_prompt: 'a cat' }] },
    })

    await expect(generateImages(samplePayload())).rejects.toThrow(
      'empty_response'
    )
  })
})
