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
import type { AspectRatio, Quality, StudioParams } from './types'

export const API_ENDPOINTS = {
  IMAGES_GENERATIONS: '/pg/images/generations',
  PRICING: '/api/pricing',
} as const

/** 支持图片生成的端点类型（与 /api/pricing 的 supported_endpoint_types 对齐） */
export const IMAGE_GENERATION_ENDPOINT = 'image-generation'

/**
 * Aspect ratios and their native output dimensions (px),
 * matching the Qwen-Image generation server.
 */
export const ASPECT_RATIOS: Record<AspectRatio, [number, number]> = {
  '1:1': [1328, 1328],
  '16:9': [1664, 928],
  '9:16': [928, 1664],
  '4:3': [1472, 1104],
  '3:4': [1104, 1472],
  '3:2': [1584, 1056],
  '2:3': [1056, 1584],
}

export const ASPECT_RATIO_ORDER: AspectRatio[] = [
  '1:1',
  '16:9',
  '9:16',
  '4:3',
  '3:4',
  '3:2',
  '2:3',
]

export const COUNT_OPTIONS = [1, 2, 3, 4] as const

/** 画质档位 → 推理步数（请求体字段 num_inference_steps）。 */
export const QUALITY_OPTIONS = [
  { id: 'fast', labelKey: 'Fast', steps: 20 },
  { id: 'standard', labelKey: 'Standard', steps: 30 },
  { id: 'high', labelKey: 'High', steps: 50 },
] as const

export function qualitySteps(quality: Quality): number {
  const option = QUALITY_OPTIONS.find((entry) => entry.id === quality)
  if (!option) {
    return QUALITY_OPTIONS[1].steps
  }
  return option.steps
}

export const LIMITS = {
  promptMax: 16000,
} as const

export const DEFAULT_PARAMS: StudioParams = {
  prompt: '',
  ratio: '16:9',
  count: 1,
  quality: 'standard',
  seed: 42,
  cfg: 4,
}

// 同步生成阻塞 40 秒 ~ 5 分钟，超时放宽到 10 分钟
export const GENERATION_TIMEOUT_MS = 10 * 60 * 1000
