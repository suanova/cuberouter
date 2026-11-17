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
 * The id rule of a Cuby app's cuby-app.json, which the backend enforces too:
 * 2 to 40 lower-case letters, digits or hyphens, not starting with a hyphen.
 * Checked here as well so the administrator sees the reason while typing
 * rather than after submitting.
 */
const CUBY_APP_ID_PATTERN = /^[a-z0-9][a-z0-9-]{1,39}$/

export function isValidCubyAppId(value: string): boolean {
  return CUBY_APP_ID_PATTERN.test(value.trim())
}
