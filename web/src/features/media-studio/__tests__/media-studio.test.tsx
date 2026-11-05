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
import userEvent from '@testing-library/user-event'
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

test('the gallery opens on Templates and switches to Qwen examples with the keyboard', async () => {
  const user = userEvent.setup()
  page()
  const gallery = screen.getByRole('tablist', { name: 'Template gallery' })
  const templates = within(gallery).getByRole('tab', { name: 'Templates' })
  const samples = within(gallery).getByRole('tab', {
    name: 'Official Qwen examples',
  })
  expect(templates).toHaveAttribute('aria-selected', 'true')
  expect(screen.getByRole('button', { name: 'All templates' })).toBeVisible()
  expect(screen.queryByRole('button', { name: 'Load example' })).toBeNull()

  templates.focus()
  await user.keyboard('{ArrowRight}')
  expect(samples).toHaveFocus()
  await user.keyboard('{Enter}')
  expect(samples).toHaveAttribute('aria-selected', 'true')
  expect(
    screen.getAllByRole('button', { name: 'Load example' })[0]
  ).toBeVisible()
  expect(screen.queryByRole('button', { name: 'All templates' })).toBeNull()
})

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

test('a queued edit retains a box even when its model has no image-mask tag', async () => {
  const getPricing = http.get.getMockImplementation()
  http.get.mockImplementation(async (url) =>
    url === '/api/media-studio/config'
      ? {
          data: {
            image_queue_enabled: true,
            image_queue_access: true,
            image_queue_models: ['edit-model'],
          },
        }
      : getPricing?.(url)
  )
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
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
  const surface = await screen.findByRole('group', { name: 'Selection area' })
  await waitFor(() => expect(surface).toHaveAttribute('aria-disabled', 'false'))
  fireEvent.keyDown(surface, { key: 'Enter' })
  expect(screen.getByRole('button', { name: 'Undo box' })).toBeEnabled()
})

test.each(['standard', 'queued'])(
  '%s edit shows applied colored marks on the reference after closing the editor',
  async (mode) => {
    const getPricing = http.get.getMockImplementation()
    http.get.mockImplementation(async (url) =>
      url === '/api/media-studio/config'
        ? {
            data: {
              image_queue_enabled: mode === 'queued',
              image_queue_access: true,
              image_queue_models: ['edit-model'],
            },
          }
        : getPricing?.(url)
    )
    page()
    await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
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
    fireEvent.click(
      await screen.findByRole('button', { name: 'Edit reference image' })
    )
    const dialog = within(screen.getByRole('dialog'))
    fireEvent.click(dialog.getByRole('button', { name: 'Colored marks' }))
    const surface = dialog.getByRole('group', {
      name: 'Reference drawing surface',
    })
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'ArrowRight' })
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'Enter' })
    fireEvent.change(dialog.getByLabelText('Stroke color'), {
      target: { value: 'red' },
    })
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'ArrowDown' })
    fireEvent.keyDown(surface, { key: ' ' })
    fireEvent.keyDown(surface, { key: 'Enter' })
    fireEvent.click(dialog.getByRole('button', { name: 'Use drawing' }))

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    const reference = within(
      screen.getByRole('region', { name: 'Reference images' })
    ).getByRole('figure')
    const preview = within(reference).getByRole('img', {
      name: 'Applied drawing',
    })
    expect(preview).toBeVisible()
    expect(preview.querySelector('path[stroke="blue"]')).toHaveAttribute('d')
    expect(preview.querySelector('path[stroke="red"]')).toHaveAttribute('d')
  }
)

test('a reference attached in the composer reaches the relay as multipart', async () => {
  // 图生图的成败全在这条契约上：参考图作为 multipart 的 image 文件 part 提交,
  // 标量模型/提示走 form 字段, 全程不碰对象存储。
  page()
  await waitFor(() => expect(screen.getByLabelText('Model')).toBeEnabled())
  // 「Image to image」在模版图库的筛选里也有同名按钮，必须限定在写作台的
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
  const [url, body] = http.post.mock.calls[0] as [string, FormData]
  expect(url).toBe('/pg/images/edits')
  expect(body).toBeInstanceOf(FormData)
  expect(body.get('model')).toBe('edit-model')
  expect(body.get('prompt')).toBe('Blue coat')
  expect(body.get('image')).toBeInstanceOf(File)
  expect(
    http.post.mock.calls.some(([p]) => String(p).includes('presign'))
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
