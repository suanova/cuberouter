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
import { ASPECT_RATIOS } from '../constants'
import type { StudioParams } from '../types'

/**
 * 请求体契约（与渠道侧约定）：
 * - model: 页面选中的图片模型
 * - size: 像素尺寸 "WxH"（如 "1328x1328"），由选中比例映射为机器原生分辨率
 * - seed / num_inference_steps / true_cfg_scale: 机器原生扩展字段
 */
export interface GenerationRequestBody {
  model: string
  prompt: string
  n: number
  size: string
  seed: number
  num_inference_steps: number
  true_cfg_scale: number
}

export function buildGenerationRequest(
  params: StudioParams,
  model: string,
): GenerationRequestBody {
  const [width, height] = ASPECT_RATIOS[params.ratio]
  return {
    model,
    prompt: params.prompt.trim(),
    n: params.count,
    size: `${width}x${height}`,
    seed: params.seed,
    num_inference_steps: params.steps,
    true_cfg_scale: params.cfg,
  }
}
