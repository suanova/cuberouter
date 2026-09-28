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
import { withBasePath } from '@/lib/base-path'

export function applyFaviconToDom(url: string) {
  if (typeof document === 'undefined' || !url) return
  try {
    // The configured logo is a server path ('/logo.png'), which is only correct
    // on this host when the deployment prefix is included. withBasePath leaves a
    // fully-qualified URL alone, so an off-site logo keeps working.
    const resolved = new URL(withBasePath(url), window.location.href).href
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
