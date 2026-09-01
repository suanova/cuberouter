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
import { getCurrencyDisplay } from '@/lib/currency'

import { DEFAULT_DISCOUNT_RATE } from '../constants'

// ============================================================================
// Wallet-specific Formatting Functions
// ============================================================================

/**
 * Format Creem price with currency symbol (USD/EUR)
 */
export function formatCreemPrice(
  price: number,
  currency: 'USD' | 'EUR'
): string {
  const symbol = currency === 'EUR' ? '€' : '$'
  return `${symbol}${price.toFixed(2)}`
}

/**
 * Format large quota numbers with K/M suffix
 */
export function formatQuotaShort(quota: number): string {
  if (quota >= 1000000) {
    return `${(quota / 1000000).toFixed(1)}M`
  }
  if (quota >= 1000) {
    return `${(quota / 1000).toFixed(1)}K`
  }
  return quota.toString()
}

/**
 * Currency symbol for the current display type: ¥ for CNY, $ for USD,
 * the custom symbol for CUSTOM, empty for TOKENS.
 */
export function getCurrencySymbol(): string {
  const { config } = getCurrencyDisplay()
  switch (config.quotaDisplayType) {
    case 'CNY':
      return '¥'
    case 'USD':
      return '$'
    case 'CUSTOM':
      return config.customCurrencySymbol || '¤'
    default:
      return ''
  }
}

/**
 * Local payment currency symbol for the Pay/Save amounts: those values are
 * the payment amount converted via Price (local currency), which differs
 * from the display currency in USD mode (e.g. $10 display → Pay ¥73).
 */
export function getLocalCurrencySymbol(): string {
  const { config } = getCurrencyDisplay()
  switch (config.quotaDisplayType) {
    case 'TOKENS':
      return ''
    case 'CUSTOM':
      return config.customCurrencySymbol || '¤'
    default:
      return '¥'
  }
}

/**
 * Format currency amount that is already in local currency.
 * This is used for payment amounts that have been calculated via priceRatio.
 */
export function formatCurrency(amount: number | string): string {
  const numeric =
    typeof amount === 'number' ? amount : Number.parseFloat(String(amount))
  if (!Number.isFinite(numeric)) return '-'

  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 0,
    maximumFractionDigits: Math.abs(numeric) >= 1 ? 2 : 4,
  }).format(numeric)
}

/**
 * Get discount label for display (e.g., "20% OFF")
 */
export function getDiscountLabel(discount: number): string {
  if (discount >= DEFAULT_DISCOUNT_RATE) {
    return ''
  }
  const off = Math.round((1 - discount) * 100)
  return `${off}% OFF`
}

/**
 * Calculate pricing details for a preset amount.
 * presetValue 的语义 = 显示货币(CNY 填元 / USD 填美元 / TOKENS 填 token 数)。
 * 支付金额(本地货币):CNY 模式值即元(rate>1 时 value/rate×price = value);
 * USD 模式(rate=1)为 value × priceRatio。
 */
export function calculatePresetPricing(
  presetValue: number,
  priceRatio: number,
  discount: number,
  usdExchangeRate: number = 1
) {
  const originalPrice =
    usdExchangeRate > 1
      ? (presetValue / usdExchangeRate) * priceRatio
      : presetValue * priceRatio
  const actualPrice = originalPrice * discount
  const savedAmount = originalPrice - actualPrice
  const hasDiscount = discount < 1.0
  const displayValue = presetValue

  return {
    displayValue,
    originalPrice,
    actualPrice,
    savedAmount,
    hasDiscount,
  }
}
