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
// The logo travels to every visitor inside the public /api/status payload, so
// the uploaded image is kept small enough to stay cheap there.
export const MAX_LOGO_IMAGE_BYTES = 200 * 1024

const LOGO_MIME_TYPES = new Set([
  'image/png',
  'image/jpeg',
  'image/webp',
  'image/gif',
  'image/svg+xml',
])

const LOGO_DATA_URL_PATTERN =
  /^data:image\/(?:png|jpeg|webp|gif|svg\+xml);base64,[A-Za-z0-9+/]+={0,2}$/

// Base64 inflates the image by 4/3 and pads to a 4-character boundary; the
// slack covers the data: prefix.
const MAX_LOGO_DATA_URL_LENGTH = Math.ceil(MAX_LOGO_IMAGE_BYTES / 3) * 4 + 64

export function isValidLogoFile(file: File): boolean {
  return (
    LOGO_MIME_TYPES.has(file.type) &&
    file.size > 0 &&
    file.size <= MAX_LOGO_IMAGE_BYTES
  )
}

/**
 * A logo is either an external HTTP(S) URL or an image uploaded through this
 * form, which FileReader stores inline as a base64 data URL.
 */
export function isValidLogoValue(value: string): boolean {
  if (value === '') return true
  if (value.startsWith('data:')) {
    return (
      value.length <= MAX_LOGO_DATA_URL_LENGTH &&
      LOGO_DATA_URL_PATTERN.test(value)
    )
  }

  try {
    const url = new URL(value)
    return url.protocol === 'http:' || url.protocol === 'https:'
  } catch {
    return false
  }
}
