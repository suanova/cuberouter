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
import { CHANNEL_TYPE_CUBE_STACK, MODEL_FETCHABLE_TYPES } from '../constants'
import { CHANNEL_TYPE_ADVANCED_CUSTOM } from './advanced-custom'

/** 阻止打开上游模型发现弹窗的原因；null 表示放行。 */
export type ModelFetchBlockReason =
  | 'unsupported_type'
  | 'missing_permission'
  | 'missing_key'

/**
 * 模型发现路由无需鉴权的渠道类型：Advanced Custom 自带的发现端点由用户配置，
 * CubeStack (SGLang) 本地部署通常不开鉴权。这两类在新增时允许留空 key 直接取模型。
 */
export const KEYLESS_MODEL_FETCH_TYPES = new Set<number>([
  CHANNEL_TYPE_ADVANCED_CUSTOM,
  CHANNEL_TYPE_CUBE_STACK,
])

/**
 * 判断「Fetch from Upstream」是否放行。只在新增时校验 key：编辑态的 key 存于库中、
 * 不回填到表单，前端拿不到也就无从校验。CubeStack 的空 key 例外仅覆盖新增态，
 * 其余渠道类型仍要求先填 key，避免把无凭据请求打到需要鉴权的上游。
 */
export function resolveModelFetchBlock({
  type,
  isEditing,
  canEditSensitive,
  apiKey,
}: {
  type: number
  isEditing: boolean
  canEditSensitive: boolean
  apiKey?: string
}): ModelFetchBlockReason | null {
  if (!MODEL_FETCHABLE_TYPES.has(type)) return 'unsupported_type'
  if (!isEditing && !canEditSensitive) return 'missing_permission'
  if (!isEditing && !KEYLESS_MODEL_FETCH_TYPES.has(type) && !apiKey?.trim()) {
    return 'missing_key'
  }
  return null
}
