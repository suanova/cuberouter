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
import { basePath } from '@/lib/base-path'

/**
 * Base URL for reaching this deployment's API surface: the configured
 * ServerAddress when there is one, this site otherwise. Either way it ends at
 * the deployment prefix, so callers append '/v1/...' directly.
 *
 * ServerAddress is an origin -- scheme, host and port, no path. WebAuthn builds
 * its list of allowed origins from it, and a path there is invalid, so the
 * prefix has to come from the deployment itself rather than from the configured
 * value. Getting this wrong is silent: the link simply points at the site root.
 */
export function resolveServerAddress(
  configured?: unknown,
  fallback = ''
): string {
  const trimmed =
    typeof configured === 'string' ? configured.replace(/\/+$/, '') : ''
  if (trimmed) return `${trimmed}${basePath}`
  if (typeof window === 'undefined') return fallback
  return `${window.location.origin}${basePath}`
}
