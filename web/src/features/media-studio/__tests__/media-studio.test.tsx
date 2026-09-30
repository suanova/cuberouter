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
import { beforeEach, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { MediaStudio } from '../index'
import { deleteStudioJob, listStudioJobs } from '../lib/studio-storage'

const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))
beforeEach(async () => {
  vi.clearAllMocks()
  await deleteStudioJob(301)
  useAuthStore
    .getState()
    .auth.setUser({ id: 301, username: 'reviewer', role: 1 })
  http.get.mockImplementation(async () => ({
    data: {
      success: true,
      data: {
        pricings: [
          {
            model_name: 'image-model',
            tags: 'text-to-image',
          },
          {
            // 带 image-generation 端点类型但只声明 image-to-image：只支持编辑的
            // 模型不该出现在文生图列表里。
            model_name: 'edit-model',
            tags: 'image-to-image',
            supported_endpoint_types: ['image-generation'],
          },
          {
            model_name: 'chat-model',
            tags: 'chat',
            supported_endpoint_types: ['openai'],
          },
        ],
      },
    },
  }))
  http.post.mockResolvedValue({
    data: { created: 7, data: [{ b64_json: 'iVBORw0KGgoAAAAB' }] },
    headers: { 'x-request-id': 'req-1' },
  })
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
test('standard channel generation saves actual bytes locally and reloads history', async () => {
  const view = page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  expect(
    screen.queryByRole('option', { name: 'chat-model' })
  ).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Model'), {
    target: { value: 'image-model' },
  })
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  expect(http.post).toHaveBeenCalledWith(
    '/pg/images/generations',
    {
      model: 'image-model',
      prompt: 'A cat',
      n: 1,
      size: '1024x1024',
      quality: 'standard',
      num_inference_steps: 30,
      seed: 42,
      true_cfg_scale: 4,
    },
    expect.anything()
  )
  await waitFor(async () => expect(await listStudioJobs(301)).toHaveLength(1))
  view.unmount()
  page()
  fireEvent.click(screen.getByRole('button', { name: 'Result' }))
  await screen.findByRole('button', { name: 'Open creation: A cat' })
})
test('a reference attached in the composer reaches the relay as inline base64', async () => {
  // 图生图的成败全在这条契约上：参考图必须以 data URL 原样进 `image` 字段，且整个
  // 提交过程不碰对象存储。上传功能没有配置也不影响（见 workflow-ui.test.tsx）。
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  // 「Image to image」在模版图库的筛选条里也有一个同名按钮，必须限定在写作台的
  // 模式切换组里点，否则拿到的是图库的筛选。
  fireEvent.click(
    within(screen.getByLabelText('Creation mode')).getByRole('button', {
      name: 'Image to image',
    })
  )
  fireEvent.change(screen.getByLabelText('Upload reference images'), {
    target: {
      files: [new File(['PNG'], 'reference.png', { type: 'image/png' })],
    },
  })
  await screen.findByAltText('Reference image')
  fireEvent.change(screen.getByLabelText('Describe your changes'), {
    target: { value: 'Blue coat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  expect(http.post).toHaveBeenCalledTimes(1)
  expect(http.post).toHaveBeenCalledWith(
    '/pg/images/edits',
    expect.objectContaining({
      model: 'edit-model',
      prompt: 'Blue coat',
      // data URL 前缀之外是 FileReader 对 'PNG' 的 base64 编码。
      image: 'data:image/png;base64,UE5H',
    }),
    expect.anything()
  )
  expect(
    http.post.mock.calls.some(([path]) => String(path).includes('presign'))
  ).toBe(false)
})
test('continuing from a generated image preserves the original and switches to reference editing', async () => {
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  fireEvent.click(screen.getByRole('button', { name: 'Continue editing' }))
  await screen.findByAltText('Reference image')
  expect(screen.getByLabelText('Model')).toHaveValue('edit-model')
  expect(
    screen.getByText('Editing a previous version. The original is preserved.')
  ).toBeInTheDocument()
  expect(await listStudioJobs(301)).toHaveLength(1)
})
test('generation failure exposes the error and never creates a successful history entry', async () => {
  http.post.mockRejectedValue(new Error('Provider unavailable'))
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'Cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByText('Provider unavailable')
  expect(await listStudioJobs(301)).toEqual([])
  expect(http.post).toHaveBeenCalledTimes(1)
})

test('switching signed-in accounts clears the previous account result and history view', async () => {
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'Account 301 cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  await act(async () => {
    useAuthStore
      .getState()
      .auth.setUser({ id: 302, username: 'another', role: 1 })
  })
  expect(screen.queryByAltText('Generated image')).not.toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Result' }))
  await screen.findByText('No generations in this browser yet.')
  expect(
    screen.queryByRole('button', { name: 'Open creation: Account 301 cat' })
  ).not.toBeInTheDocument()
})
test('the in-progress view reports progress without exposing the request payload', async () => {
  let release: (value: unknown) => void = () => {}
  http.post.mockImplementation(
    () =>
      new Promise((resolve) => {
        release = resolve
      })
  )
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  expect(await screen.findByText('Generating 1 image…')).toBeInTheDocument()
  expect(
    screen.getByText(
      'Generation is synchronous and usually takes 40 seconds to 5 minutes. Keep this page open.'
    )
  ).toBeInTheDocument()
  expect(document.querySelector('pre')).toBeNull()
  await act(async () => {
    release({
      data: { created: 7, data: [{ b64_json: 'iVBORw0KGgoAAAAB' }] },
      headers: {},
    })
  })
  await screen.findByAltText('Generated image')
})
test('the result view stacks the image preview above the creation history', async () => {
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  // 结果与历史合并后，导航只剩「模版图库」和「结果」两个入口。
  expect(
    screen.queryByRole('button', { name: 'Creation history' })
  ).not.toBeInTheDocument()
  const preview = screen.getByRole('region', { name: 'Image preview' })
  const history = screen.getByRole('region', { name: 'Creation history' })
  expect(preview.compareDocumentPosition(history)).toBe(
    Node.DOCUMENT_POSITION_FOLLOWING
  )
  await screen.findByRole('button', { name: 'Open creation: A cat' })
})
test('the local history note is shown on the merged result view only', async () => {
  const note = /^History is stored in this browser/
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  expect(screen.queryByText(note)).not.toBeInTheDocument()
  fireEvent.change(screen.getByLabelText('Prompt'), {
    target: { value: 'A cat' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))
  await screen.findByAltText('Generated image')
  expect(screen.getByText(note)).toBeInTheDocument()
  fireEvent.click(screen.getByRole('button', { name: 'Template gallery' }))
  expect(screen.queryByText(note)).not.toBeInTheDocument()
})
