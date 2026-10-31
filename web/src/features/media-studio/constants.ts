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
import type { AspectRatio, Quality, StudioParams } from './types'

export const API_ENDPOINTS = {
  IMAGES_GENERATIONS: '/pg/images/generations',
  VIDEO_GENERATIONS: '/pg/video/generations',
  PRICING: '/api/pricing',
} as const

/**
 * 模型元数据标签：由运维在「模型元数据」页声明模型的生成能力。Media Studio 据此
 * 分类模型，不再按模型名或端点类型猜测——名字分不清生成与编辑。
 */
export const STUDIO_TAG_TEXT_TO_IMAGE = 'text-to-image'
export const STUDIO_TAG_IMAGE_TO_IMAGE = 'image-to-image'
export const STUDIO_TAG_TEXT_TO_VIDEO = 'text-to-video'
export const STUDIO_TAG_IMAGE_TO_VIDEO = 'image-to-video'
/**
 * 比 image-to-image 多一个能力：接受 mask 蒙版并按透明区域局部重绘。声明了它的模型
 * 才在编辑模式显示框选工具——没有声明的上游会把 mask 当未知字段丢掉，用户框了区域
 * 却得到整图重绘，比不显示这个工具更难排查。
 */
export const STUDIO_TAG_IMAGE_MASK = 'image-mask'

/**
 * 视频时长选项（秒）。各供应商支持的档位不同（如 Kling 5/10、Vidu 5/10），
 * 这里取公共交集；不支持时由上游按 400 报错提示。
 */
export const VIDEO_DURATION_OPTIONS = [5, 10] as const

/** 视频分辨率选项，统一任务体字段 size（如 "720p"）。 */
export const VIDEO_RESOLUTION_OPTIONS = [
  '360p',
  '540p',
  '720p',
  '1080p',
] as const

export const VIDEO_DEFAULT_DURATION = 5
export const VIDEO_DEFAULT_RESOLUTION = '720p'

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

export function serverImageSteps(quality: Quality): number {
  return quality === 'standard' ? 40 : qualitySteps(quality)
}

/** 机器原生参数的可调范围；留空时仍按画质档位与机器默认值下发。 */
export const STEPS_RANGE = { min: 1, max: 60 } as const
export const CFG_RANGE = { min: 0, max: 10 } as const

export const LIMITS = {
  promptMax: 16000,
  maxReferences: 10,
  maxReferenceBytes: 10 * 1024 * 1024,
  // 归一化后参考图通常每张 200~400 KB，10 张仍要留出请求体余量。
  maxReferenceTotalBytes: 11 * 1024 * 1024,
  // 一次框选是一块重绘区域；再多的框已经没法用一句话描述，只说明是误拖。
  maxMaskRegions: 20,
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

// 视频任务异步轮询间隔：上游状态更新通常在 10 ~ 60 秒，5 秒是负载与时效的折中。
export const VIDEO_POLL_INTERVAL_MS = 5 * 1000
