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
import { nanoid } from 'nanoid'

import { IMAGE_PRICE_TIER_OPTIONS } from '@/features/pricing/lib/image-price'
import type { ImagePriceTable } from '@/features/pricing/types'
import { localToUsdNumber, usdToLocalNumber } from '@/lib/currency'

import { formatPricingNumber, rebaseDisplayPriceDraft } from './pricing-format'

/**
 * 图片价格草稿的货币换算边界(与视频价格草稿同口径):
 * - 加载(表 → 草稿):内部 USD/张 → 显示货币字符串(×rate 入口,formatPricingNumber 归整);
 * - 提交(草稿 → 表):显示货币数值 → USD/张(÷rate 出口)。
 *
 * 档位固定为 fast/standard/high 三行(IMAGE_PRICE_TIER_OPTIONS 顺序),
 * 不可增删;价格为空的档位视为未定价,不进入载荷(后端按锚点计费)。
 */
export type ImagePriceRowDraft = {
  id: string
  tier: (typeof IMAGE_PRICE_TIER_OPTIONS)[number]['tier']
  price: string
}

export function imagePriceDraftsFromTable(
  table: ImagePriceTable
): ImagePriceRowDraft[] {
  return IMAGE_PRICE_TIER_OPTIONS.map((option) => {
    const row = table.rows.find((entry) => entry.tier === option.tier)
    return {
      id: nanoid(),
      tier: option.tier,
      price: row ? formatPricingNumber(usdToLocalNumber(row.price)) : '',
    }
  })
}

function parsePriceDraft(value: string): number {
  const trimmed = value.trim()
  if (trimmed === '') return 0
  const parsed = Number(trimmed)
  if (!Number.isFinite(parsed)) return 0
  return localToUsdNumber(parsed)
}

/**
 * Emits the table payload for the fixed tier drafts. Tiers with an empty
 * price are dropped (unpriced tiers bill at the anchor); an explicit "0"
 * is kept as-is so backend validation rejects it.
 * Draft numbers are display currency and come back out as USD per image.
 */
export function imagePriceTableFromDrafts(
  drafts: ImagePriceRowDraft[]
): ImagePriceTable {
  return {
    rows: drafts
      .filter((draft) => draft.price.trim() !== '')
      .map((draft) => ({
        tier: draft.tier,
        price: parsePriceDraft(draft.price),
      })),
  }
}

/**
 * 汇率变化时 rebase 整张草稿表:每行价格串保持其底层 USD 意图换算到新汇率
 * (档位固定不动;空/未完成录入原样保留)。
 */
export function rebaseImagePriceDrafts(
  drafts: ImagePriceRowDraft[],
  fromRate: number,
  toRate: number
): ImagePriceRowDraft[] {
  if (fromRate === toRate) return drafts
  return drafts.map((draft) => ({
    ...draft,
    price: rebaseDisplayPriceDraft(draft.price, fromRate, toRate),
  }))
}

export function updateImagePriceRowDraft(
  drafts: ImagePriceRowDraft[],
  index: number,
  patch: Partial<ImagePriceRowDraft>
): ImagePriceRowDraft[] {
  return drafts.map((draft, i) =>
    i === index ? { ...draft, ...patch } : draft
  )
}
