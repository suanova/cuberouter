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
import { api } from '@/lib/api'

import { GENERATION_TIMEOUT_MS } from './constants'
import { extractImages, type ImageResponseBody } from './lib/image-response'
import { imageRequest } from './lib/workflow'
import type {
  StudioAsset,
  WorkflowConfig,
  WorkflowDraft,
  WorkflowJob,
} from './workflow-types'

export function blobDataURL(blob: Blob): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader()
    reader.onload = () => resolve(String(reader.result))
    reader.onerror = () => reject(new Error('Could not read image.'))
    reader.readAsDataURL(blob)
  })
}
export async function referenceAsset(file: Blob): Promise<StudioAsset> {
  if (
    !['image/png', 'image/jpeg', 'image/webp'].includes(file.type) ||
    file.size <= 0 ||
    file.size > 10 * 1024 * 1024
  ) {
    throw new Error('Upload PNG, JPEG or WebP files of at most 10 MB each.')
  }
  return {
    id: crypto.randomUUID(),
    mime: file.type,
    url: await blobDataURL(file),
  }
}
// 上游 data URL 的媒体类型不受控：除 image/png、image/jpeg 外，还出现过
// image/jpg 别名、带 charset 参数、非 base64 的百分号编码，以及
// application/octet-stream。只要不是可执行的文本类型就按自包含资源处理。
const SELF_CONTAINED_IMAGE =
  /^data:(image\/[a-z0-9.+-]+|application\/octet-stream)/i

/** 取 data URL 声明的媒体类型，并把 image/jpg 归一为 image/jpeg。 */
function dataUrlMime(url: string): string {
  const end = url.slice(5).search(/[;,]/)
  const mime = end === -1 ? 'application/octet-stream' : url.slice(5, 5 + end)
  return /^image\/jpg$/i.test(mime) ? 'image/jpeg' : mime
}

// Download only in the browser, without CubeRouter credentials or a server URL proxy.
export async function localImage(url: string): Promise<StudioAsset> {
  if (SELF_CONTAINED_IMAGE.test(url)) {
    return {
      id: crypto.randomUUID(),
      mime: dataUrlMime(url),
      url,
    }
  }
  const parsed = new URL(url, window.location.href)
  if (
    !['https:', 'http:'].includes(parsed.protocol) ||
    parsed.username ||
    parsed.password
  ) {
    throw new Error('Unsupported image URL.')
  }
  const response = await fetch(parsed.href, {
    credentials: 'omit',
    referrerPolicy: 'no-referrer',
  })
  if (!response.ok) {
    throw new Error('Could not download image for local history.')
  }
  const blob = await response.blob()
  if (
    ![
      'image/png',
      'image/jpeg',
      'image/webp',
      'image/gif',
      'image/bmp',
    ].includes(blob.type) ||
    blob.size > 20 * 1024 * 1024
  ) {
    throw new Error('Unsupported image download.')
  }
  return {
    id: crypto.randomUUID(),
    mime: blob.type,
    url: await blobDataURL(blob),
  }
}
async function uploadReference(asset: StudioAsset): Promise<string> {
  const file = await (await fetch(asset.url)).blob()
  if (
    file.size <= 0 ||
    file.size > 10 * 1024 * 1024 ||
    !['image/png', 'image/jpeg', 'image/webp'].includes(file.type)
  ) {
    throw new Error('Upload PNG, JPEG or WebP files of at most 10 MB each.')
  }
  const { data } = await api.post<{
    upload_url: string
    image_url: string
    headers: Record<string, string>
  }>(
    '/api/media-studio/uploads/presign',
    { content_type: file.type, size: file.size },
    { skipErrorHandler: true }
  )
  const response = await fetch(data.upload_url, {
    method: 'PUT',
    headers: data.headers,
    body: file,
    credentials: 'omit',
    referrerPolicy: 'no-referrer',
  })
  if (!response.ok) {
    throw new Error('Reference upload failed. No generation was submitted.')
  }
  return data.image_url
}
export const workflowAPI = {
  config: async (): Promise<WorkflowConfig> =>
    (await api.get('/api/media-studio/config', { skipErrorHandler: true }))
      .data,
  generate: async (
    draft: WorkflowDraft
  ): Promise<{ job: WorkflowJob; warning?: string }> => {
    const started = Date.now()
    const images = []
    if (draft.mode === 'edit') {
      for (const asset of draft.references) {
        images.push(await uploadReference(asset))
      }
    }
    const response = await api.post<ImageResponseBody>(
      `/pg/images/${draft.mode === 'edit' ? 'edits' : 'generations'}`,
      imageRequest(draft, images),
      { timeout: GENERATION_TIMEOUT_MS, skipErrorHandler: true }
    )
    const output = extractImages(response.data)
    if (!output.length) throw new Error('The model returned no images.')
    let warning: string | undefined
    const assets: StudioAsset[] = []
    for (const image of output) {
      try {
        assets.push(await localImage(image.url))
      } catch {
        // Keep a successful provider result usable even when its CORS policy prevents persistence.
        const fallback = new URL(image.url, window.location.href)
        if (
          !['https:', 'http:'].includes(fallback.protocol) ||
          fallback.username ||
          fallback.password
        ) {
          throw new Error('Unsupported image URL.')
        }
        assets.push({
          id: crypto.randomUUID(),
          url: fallback.href,
          mime: 'image/png',
        })
        warning =
          'This result could not be saved locally. Download it before leaving this page.'
      }
    }
    return {
      job: {
        id: crypto.randomUUID(),
        created_at: Date.now(),
        request: structuredClone(draft),
        images: assets,
        elapsed_ms: Date.now() - started,
        request_id: response.headers?.['x-request-id'],
      },
      warning,
    }
  },
}
