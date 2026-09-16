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
import { api } from '@/lib/api'

import { API_ENDPOINTS, GENERATION_TIMEOUT_MS, IMAGE_GENERATION_ENDPOINT } from './constants'
import { extractImages, type ImageResponseBody } from './lib/image-response'
import type { GenerationRequestBody } from './lib/request-builder'
import type { GeneratedImage } from './types'

interface PricingModelItem {
  model_name?: unknown
  supported_endpoint_types?: unknown
}

interface PricingResponseData {
  success?: unknown
  data?: { pricings?: unknown }
}

/**
 * 拉取当前用户可用、且支持图片生成（supported_endpoint_types 含
 * "image-generation"）的模型名，去重后按名称排序返回。
 */
export async function getStudioModels(): Promise<string[]> {
  const res = await api.get<PricingResponseData>(API_ENDPOINTS.PRICING)
  const body: PricingResponseData = res.data

  const pricings =
    body && body.success && body.data && Array.isArray(body.data.pricings)
      ? (body.data.pricings as PricingModelItem[])
      : []

  const names = new Set<string>()
  for (const item of pricings) {
    if (!item || typeof item.model_name !== 'string' || item.model_name === '') {
      continue
    }
    const endpoints = Array.isArray(item.supported_endpoint_types)
      ? item.supported_endpoint_types
      : []
    if (endpoints.includes(IMAGE_GENERATION_ENDPOINT)) {
      names.add(item.model_name)
    }
  }

  return [...names].sort((a, b) => a.localeCompare(b))
}

export interface GenerationApiResult {
  images: GeneratedImage[]
  created: number
  raw: unknown
}

/**
 * 同步图片生成：阻塞直到上游出图（40 秒 ~ 5 分钟）。
 * 响应遵循 OpenAI images 格式 { created, data: [{ url | b64_json }] }；
 * b64_json 条目由 extractImages 转换为 data URL。
 */
export async function generateImages(
  payload: GenerationRequestBody,
  signal?: AbortSignal,
): Promise<GenerationApiResult> {
  const res = await api.post(API_ENDPOINTS.IMAGES_GENERATIONS, payload, {
    signal,
    timeout: GENERATION_TIMEOUT_MS,
    skipErrorHandler: true,
  } as Record<string, unknown>)

  const body: ImageResponseBody = res.data
  const images = extractImages(body)
  if (images.length === 0) {
    // 2xx 但没有图片：视为上游异常，交给调用方按错误处理
    throw new Error('empty_response')
  }
  return {
    images,
    created: typeof body?.created === 'number' ? body.created : 0,
    raw: body,
  }
}
