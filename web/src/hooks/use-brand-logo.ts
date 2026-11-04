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
import { useSystemConfig } from '@/hooks/use-system-config'
import { withBasePath } from '@/lib/base-path'
import { DEFAULT_LOGO } from '@/lib/constants'

// The bundled wordmark shown whenever no administrator logo is configured.
// mapStatusDataToConfig replaces an empty Logo option with the square app icon
// (DEFAULT_LOGO); brand marks have always drawn this wordmark instead, so it
// stays the fallback here.
export const DEFAULT_BRAND_LOGO = withBasePath('/head.png')

/**
 * Resolve the brand mark shown in headers, footers and the auth screens.
 *
 * Returns the administrator-configured logo when one is set, otherwise the
 * bundled wordmark. `recolorForDark` is true only for the bundled asset: it is
 * recoloured to white on dark surfaces via CSS, while an uploaded image is
 * shown untouched because we cannot know whether it has a dark variant.
 */
export function useBrandLogo() {
  const { logo } = useSystemConfig()
  const isCustom = Boolean(logo) && logo !== DEFAULT_LOGO

  return {
    logo: isCustom ? logo : DEFAULT_BRAND_LOGO,
    recolorForDark: !isCustom,
  }
}
