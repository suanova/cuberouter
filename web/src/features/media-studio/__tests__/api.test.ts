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
    num_inference_steps: 40,
    true_cfg_scale: 1,
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

  test('keeps only image-generation models, deduped and sorted', async () => {
    get.mockResolvedValue(
      pricingResponse([
        { model_name: 'qwen-image-2512', supported_endpoint_types: ['image-generation', 'openai'] },
        { model_name: 'flux-dev', supported_endpoint_types: ['image-generation'] },
        { model_name: 'qwen-image-2512', supported_endpoint_types: ['image-generation'] },
        { model_name: 'gpt-4o', supported_endpoint_types: ['openai'] },
        { model_name: 'doubao-video', supported_endpoint_types: ['openai-video'] },
        { model_name: 'no-endpoints-model' },
      ]),
    )

    await expect(getStudioModels()).resolves.toEqual(['flux-dev', 'qwen-image-2512'])
    expect(get).toHaveBeenCalledWith('/api/pricing')
  })

  test('returns an empty list when no model supports image generation', async () => {
    get.mockResolvedValue(
      pricingResponse([
        { model_name: 'gpt-4o', supported_endpoint_types: ['openai'] },
      ]),
    )

    await expect(getStudioModels()).resolves.toEqual([])
  })

  test('returns an empty list when the pricing payload is unusable', async () => {
    get.mockResolvedValue({ data: { success: false, data: null } })

    await expect(getStudioModels()).resolves.toEqual([])
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
