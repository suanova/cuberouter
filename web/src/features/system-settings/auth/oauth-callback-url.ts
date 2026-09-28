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
import { basePath } from '@/lib/base-path'

export function resolveOAuthSiteUrl(
  serverAddress: string,
  fallback: string
): string {
  const normalized = serverAddress.trim().replace(/\/+$/, '')
  return normalized || fallback
}

/**
 * Callback URL an administrator registers with the identity provider, so it has
 * to be the URL the provider will actually reach. ServerAddress is an origin by
 * contract (WebAuthn builds its allowed origins from it), hence the deployment
 * prefix is applied to the callback path rather than read from the configured
 * value.
 */
export function buildOAuthCallbackUrl(
  serverAddress: string,
  callbackPath: string,
  fallback: string
): string {
  const siteUrl = resolveOAuthSiteUrl(serverAddress, fallback)
  return `${siteUrl}${basePath}/oauth/${callbackPath.replace(/^\/+/, '')}`
}
