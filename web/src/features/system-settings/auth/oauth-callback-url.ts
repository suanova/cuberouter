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
 * to be the URL the provider will actually reach. The server composes that URL
 * from ServerAddress and nothing else -- oauth/oidc.go has no notion of
 * BASE_PATH -- so a deployment published under a prefix must carry it in the
 * configured value; adding it here as well would double it. The prefix still
 * applies to the fallback, which is this site's own address.
 */
export function buildOAuthCallbackUrl(
  serverAddress: string,
  callbackPath: string,
  fallback: string
): string {
  const configured = serverAddress.trim().replace(/\/+$/, '')
  const siteUrl = configured || `${fallback}${basePath}`
  return `${siteUrl}/oauth/${callbackPath.replace(/^\/+/, '')}`
}
