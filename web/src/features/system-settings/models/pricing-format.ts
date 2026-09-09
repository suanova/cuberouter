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
const DISPLAY_DECIMALS = 12
const SNAP_DECIMALS = 8
const SNAP_EPSILON = 1e-12

function toNumberOrNull(value: unknown): number | null {
  if (
    value === '' ||
    value === null ||
    value === undefined ||
    value === false
  ) {
    return null
  }

  const num = Number(value)
  return Number.isFinite(num) ? num : null
}

function roundToDecimals(value: number, decimals: number): number {
  const factor = 10 ** decimals
  return Math.round(value * factor) / factor
}

function snapFloatDrift(value: number): number {
  const tolerance = Math.max(SNAP_EPSILON, Math.abs(value) * Number.EPSILON * 8)

  for (let decimals = 0; decimals <= SNAP_DECIMALS; decimals += 1) {
    const rounded = roundToDecimals(value, decimals)
    if (Math.abs(value - rounded) <= tolerance) {
      return rounded
    }
  }

  return value
}

// 归整展示串:≥1e-12 量级(12 位小数内)保真;更小量级(< ~5e-13)会被 toFixed(12)
// 归为 "0" —— 现实定价不可达。结果经 parseFloat 修剪尾零,不是字符串级恒等。
export function formatPricingNumber(value: unknown): string {
  const num = toNumberOrNull(value)
  if (num === null) return ''

  const normalized = snapFloatDrift(num)
  return Number.parseFloat(normalized.toFixed(DISPLAY_DECIMALS)).toString()
}

/**
 * 显示货币草稿 rebase:把同一底层美元意图从 fromRate 重换算到 toRate。
 * 不可解析的录入串(空串、输入中的 '.' 等)原样返回,不打断输入;两汇率相等时恒等。
 * 用于编辑态站点展示货币变化时重算显示草稿,保证保存端 ÷当前汇率与草稿显示的
 * 美元意图一致(否则跨汇率保存会把旧汇率显示值当新汇率值写入)。
 */
export function rebaseDisplayPriceDraft(
  value: string,
  fromRate: number,
  toRate: number
): string {
  if (fromRate === toRate) return value
  const num = toNumberOrNull(value)
  if (num === null) return value
  return formatPricingNumber((num / fromRate) * toRate)
}
