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
import 'fake-indexeddb/auto'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import {
  act,
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { MediaStudio } from '../index'
import { deleteVideoJob, listVideoJobs } from '../lib/video-storage'

const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))

const VIDEO_URL = 'https://cdn.example/generated.mp4'
const PRICING = {
  success: true,
  data: {
    pricings: [
      { model_name: 'image-model', tags: 'text-to-image' },
      { model_name: 'viduq3-pro', tags: 'text-to-video' },
      { model_name: 'kling-v2', tags: 'image-to-video' },
    ],
  },
}

function taskResponse(status: string, extra: Record<string, unknown> = {}) {
  return {
    data: {
      code: 'success',
      data: {
        task_id: 'task_v1',
        status,
        progress: status === 'SUCCESS' ? '100%' : '40%',
        ...extra,
      },
    },
  }
}

let taskResponses: unknown[] = []
const videoBytes = new Uint8Array([9, 9, 9])

beforeEach(async () => {
  vi.clearAllMocks()
  taskResponses = []
  window.localStorage.removeItem('cuberouter-video-pending-401')
  // fake-indexeddb 是文件内共享的内存库：先清掉上个用例留下的视频，
  // 保证每个用例都从空历史开始（此时是真实计时，事件正常投递）。
  await deleteVideoJob(401)
  http.get.mockImplementation(async (url: string) => {
    if (url === '/api/pricing') return { data: PRICING }
    if (url.startsWith('/pg/video/generations/task_')) {
      return (taskResponses.shift() ??
        taskResponse('SUCCESS', { result_url: VIDEO_URL })) as {
        data: unknown
      }
    }
    throw new Error(`unexpected GET ${url}`)
  })
  http.post.mockImplementation(async (url: string) => {
    if (url === '/pg/video/generations') {
      return { data: { task_id: 'task_v1', status: 'queued' } }
    }
    throw new Error(`unexpected POST ${url}`)
  })
  vi.stubGlobal(
    'fetch',
    vi.fn(async () => ({
      ok: true,
      blob: async () => new Blob([videoBytes], { type: 'video/mp4' }),
    }))
  )
  // jsdom 没有 URL.createObjectURL；视频字节显示走 object URL，这里替换成可断言的假地址。
  Object.defineProperty(URL, 'createObjectURL', {
    value: vi.fn(() => 'blob:mock-video'),
    configurable: true,
    writable: true,
  })
  Object.defineProperty(URL, 'revokeObjectURL', {
    value: vi.fn(),
    configurable: true,
    writable: true,
  })
  useAuthStore
    .getState()
    .auth.setUser({ id: 401, username: 'video-reviewer', role: 1 })
})

afterEach(() => {
  vi.useRealTimers()
  vi.unstubAllGlobals()
})

function page() {
  const query = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={query}>
      <MediaStudio />
    </QueryClientProvider>
  )
}

/** 真实计时阶段：渲染、等目录加载、切到视频 tab（返回 render 结果便于卸载）。 */
async function openVideoTab() {
  const result = page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  fireEvent.click(
    within(screen.getByRole('navigation', { name: 'Media types' })).getByRole(
      'button',
      { name: 'Video' }
    )
  )
  return result
}

test('defaults to the image tab and switches to the video composer', async () => {
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())

  const tabs = screen.getByRole('navigation', { name: 'Media types' })
  expect(within(tabs).getByRole('button', { name: 'Images' })).toHaveAttribute(
    'aria-pressed',
    'true'
  )
  expect(
    within(tabs).getByRole('button', { name: 'Video' })
  ).toHaveAttribute('aria-pressed', 'false')
  expect(
    screen.queryByRole('button', { name: 'Generate video' })
  ).not.toBeInTheDocument()

  fireEvent.click(within(tabs).getByRole('button', { name: 'Video' }))
  expect(screen.getByRole('button', { name: 'Generate video' })).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Generate image' })
  ).not.toBeInTheDocument()
  expect(
    within(screen.getByLabelText('Video mode')).getByRole('button', {
      name: 'Text to video',
    })
  ).toHaveAttribute('aria-pressed', 'true')

  fireEvent.click(within(tabs).getByRole('button', { name: 'Images' }))
  expect(screen.getByRole('button', { name: 'Generate image' })).toBeInTheDocument()
  expect(
    screen.queryByRole('button', { name: 'Generate video' })
  ).not.toBeInTheDocument()
})

test('text to video submits the unified task body, polls and renders the finished video', async () => {
  taskResponses = [
    taskResponse('IN_PROGRESS'),
    taskResponse('SUCCESS', { result_url: VIDEO_URL }),
  ]
  await openVideoTab()
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A rabbit on the beach' },
  })
  vi.useFakeTimers()

  fireEvent.click(screen.getByRole('button', { name: 'Generate video' }))
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  expect(http.post).toHaveBeenCalledWith(
    '/pg/video/generations',
    {
      model: 'viduq3-pro',
      prompt: 'A rabbit on the beach',
      duration: 5,
      size: '720p',
    },
    expect.anything()
  )
  // 第一次轮询仍在生成：显示进度，不出视频。
  expect(screen.getByRole('status')).toHaveTextContent('Generating video…')
  expect(screen.queryByLabelText('Generated video')).not.toBeInTheDocument()
  // 进行中的任务按账号持久化，页面关闭重开后可恢复轮询。
  expect(
    window.localStorage.getItem('cuberouter-video-pending-401')
  ).toContain('task_v1')

  await act(async () => {
    await vi.advanceTimersByTimeAsync(5000)
  })
  expect(screen.getByLabelText('Generated video')).toHaveAttribute(
    'src',
    'blob:mock-video'
  )
  expect(global.fetch).toHaveBeenCalledWith(
    VIDEO_URL,
    expect.objectContaining({ credentials: 'omit' })
  )

  // 生成结束后待恢复记录清除。
  expect(
    window.localStorage.getItem('cuberouter-video-pending-401')
  ).toBeNull()

  // fake-indexeddb 通过定时器投递 IndexedDB 事件（且落库事务会串行化后续
  // 读取），假计时下要先发起读取再推进定时器，否则会永久等待。
  const jobsPromise = listVideoJobs(401)
  await act(async () => {
    await vi.advanceTimersByTimeAsync(100)
  })
  const jobs = await jobsPromise
  expect(jobs).toHaveLength(1)
  expect(jobs[0].request.prompt).toBe('A rabbit on the beach')
})

test('resumes polling of a pending task after the page is reopened', async () => {
  taskResponses = [taskResponse('IN_PROGRESS')]
  const first = await openVideoTab()
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A rabbit' },
  })
  vi.useFakeTimers()

  fireEvent.click(screen.getByRole('button', { name: 'Generate video' }))
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  // 进行中任务已持久化，此时「关闭页面」。
  expect(
    window.localStorage.getItem('cuberouter-video-pending-401')
  ).toContain('task_v1')
  first.unmount()

  // 重开页面：全新 query client，localStorage 里还有待恢复记录。
  // 恢复后 busy=true 会禁用模型下拉，所以直接切 tab 不等目录加载；
  // 恢复流程与终态落库都是微任务/零时延定时器，全程保持假计时，
  // 用「先发起读取再推进定时器」的方式读历史（与上面用例一致）。
  taskResponses = [taskResponse('SUCCESS', { result_url: VIDEO_URL })]
  const second = page()
  fireEvent.click(
    within(screen.getByRole('navigation', { name: 'Media types' })).getByRole(
      'button',
      { name: 'Video' }
    )
  )
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  // 挂载后自动恢复轮询：任务接口被再次请求，结果直接渲染。
  expect(
    http.get.mock.calls.some(
      ([url]) => url === '/pg/video/generations/task_v1'
    )
  ).toBe(true)
  expect(screen.getByLabelText('Generated video')).toBeInTheDocument()
  // 终态处理完毕：待恢复记录清除。
  expect(
    window.localStorage.getItem('cuberouter-video-pending-401')
  ).toBeNull()

  // 先让落库事务单独走完再读：fake-indexeddb 下与 save 的 commit 落在同一
  // 计时批次的读取可能拿到更早的快照（真实浏览器的按存储序列化不会）。
  await act(async () => {
    await vi.advanceTimersByTimeAsync(100)
  })
  const jobsPromise = listVideoJobs(401)
  await act(async () => {
    await vi.advanceTimersByTimeAsync(100)
  })
  const jobs = await jobsPromise
  expect(jobs).toHaveLength(1)
  expect(jobs[0].request.prompt).toBe('A rabbit')
  second.unmount()
})

test('image to video submits the first frame as an inline data URL', async () => {
  taskResponses = [taskResponse('SUCCESS', { result_url: VIDEO_URL })]
  await openVideoTab()
  fireEvent.click(
    within(screen.getByLabelText('Video mode')).getByRole('button', {
      name: 'Image to video',
    })
  )
  // 没有首帧图时不能提交。
  expect(screen.getByRole('button', { name: 'Generate video' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Upload a first frame image'), {
    target: { files: [new File(['PNG'], 'first.png', { type: 'image/png' })] },
  })
  await screen.findByAltText('First frame image')
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'Walk into the scene' },
  })
  vi.useFakeTimers()

  fireEvent.click(screen.getByRole('button', { name: 'Generate video' }))
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  expect(http.post).toHaveBeenCalledWith(
    '/pg/video/generations',
    {
      model: 'kling-v2',
      prompt: 'Walk into the scene',
      duration: 5,
      size: '720p',
      // data URL 前缀之外是 FileReader 对 'PNG' 的 base64 编码。
      image: 'data:image/png;base64,UE5H',
    },
    expect.anything()
  )

  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  expect(screen.getByLabelText('Generated video')).toBeInTheDocument()
  expect(screen.getByText('First frame comparison')).toBeInTheDocument()
})

test('a failed task stops polling and surfaces the upstream fail reason', async () => {
  taskResponses = [
    taskResponse('FAILURE', { fail_reason: 'prompt rejected by upstream' }),
  ]
  await openVideoTab()
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'Something' },
  })
  vi.useFakeTimers()

  fireEvent.click(screen.getByRole('button', { name: 'Generate video' }))
  await act(async () => {
    await vi.advanceTimersByTimeAsync(0)
  })
  // status 角色的可访问名称只来自作者属性（aria-label），不含内容，
  // 所以按 role 定位后断言文本。
  expect(screen.getByRole('status')).toHaveTextContent(
    'prompt rejected by upstream'
  )
  expect(screen.queryByLabelText('Generated video')).not.toBeInTheDocument()
  // 失败终态同样清除待恢复记录，重开页面不会反复弹这个错误。
  expect(
    window.localStorage.getItem('cuberouter-video-pending-401')
  ).toBeNull()

  const getCallCount = http.get.mock.calls.filter(([url]) =>
    String(url).startsWith('/pg/video/generations/task_')
  ).length
  await act(async () => {
    await vi.advanceTimersByTimeAsync(15000)
  })
  // 终态之后不再轮询。
  expect(
    http.get.mock.calls.filter(([url]) =>
      String(url).startsWith('/pg/video/generations/task_')
    ).length
  ).toBe(getCallCount)
})

test('keeps image generation working from the image tab', async () => {
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  expect(
    screen.queryByLabelText('Upload a first frame image')
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A cat' },
  })
  http.post.mockImplementation(async (url: string) => {
    if (url === '/pg/images/generations') {
      return {
        data: { created: 7, data: [{ b64_json: 'iVBORw0KGgoAAAAB' }] },
        headers: { 'x-request-id': 'req-1' },
      }
    }
    throw new Error(`unexpected POST ${url}`)
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  expect(http.post).toHaveBeenCalledWith(
    '/pg/images/generations',
    expect.objectContaining({ model: 'image-model' }),
    expect.anything()
  )
})
