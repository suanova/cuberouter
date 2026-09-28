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
/**
 * Point the document favicon at `url`.
 *
 * `url` must already be usable as-is: an absolute URL, or a root-relative path
 * that includes the deployment prefix. Callers holding a path straight from the
 * server resolve it through withBasePath first (see use-system-config.ts and
 * main.tsx); doing it again in here would double-apply the prefix whenever the
 * caller had already resolved it.
 */
export function applyFaviconToDom(url: string) {
  if (typeof document === 'undefined' || !url) return
  try {
    // Resolve to an absolute URL purely so the dedupe check below can compare it
    // against an existing <link>'s href, which the browser always reports as
    // absolute. withBasePath is not applied here -- see the note above.
    const resolved = new URL(url, window.location.href).href
    const existing =
      document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]')
    if (existing.length === 1 && existing[0].href === resolved) return
    const link = document.createElement('link')
    link.rel = 'icon'
    // Assign the resolved URL: the dedupe check above compares against it, so
    // assigning the raw input would leave an unreachable early return and
    // re-append the icon on every call.
    link.href = resolved
    existing.forEach((l) => l.remove())
    document.head.appendChild(link)
  } catch {
    // Ignore malformed URLs
  }
}
