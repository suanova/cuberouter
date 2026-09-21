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

/**
 * 本地图片与生成记录的标识。不要直接用 crypto.randomUUID()：该 API 只在安全上下文
 * （HTTPS 或 localhost）暴露，而本项目常部署在 http://<ip>:<port>，此时它是 undefined，
 * 会让造 id 的每一步抛 TypeError，成功生成的图片反而报错显示不出来。
 * crypto.getRandomValues 在非安全上下文同样可用，故作为替代。
 */
function newId(): string {
  if (typeof crypto?.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  const bytes = crypto.getRandomValues(new Uint8Array(16))
  return Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')
}

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
    id: newId(),
    mime: file.type,
    url: await blobDataURL(file),
  }
}
// data URL 是自包含资源：<img> 直接渲染，既不经过网络也不携带凭据。上游声明的
// 媒体类型完全不受控——除 image/png、image/jpeg 外，出现过 image/jpg 别名、带
// charset 参数、binary/octet-stream，以及干脆省略媒体类型的 `data:;base64,`。
// 一旦按类型白名单过滤，这些都能正常显示的结果反而会被判成非法网址，因此这里只
// 判断 scheme：data: 一定自包含，无需再看后面的媒体类型。
const SELF_CONTAINED_URL = /^data:/i

/** 取 data URL 声明的媒体类型（仅用于本地历史的元数据），image/jpg 归一为 image/jpeg。 */
function dataUrlMime(url: string): string {
  const body = url.slice(5)
  const end = body.search(/[;,]/)
  const mime = end === -1 ? body : body.slice(0, end)
  if (!mime.includes('/')) return 'application/octet-stream'
  return /^image\/jpg$/i.test(mime) ? 'image/jpeg' : mime
}

/**
 * 浏览器能直接当作图片加载的地址，用于「显示」而不是「保存」：
 * data:（自包含）、http/https（包括带凭据或没有扩展名的上游网址）、blob:。
 * 相对路径按当前页面解析。只有 javascript: 之类的 scheme 既不能进 <img src>，
 * 放进下载链接还会带执行风险，必须拒绝。
 */
function renderableUrl(url: string): string | null {
  if (SELF_CONTAINED_URL.test(url)) {
    // 原样返回：重新序列化可能改动 base64 里的字符。
    return url
  }
  let parsed: URL
  try {
    parsed = new URL(url, window.location.href)
  } catch {
    return null
  }
  return ['http:', 'https:', 'blob:'].includes(parsed.protocol)
    ? parsed.href
    : null
}

// Download only in the browser, without CubeRouter credentials or a server URL proxy.
// 这个函数比 renderableUrl 严格得多：它要产出可存进 IndexedDB、后续还能上传的
// 自包含资源，因此会拒绝带凭据的网址和无法内联的远程地址。拒绝只影响「能否保存」，
// 调用方必须回退到 renderableUrl 继续显示，不能因此让一次成功的生成失败。
export async function localImage(url: string): Promise<StudioAsset> {
  if (SELF_CONTAINED_URL.test(url)) {
    return {
      id: newId(),
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
    id: newId(),
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
        // 保存失败不影响显示：上游网址只要浏览器能加载就照原样用。CORS、跨域凭据、
        // 非白名单媒体类型都只意味着「存不进本地历史」，不是「这张图不能用」。
        const href = renderableUrl(image.url)
        if (!href) {
          throw new Error('Unsupported image URL.')
        }
        assets.push({
          id: newId(),
          url: href,
          mime: 'image/png',
        })
        warning =
          'This result could not be saved locally. Download it before leaving this page.'
      }
    }
    return {
      job: {
        id: newId(),
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
