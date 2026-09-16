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

import { API_ENDPOINTS, GENERATION_TIMEOUT_MS } from './constants'
import type { GenerationRequestBody } from './lib/request-builder'
import type { GeneratedImage } from './types'

interface OpenAIImageResponseItem {
  url?: unknown
  b64_json?: unknown
}

interface OpenAIImageResponse {
  created?: unknown
  data?: unknown
}

export interface GenerationApiResult {
  images: GeneratedImage[]
  created: number
  raw: unknown
}

/**
 * 同步图片生成：阻塞直到上游出图（40 秒 ~ 5 分钟）。
 * 响应遵循 OpenAI images 格式 { created, data: [{ url }] }。
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

  const body: OpenAIImageResponse = res.data
  const items = Array.isArray(body?.data) ? body.data : []
  const images: GeneratedImage[] = []
  for (const item of items as OpenAIImageResponseItem[]) {
    if (item && typeof item.url === 'string' && item.url !== '') {
      images.push({ url: item.url })
    }
  }
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
