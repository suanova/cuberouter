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
 * One table filter's value, as the string a request parameter needs.
 *
 * Every organization read endpoint takes one value per field, so a filter held
 * as an array — which is how the shared table state represents a multi-select —
 * contributes its first entry. An empty array, an empty string and an absent
 * filter all mean "not filtering", and all read as `undefined` so a caller can
 * tell a chosen value from a cleared one with a single check.
 */
export function organizationColumnFilterValue(
  columnFilters: Array<{ id: string; value: unknown }>,
  columnId: string
): string | undefined {
  const value = columnFilters.find((filter) => filter.id === columnId)?.value
  if (Array.isArray(value)) {
    const first = value[0]
    return typeof first === 'string' && first ? first : undefined
  }
  return typeof value === 'string' && value ? value : undefined
}

/**
 * Writes one keyword filter, or removes it.
 *
 * A cleared box removes the filter rather than storing an empty string, so the
 * URL stops carrying a parameter the endpoint would read as a filter on nothing.
 * The value is trimmed on the way in for the same reason the endpoints trim
 * what they receive: a box holding a space is not a filter.
 */
export function setOrganizationTextFilter(
  onChange: (filters: Array<{ id: string; value: unknown }>) => void,
  current: Array<{ id: string; value: unknown }>,
  columnId: string,
  value: string
): void {
  const others = current.filter((filter) => filter.id !== columnId)
  const trimmed = value.trim()
  onChange(trimmed ? [...others, { id: columnId, value: trimmed }] : others)
}
