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
import type { UserModelDiscounts } from '../types'

/**
 * The dialogs edit model discounts as whole percents (1-100) while the backend
 * stores the 0.01-1 multiplier actually applied to the model ratio, so the
 * editor converts between the two and keeps the stored value at 4 decimals.
 */
export function percentToDiscount(percent: number): number {
  return Math.round(percent * 100) / 10000
}

export function discountToPercent(discount: number): number {
  return discount * 100
}

export function isValidPercent(percent: number): boolean {
  return Number.isFinite(percent) && percent >= 1 && percent <= 100
}

/**
 * Returns a new map with `model` set to `percent`. Throws on a percent the
 * backend would reject, so a caller never writes a discount that cannot be
 * saved.
 */
export function upsertDiscount(
  value: UserModelDiscounts,
  model: string,
  percent: number
): UserModelDiscounts {
  if (!isValidPercent(percent)) {
    throw new RangeError(`Invalid model discount percent: ${percent}`)
  }
  return { ...value, [model]: percentToDiscount(percent) }
}

export function removeDiscount(
  value: UserModelDiscounts,
  model: string
): UserModelDiscounts {
  const next = { ...value }
  delete next[model]
  return next
}
