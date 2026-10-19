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
 * The configured counterpart site must be an absolute HTTP(S) address.
 *
 * The value ends up in an `<a href>` in every page header, so the scheme is a
 * security boundary, not a formatting preference. `new URL()` happily parses
 * `javascript:`, `data:` and other schemes; React only neutralizes the former
 * (`data:` passes straight through), so anything that is not HTTP(S) is
 * rejected here -- both when the administrator saves it and when the header
 * renders it.
 *
 * True implies `new URL(value)` already succeeded, so callers that passed this
 * check may parse the value without their own try/catch.
 */
export function isValidCounterpartUrl(
  value: string | undefined
): value is string {
  if (!value) return false

  try {
    const url = new URL(value)
    return (
      (url.protocol === 'http:' || url.protocol === 'https:') &&
      url.hostname.length > 0
    )
  } catch {
    return false
  }
}
