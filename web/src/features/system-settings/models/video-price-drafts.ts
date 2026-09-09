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

import type { VideoPriceTable } from '@/features/pricing/types'
import { localToUsdNumber, usdToLocalNumber } from '@/lib/currency'

import { formatPricingNumber, rebaseDisplayPriceDraft } from './pricing-format'

/**
 * 视频价格草稿的货币换算边界:
 * - 加载(表 → 草稿):内部 USD/s → 显示货币字符串(×rate 入口,formatPricingNumber 归整);
 * - 提交(草稿 → 表):显示货币数值 → USD/s(÷rate 出口)。
 * 换算经 formatPricingNumber/snapFloatDrift 归整:≥1e-12 量级的现实定价往返保真;
 * 更小量级会归零(现实定价不可达),故显示货币与 USD 相同(或汇率非法回落 1)时
 * 仅跳过 ×rate,不保证字符串级恒等(尾零会被修剪、极小量级归 "0")。草稿内部字符串
 * 始终为用户输入的显示货币,只有这两个边界函数做换算。
 */
export type VideoPriceRowDraft = {
  id: string
  resolution: string
  normalPrice: string
  offPeakPrice: string
}

export function createVideoPriceRowDraft(): VideoPriceRowDraft {
  return { id: nanoid(), resolution: '', normalPrice: '', offPeakPrice: '' }
}

export function videoPriceDraftsFromTable(
  table: VideoPriceTable
): VideoPriceRowDraft[] {
  return table.rows.map((row) => ({
    id: nanoid(),
    resolution: row.resolution,
    normalPrice: formatPricingNumber(usdToLocalNumber(row.normal_price)),
    offPeakPrice: formatPricingNumber(usdToLocalNumber(row.off_peak_price)),
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
 * Draft numbers are display currency and come back out as USD/s.
 */
export function videoPriceTableFromDrafts(
  drafts: VideoPriceRowDraft[]
): VideoPriceTable {
  return {
    rows: drafts
      .filter(
        (draft) =>
          draft.resolution.trim() !== '' ||
          draft.normalPrice.trim() !== '' ||
          draft.offPeakPrice.trim() !== ''
      )
      .map((draft) => ({
        resolution: draft.resolution.trim(),
        normal_price: parsePriceDraft(draft.normalPrice),
        off_peak_price: parsePriceDraft(draft.offPeakPrice),
      })),
  }
}

/**
 * 汇率变化时 rebase 整张草稿表:每行价格串保持其底层 USD 意图换算到新汇率
 * (resolution 不动;空/未完成录入原样保留)。
 */
export function rebaseVideoPriceDrafts(
  drafts: VideoPriceRowDraft[],
  fromRate: number,
  toRate: number
): VideoPriceRowDraft[] {
  if (fromRate === toRate) return drafts
  return drafts.map((draft) => ({
    ...draft,
    normalPrice: rebaseDisplayPriceDraft(draft.normalPrice, fromRate, toRate),
    offPeakPrice: rebaseDisplayPriceDraft(
      draft.offPeakPrice,
      fromRate,
      toRate
    ),
  }))
}

export function addVideoPriceRowDraft(
  drafts: VideoPriceRowDraft[]
): VideoPriceRowDraft[] {
  return [...drafts, createVideoPriceRowDraft()]
}

export function removeVideoPriceRowDraft(
  drafts: VideoPriceRowDraft[],
  index: number
): VideoPriceRowDraft[] {
  return drafts.filter((_, i) => i !== index)
}

export function updateVideoPriceRowDraft(
  drafts: VideoPriceRowDraft[],
  index: number,
  patch: Partial<VideoPriceRowDraft>
): VideoPriceRowDraft[] {
  return drafts.map((draft, i) => (i === index ? { ...draft, ...patch } : draft))
}
