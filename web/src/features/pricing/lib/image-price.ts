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
import type { ImagePriceTier } from '../types'

export type ImagePriceTierLabelKey = 'Fast' | 'Standard' | 'High'

/**
 * 图片按张计费价目表的固定画质档位(与图像请求的 quality 字段取值一致)。
 * 顺序即编辑器/展示的行序:Fast → Standard → High。
 */
export const IMAGE_PRICE_TIER_OPTIONS: ReadonlyArray<{
  tier: ImagePriceTier
  labelKey: ImagePriceTierLabelKey
}> = [
  { tier: 'fast', labelKey: 'Fast' },
  { tier: 'standard', labelKey: 'Standard' },
  { tier: 'high', labelKey: 'High' },
]

/**
 * 档位 → i18n 标签键。未知档位(旧数据/后端异常值)返回 undefined,
 * 调用方回退展示原始 tier 字符串。
 */
export function imagePriceTierLabelKey(
  tier: string
): ImagePriceTierLabelKey | undefined {
  return IMAGE_PRICE_TIER_OPTIONS.find((option) => option.tier === tier)
    ?.labelKey
}
