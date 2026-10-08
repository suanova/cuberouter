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
 * A configured ServerAddress is the complete public base and may already carry
 * the deployment prefix -- the same value the server uses to build capability,
 * OAuth redirect and task URLs, none of which know about BASE_PATH. It is
 * therefore used as-is; the prefix is only added to the fallback, where the
 * origin comes from window.location and carries no path. Adding it to a
 * configured value doubles the prefix (.../cuberouter/cuberouter/v1/...) for
 * every deployment that publishes under one.
 */
export function resolveServerAddress(
  configured?: unknown,
  fallback = ''
): string {
  const trimmed =
    typeof configured === 'string' ? configured.trim().replace(/\/+$/, '') : ''
  if (trimmed) return trimmed
  if (typeof window === 'undefined') return fallback
  return `${window.location.origin}${basePath}`
}
