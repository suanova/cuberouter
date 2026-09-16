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

import type { ImagePriceTable } from '@/features/pricing/types'
import { localToUsdNumber, usdToLocalNumber } from '@/lib/currency'

import { formatPricingNumber, rebaseDisplayPriceDraft } from './pricing-format'

/**
 * 图片价格草稿的货币换算边界(与视频价格草稿同口径):
 * - 加载(表 → 草稿):内部 USD/张 → 显示货币字符串(×rate 入口,formatPricingNumber 归整);
 * - 提交(草稿 → 表):显示货币数值 → USD/张(÷rate 出口)。
 */
export type ImagePriceRowDraft = {
  id: string
  resolution: string
  price: string
}

export function createImagePriceRowDraft(): ImagePriceRowDraft {
  return { id: nanoid(), resolution: '', price: '' }
}

export function imagePriceDraftsFromTable(
  table: ImagePriceTable
): ImagePriceRowDraft[] {
  return table.rows.map((row) => ({
    id: nanoid(),
    resolution: row.resolution,
    price: formatPricingNumber(usdToLocalNumber(row.price)),
  }))
}

function parsePriceDraft(value: string): number {
  const trimmed = value.trim()
  if (trimmed === '') return 0
  const parsed = Number(trimmed)
  if (!Number.isFinite(parsed)) return 0
  return localToUsdNumber(parsed)
}

/**
 * Emits the table payload for a set of drafts. Fully empty rows are dropped;
 * partially filled rows are kept as-is so backend validation rejects them.
 * Draft numbers are display currency and come back out as USD per image.
 */
export function imagePriceTableFromDrafts(
  drafts: ImagePriceRowDraft[]
): ImagePriceTable {
  return {
    rows: drafts
      .filter(
        (draft) => draft.resolution.trim() !== '' || draft.price.trim() !== ''
      )
      .map((draft) => ({
        resolution: draft.resolution.trim(),
        price: parsePriceDraft(draft.price),
      })),
  }
}

/**
 * 汇率变化时 rebase 整张草稿表:每行价格串保持其底层 USD 意图换算到新汇率
 * (resolution 不动;空/未完成录入原样保留)。
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

export function addImagePriceRowDraft(
  drafts: ImagePriceRowDraft[]
): ImagePriceRowDraft[] {
  return [...drafts, createImagePriceRowDraft()]
}

export function removeImagePriceRowDraft(
  drafts: ImagePriceRowDraft[],
  index: number
): ImagePriceRowDraft[] {
  return drafts.filter((_, i) => i !== index)
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
