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
import type { GeneratedImage } from '../types'

interface ImageResponseItem {
  url?: unknown
  b64_json?: unknown
  [key: string]: unknown
}

export interface ImageResponseBody {
  data?: unknown
  [key: string]: unknown
}

/**
 * base64 前缀 → MIME 类型。上游（OpenAI 兼容）图片服务可能返回
 * `url` 或 `b64_json` 两种形式；本页面上游（qwen-image 等）返回
 * b64_json，前端需要把它还原成可直接渲染的 data URL。
 */
const B64_MIME_PREFIXES: Array<[string, string]> = [
  ['iVBOR', 'image/png'],
  ['/9j/', 'image/jpeg'],
  ['R0lGOD', 'image/gif'],
  ['UklGR', 'image/webp'],
  ['Qk02', 'image/bmp'],
]

/** 识别 base64 图片的 MIME 类型；无法识别时默认 png（浏览器对 <img> 会嗅探实际内容）。 */
function detectB64Mime(b64: string): string {
  for (const [prefix, mime] of B64_MIME_PREFIXES) {
    if (b64.startsWith(prefix)) {
      return mime
    }
  }
  return 'image/png'
}

/** 将 base64 图片编码转换为 data URL。 */
export function b64ToDataUrl(b64: string): string {
  return `data:${detectB64Mime(b64)};base64,${b64}`
}

/**
 * 把 OpenAI images 响应体中的 data[] 提取为可渲染的图片列表：
 * 优先 url，其次 b64_json（转 data URL）；两者皆无的条目跳过。
 */
export function extractImages(body: ImageResponseBody): GeneratedImage[] {
  const items = Array.isArray(body?.data) ? body.data : []
  const images: GeneratedImage[] = []
  for (const item of items) {
    if (!item || typeof item !== 'object') {
      continue
    }
    const entry = item as ImageResponseItem
    if (typeof entry.url === 'string' && entry.url !== '') {
      images.push({ url: entry.url })
      continue
    }
    if (typeof entry.b64_json === 'string' && entry.b64_json !== '') {
      images.push({ url: b64ToDataUrl(entry.b64_json) })
    }
  }
  return images
}
