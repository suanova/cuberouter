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

/**
 * Keep only positive integer organization IDs, deduplicated and sorted so the
 * saved value and the change comparison are stable regardless of pick order.
 */
export function normalizeDshOrganizationIds(values: unknown[]): number[] {
  const ids = new Set<number>()
  for (const value of values) {
    const id = typeof value === 'number' ? value : Number(value)
    if (Number.isInteger(id) && id > 0) {
      ids.add(id)
    }
  }
  return [...ids].sort((a, b) => a - b)
}
