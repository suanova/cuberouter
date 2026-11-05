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
import type { MaskRegion } from './lib/mask'
import type { Quality } from './types'

export interface StudioAsset {
  id: string
  url: string
  mime: string
}
/** 按运维声明的模型标签分类的可用模型：文生图/图生图与文生视频/图生视频互不推断。 */
export interface StudioModelCatalog {
  textToImage: string[]
  imageToImage: string[]
  textToVideo: string[]
  imageToVideo: string[]
  /** 支持蒙版区域重绘（image-mask 标签）的编辑模型。 */
  imageMask: string[]
}
export interface WorkflowDraft {
  markup?: import('./lib/reference-markup').ReferenceMarkup
  /** Original user instruction when an editor adds model-facing guidance. */
  user_prompt?: string
  mode: 'create' | 'edit'
  model: string
  prompt: string
  size: string
  count: number
  quality: Quality
  references: StudioAsset[]
  /** 留空时按画质档位推导（fast/standard/high → 20/30/50）。 */
  steps?: number
  /** 留空时使用机器默认值。 */
  cfg?: number
  /** 留空时使用机器默认值；指定后同一提示词可复现同一张图。 */
  seed?: number
  /**
   * 框选重绘区域（归一化坐标），作用于第一张参考图。留空即整图编辑，
   * 请求体不带 mask。
   */
  mask?: MaskRegion[]
  parent_id?: string
}
export interface WorkflowJob {
  id: string
  created_at: number
  request: WorkflowDraft
  images: StudioAsset[]
  elapsed_ms: number
  request_id?: string
}
export interface StudioTemplate {
  id: string
  title: string
  category: string
  collection?: string
  mode: 'create' | 'edit'
  size: string
  description: string
  tip: string
  prompt: string
  preview: string
  before?: string
  seed: number
  steps: number
  cfg: number
  fields: Array<{ key: string; label: string; value: string }>
}
