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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
  http.get.mockImplementation(async (path: string) =>
    path.endsWith('/config')
      ? { data: { upload_enabled: true, edit_models: ['edit-model'] } }
      : {
          data: {
            success: true,
            data: {
              pricings: [
                {
                  model_name: 'image-model',
                  supported_endpoint_types: ['image-generation'],
                },
                {
                  model_name: 'edit-model',
                  supported_endpoint_types: ['image-generation'],
                },
                {
                  model_name: 'chat-model',
                  supported_endpoint_types: ['openai'],
                },
              ],
            },
          },
        }
  )
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
    { model: 'image-model', prompt: 'A cat', n: 1, size: '1024x1024' },
    expect.anything()
  )
  await waitFor(async () => expect(await listStudioJobs(301)).toHaveLength(1))
  view.unmount()
  page()
  fireEvent.click(screen.getByRole('button', { name: 'Creation history' }))
  await screen.findByRole('button', { name: 'Open creation: A cat' })
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
  fireEvent.click(screen.getByRole('button', { name: 'Creation history' }))
  await screen.findByText('No generations in this browser yet.')
  expect(
    screen.queryByRole('button', { name: 'Open creation: Account 301 cat' })
  ).not.toBeInTheDocument()
})
