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
import { api } from '@/lib/api'

import {
  API_ENDPOINTS,
  GENERATION_TIMEOUT_MS,
  STUDIO_TAG_IMAGE_TO_IMAGE,
  STUDIO_TAG_TEXT_TO_IMAGE,
} from './constants'
import { extractImages, type ImageResponseBody } from './lib/image-response'
import type { GenerationRequestBody } from './lib/request-builder'
import type { GeneratedImage } from './types'
import type { StudioModelCatalog } from './workflow-types'

interface PricingModelItem {
  model_name?: unknown
  tags?: unknown
}

interface PricingResponseData {
  success?: unknown
  data?: { pricings?: unknown }
}

/**
 * 解析 /api/pricing 的标签串。只按逗号切分，与模型元数据抽屉的写入契约
 * （values.tags.join(',')）一致——TagInput 允许标签内含空格，所以不能按空白切分。
 * 大小写与首尾空白不敏感，与后端 common.HasModelTag 语义保持一致。
 */
function parseModelTags(value: unknown): string[] {
  if (typeof value !== 'string') {
    return []
  }
  return value
    .split(',')
    .map((tag) => tag.trim().toLowerCase())
    .filter(Boolean)
}

/**
 * 拉取当前用户可用、并按运维声明的模型标签分类的模型名。
 * text-to-image 进文生图列表，image-to-image 进图生图列表，两者互不推断：
 * 只支持编辑的模型绝不会出现在文生图列表里。supported_endpoint_types 不参与
 * 分类——按模型名推断的端点类型曾把 qwen-image-edit-* 误列成文生图模型。
 */
export async function getStudioModels(): Promise<StudioModelCatalog> {
  const res = await api.get<PricingResponseData>(API_ENDPOINTS.PRICING)
  const body: PricingResponseData = res.data

  const pricings =
    body && body.success && body.data && Array.isArray(body.data.pricings)
      ? (body.data.pricings as PricingModelItem[])
      : []

  const textToImage = new Set<string>()
  const imageToImage = new Set<string>()
  for (const item of pricings) {
    if (!item || typeof item.model_name !== 'string' || item.model_name === '') {
      continue
    }
    const tags = parseModelTags(item.tags)
    if (tags.includes(STUDIO_TAG_TEXT_TO_IMAGE)) {
      textToImage.add(item.model_name)
    }
    if (tags.includes(STUDIO_TAG_IMAGE_TO_IMAGE)) {
      imageToImage.add(item.model_name)
    }
  }

  return {
    textToImage: [...textToImage].sort((a, b) => a.localeCompare(b)),
    imageToImage: [...imageToImage].sort((a, b) => a.localeCompare(b)),
  }
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
