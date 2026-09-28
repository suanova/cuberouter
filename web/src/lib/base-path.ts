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
declare global {
  interface Window {
    /**
     * URL path prefix the dashboard is published under, injected into the entry
     * HTML by the server from its BASE_PATH environment variable. Undefined in
     * tests and on the dev server, where the app is served from the site root.
     */
    __BASE_PATH__?: string
  }
}

/**
 * Reduce the injected prefix to its canonical form: either '' (served from the
 * site root) or '/prefix' with no trailing slash.
 */
function normalizeBasePath(raw: string | undefined): string {
  if (typeof raw !== 'string') return ''
  const trimmed = raw.replaceAll(/^\/+|\/+$/g, '')
  return trimmed === '' ? '' : `/${trimmed}`
}

/**
 * Deployment prefix, resolved once at startup from the value the server
 * injected into the entry HTML. Empty string means the app is served from the
 * site root, which is the default for local development and every existing
 * deployment.
 */
export const basePath: string = normalizeBasePath(
  typeof window === 'undefined' ? undefined : window.__BASE_PATH__
)

// A URL that carries its own origin: scheme-qualified (https:, data:, blob:) or
// protocol-relative (//host). Prefixing those would corrupt them.
const absoluteUrlPattern = /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i

/**
 * Resolve an app-owned root-absolute path against the deployment prefix, so a
 * dashboard published under a URL path prefix reaches its own assets, API and
 * routes instead of the site root.
 *
 * The path is returned unchanged when no prefix is configured, when it is an
 * absolute URL (an administrator may point status.docs_link at an external
 * site), or when it already starts with the prefix. That last case matters
 * because some callers derive paths from window.location.pathname, which
 * already carries the prefix.
 */
export function withBasePath(path: string): string {
  if (basePath === '' || path === '') return path
  if (absoluteUrlPattern.test(path)) return path
  if (path === basePath || path.startsWith(`${basePath}/`)) return path
  return `${basePath}${path.startsWith('/') ? '' : '/'}${path}`
}

/**
 * Absolute URL of an app-owned path on this deployment, deployment prefix
 * included. Use this instead of `${window.location.origin}${path}` when the
 * result leaves the SPA -- an OAuth redirect_uri, a shared link, a link opened
 * in a new tab.
 *
 * window.location.origin excludes the path, so building the URL from it drops
 * the prefix and points the recipient at the site root. A path that is already
 * a full URL wins over the origin, so callers can pass either.
 */
export function absoluteAppUrl(path: string): string {
  if (typeof window === 'undefined') return withBasePath(path)
  return new URL(withBasePath(path), window.location.origin).href
}
