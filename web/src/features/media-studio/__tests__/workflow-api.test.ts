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
import { beforeEach, afterEach, expect, test, vi } from 'vitest'

import { initialDraft } from '../lib/workflow'
import { workflowAPI } from '../workflow-api'

const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))
const nativeFetch = globalThis.fetch
const upload = 'https://objects.example/upload?X-Amz-Signature=upload-signature'
const download = 'https://objects.example/upload?X-Amz-Signature=read-signature'
const reference = {
  id: 'photo',
  url: 'data:image/png;base64,iVBORw0KGgoAAAAB',
  mime: 'image/png',
}
const draft = {
  ...initialDraft,
  mode: 'edit' as const,
  model: 'image-edit',
  prompt: 'Blue coat',
  references: [reference],
}
beforeEach(() => {
  vi.clearAllMocks()
  http.post.mockImplementation(async (path: string) =>
    path.endsWith('/presign')
      ? {
          data: {
            upload_url: upload,
            image_url: download,
            headers: { 'Content-Type': 'image/png' },
          },
        }
      : {
          data: { data: [{ b64_json: 'iVBORw0KGgoAAAAB' }] },
          headers: { 'x-request-id': 'request-1' },
        }
  )
})
afterEach(() => vi.unstubAllGlobals())
test('reference uploads use signed PUT without dashboard credentials and edits use the existing relay', async () => {
  const fetcher = vi.fn(
    async (input: RequestInfo | URL, options?: RequestInit) =>
      String(input).startsWith('data:')
        ? nativeFetch(input, options)
        : new Response('', { status: 200 })
  )
  vi.stubGlobal('fetch', fetcher)
  const { job } = await workflowAPI.generate(draft)
  expect(http.post.mock.calls[0]).toEqual([
    '/api/media-studio/uploads/presign',
    { content_type: 'image/png', size: 12 },
    { skipErrorHandler: true },
  ])
  expect(fetcher).toHaveBeenCalledWith(
    upload,
    expect.objectContaining({
      method: 'PUT',
      headers: { 'Content-Type': 'image/png' },
      credentials: 'omit',
      referrerPolicy: 'no-referrer',
    })
  )
  expect(http.post).toHaveBeenLastCalledWith(
    '/pg/images/edits',
    expect.objectContaining({
      model: 'image-edit',
      images: [{ image_url: download }],
    }),
    expect.anything()
  )
  expect(job.images[0].url).toBe(reference.url)
  expect(JSON.stringify(job)).not.toContain('X-Amz-Signature')
  expect(job.request_id).toBe('request-1')
})
test('failed upload prevents a billable image edit request', async () => {
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, options?: RequestInit) =>
      String(input).startsWith('data:')
        ? nativeFetch(input, options)
        : new Response('', { status: 403 })
    )
  )
  await expect(workflowAPI.generate(draft)).rejects.toThrow(
    'No generation was submitted'
  )
  expect(http.post).toHaveBeenCalledTimes(1)
})
test('a provider URL blocked by CORS remains visible with a local-persistence warning', async () => {
  http.post.mockResolvedValue({
    data: { data: [{ url: 'https://provider.example/result.png' }] },
    headers: {},
  })
  vi.stubGlobal(
    'fetch',
    vi.fn().mockRejectedValue(new TypeError('Failed to fetch'))
  )
  const output = await workflowAPI.generate({
    ...initialDraft,
    model: 'image',
    prompt: 'Cat',
  })
  expect(output.job.images[0].url).toBe('https://provider.example/result.png')
  expect(output.warning).toContain('could not be saved locally')
  expect(http.post).toHaveBeenCalledTimes(1)
})
test('provider data URLs still render when the media type is an alias, carries parameters, or is absent', async () => {
  http.post.mockResolvedValue({
    data: {
      data: [
        { url: 'data:image/jpg;base64,/9j/4AAQSkZJRg==' },
        { url: 'data:image/webp;charset=utf-8;base64,UklGRg==' },
        { url: 'data:;base64,iVBORw0KGgo=' },
        { url: 'data:binary/octet-stream;base64,iVBORw0KGgo=' },
      ],
    },
  })
  const output = await workflowAPI.generate({
    ...initialDraft,
    model: 'image',
    prompt: 'Cat',
  })
  expect(output.job.images.map((asset) => asset.mime)).toEqual([
    'image/jpeg',
    'image/webp',
    'application/octet-stream',
    'binary/octet-stream',
  ])
  expect(output.warning).toBeUndefined()
})
test('a provider URL that cannot be inlined locally is still rendered', async () => {
  // 带凭据的网址会被 localImage 直接拒绝（不会发起请求），但没有理由因此让整次
  // 生成失败——浏览器照样能把它加载成图片。
  const fetcher = vi.fn()
  vi.stubGlobal('fetch', fetcher)
  http.post.mockResolvedValue({
    data: { data: [{ url: 'https://user:token@cdn.example/result.png' }] },
    headers: {},
  })
  const output = await workflowAPI.generate({
    ...initialDraft,
    model: 'image',
    prompt: 'Cat',
  })
  expect(output.job.images[0].url).toBe(
    'https://user:token@cdn.example/result.png'
  )
  expect(output.warning).toContain('could not be saved locally')
  expect(fetcher).not.toHaveBeenCalled()
  expect(http.post).toHaveBeenCalledTimes(1)
})
test('a non-secure origin without crypto.randomUUID still returns renderable images', async () => {
  // 部署在 http://<ip>:<port> 时页面不是安全上下文，crypto.randomUUID 为 undefined。造本地
  // id 的每一步都会因此抛 TypeError，随即被 catch 误判成不支援该网址，导致每张图都失败。
  const webcrypto = globalThis.crypto
  vi.stubGlobal('crypto', {
    getRandomValues: webcrypto.getRandomValues.bind(webcrypto),
  })
  http.post.mockResolvedValue({
    data: { data: [{ url: null, b64_json: 'iVBORw0KGgoAAAAB' }] },
  })
  const output = await workflowAPI.generate({
    ...initialDraft,
    model: 'image',
    prompt: 'Cat',
  })
  expect(output.job.images[0].url).toBe('data:image/png;base64,iVBORw0KGgoAAAAB')
  expect(output.job.images[0].id).toBeTruthy()
  expect(output.warning).toBeUndefined()
})
test('a non-secure origin can upload a reference and inline a downloaded result', async () => {
  const webcrypto = globalThis.crypto
  vi.stubGlobal('crypto', {
    getRandomValues: webcrypto.getRandomValues.bind(webcrypto),
  })
  vi.stubGlobal(
    'fetch',
    vi.fn(async (input: RequestInfo | URL, options?: RequestInit) => {
      if (String(input) === 'https://provider.example/result.png') {
        return new Response('PNG', {
          status: 200,
          headers: { 'Content-Type': 'image/png' },
        })
      }
      if (String(input).startsWith('data:')) {
        return nativeFetch(input, options)
      }
      return new Response('', { status: 200 })
    })
  )
  http.post.mockImplementation(async (path: string) =>
    path.endsWith('/presign')
      ? {
          data: {
            upload_url: upload,
            image_url: download,
            headers: { 'Content-Type': 'image/png' },
          },
        }
      : { data: { data: [{ url: 'https://provider.example/result.png' }] } }
  )
  const output = await workflowAPI.generate(draft)
  expect(output.warning).toBeUndefined()
  expect(output.job.images[0].url).toMatch(/^data:image\/png;base64,/)
  expect(output.job.request.references[0].id).toBeTruthy()
})
test('unsupported provider URLs are never rendered as successful images', async () => {
  http.post.mockResolvedValue({
    data: { data: [{ url: 'javascript:alert(1)' }] },
  })
  await expect(
    workflowAPI.generate({ ...initialDraft, model: 'image', prompt: 'Cat' })
  ).rejects.toThrow('Unsupported image URL')
})
