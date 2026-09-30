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
import { normalizeReferences } from '../lib/reference-image'
import { workflowAPI } from '../workflow-api'

const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))
// 归一化的真实行为依赖 canvas，这里只在模块边界上换成 spy：没有单独指定的用例仍然跑
// 真实实现（jsdom 下就是原样返回，正是部署到无 canvas 环境的降级路径）。
vi.mock('../lib/reference-image', { spy: true })
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
  http.post.mockResolvedValue({
    data: { data: [{ b64_json: 'iVBORw0KGgoAAAAB' }] },
    headers: { 'x-request-id': 'request-1' },
  })
})
afterEach(() => vi.unstubAllGlobals())
test('edits send the reference inline as base64 without an object-store round trip', async () => {
  // 编辑渠道只认内联 base64：参考图在 draft 里本来就是 data URL，原样下发即可。
  // presign → PUT → provider GET 整条链路退出请求路径，浏览器一次 fetch 都不该发。
  const fetcher = vi.fn()
  vi.stubGlobal('fetch', fetcher)
  const { job } = await workflowAPI.generate(draft)
  expect(fetcher).not.toHaveBeenCalled()
  expect(http.post).toHaveBeenCalledTimes(1)
  expect(http.post).toHaveBeenCalledWith(
    '/pg/images/edits',
    expect.objectContaining({
      model: 'image-edit',
      image: reference.url,
    }),
    expect.anything()
  )
  expect(job.images[0].url).toBe(reference.url)
  expect(JSON.stringify(job)).not.toContain('X-Amz-Signature')
  expect(job.request_id).toBe('request-1')
})
test('several references are sent as an array of base64 strings', async () => {
  const second = { ...reference, id: 'photo-2', url: 'data:image/png;base64,YQ==' }
  await workflowAPI.generate({ ...draft, references: [reference, second] })
  expect(http.post).toHaveBeenCalledWith(
    '/pg/images/edits',
    expect.objectContaining({ image: [reference.url, second.url] }),
    expect.anything()
  )
})
test('text-to-image requests never carry a reference field', async () => {
  await workflowAPI.generate({ ...initialDraft, model: 'image', prompt: 'Cat' })
  expect(http.post.mock.calls[0][1]).not.toHaveProperty('image')
})
test('an edit sends the normalized reference while the saved job keeps the uploaded bytes', async () => {
  // 归一化只作用于下发的那一份：draft 与本地历史留的仍是用户原始字节，「原图对照」
  // 面板、下载和历史都不受影响。
  const normalized = 'data:image/webp;base64,UklGRg=='
  vi.mocked(normalizeReferences).mockResolvedValue([normalized])
  const { job } = await workflowAPI.generate(draft)
  expect(http.post).toHaveBeenCalledWith(
    '/pg/images/edits',
    expect.objectContaining({ image: normalized }),
    expect.anything()
  )
  expect(job.request.references[0].url).toBe(reference.url)
})
test('text-to-image requests never normalize a reference', async () => {
  // 文生图没有参考图，不必白跑一次解码与编码。
  await workflowAPI.generate({ ...initialDraft, model: 'image', prompt: 'Cat' })
  expect(normalizeReferences).not.toHaveBeenCalled()
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
test('a non-secure origin can inline a reference and a downloaded result', async () => {
  const webcrypto = globalThis.crypto
  vi.stubGlobal('crypto', {
    getRandomValues: webcrypto.getRandomValues.bind(webcrypto),
  })
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => {
      // 直接给出带媒体类型的 blob：各运行时把 Response 头映射到 blob.type 的行为并不一致，
      // 这里要验证的是「下载到的 png 能被内联」，不是运行时的头解析。
      return {
        ok: true,
        blob: async () => new Blob(['PNG'], { type: 'image/png' }),
      }
    })
  )
  http.post.mockResolvedValue({
    data: { data: [{ url: 'https://provider.example/result.png' }] },
  })
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
