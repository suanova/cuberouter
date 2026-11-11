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
export function domainListToText(value: string): string {
  return value
    .split(',')
    .map((domain) => domain.trim())
    .filter(Boolean)
    .join('\n')
}

export function textToDomainList(value: string): string {
  return value
    .split('\n')
    .map((domain) => domain.trim())
    .filter(Boolean)
    .join(',')
}

export function buildDomainListUpdate(
  key: 'EmailDomainWhitelist' | 'EmailDomainBlacklist',
  value: string,
  original: string
): { key: string; value: string } | null {
  const domains = textToDomainList(value)
  return domains === original ? null : { key, value: domains }
}
