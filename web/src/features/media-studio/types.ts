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
export type AspectRatio = '1:1' | '16:9' | '9:16' | '4:3' | '3:4' | '3:2' | '2:3'

/** 画质档位：映射到 num_inference_steps（见 QUALITY_OPTIONS）。 */
export type Quality = 'fast' | 'standard' | 'high'

export interface StudioParams {
  prompt: string
  ratio: AspectRatio
  count: number
  quality: Quality
  seed: number
  cfg: number
}

export interface GeneratedImage {
  url: string
  seed?: number
}

export interface GenerationResult {
  created: number
  images: GeneratedImage[]
  raw: unknown
}

export type GenerationStatus = 'idle' | 'generating' | 'success' | 'error'

/**
 * 本地历史条目：生成成功后持久化到浏览器 IndexedDB。
 * imageUrls 是自包含的 data URL（上游 b64_json 转换而来）或 http(s) URL，
 * 不依赖任何服务端存储，跨会话、清缓存前均可还原。
 */
export interface HistoryEntry {
  id: string
  prompt: string
  model: string
  params: StudioParams
  imageUrls: string[]
  elapsedMs: number
  createdAt: number
}
