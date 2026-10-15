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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, expect, test, vi } from 'vitest'

import { ImageStudioAudit } from '..'

const http = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))
const row = {
  id: 15,
  job_id: 'job-15',
  username: 'demo',
  user_id: 89,
  created_at: 100,
  expires_at: 200,
  channel_id: 3,
  source: 'api',
  state: 'completed',
  moderation_state: 'passed',
  moderation_policy: 'required',
  user_prompt: 'A blue teapot',
  model_prompt: 'Paint a blue teapot',
  parameters: '{"steps":40}',
  input_sha256: '["input-hash"]',
  output_sha256: 'output-hash',
  moderation_result: '{"sexual":0.01}',
  token_name: 'demo-key',
}
beforeEach(() => {
  vi.clearAllMocks()
  http.get.mockImplementation(async (path: string) => ({
    data: path.endsWith('policy')
      ? {
          success: true,
          groups: ['image-studio', 'default'],
          data: [{ group: 'image-studio', moderation: 'required' }],
        }
      : { success: true, audit_enabled: true, data: [row], next_before_id: 15 },
  }))
  http.put.mockResolvedValue({ data: { success: true } })
})
function mount() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <ImageStudioAudit />
    </QueryClientProvider>
  )
}
test('displays input/output audit evidence and submits filters without an empty user ID', async () => {
  mount()
  await screen.findByText('A blue teapot')
  expect(screen.getByText('["input-hash"]')).toBeInTheDocument()
  expect(screen.getByText('{"sexual":0.01}')).toBeInTheDocument()
  expect(http.get).toHaveBeenCalledWith(
    '/api/image-studio-audit',
    expect.objectContaining({
      params: expect.objectContaining({ user_id: undefined }),
    })
  )
  fireEvent.change(screen.getByLabelText('User ID'), {
    target: { value: '89' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Search' }))
  await waitFor(() =>
    expect(http.get).toHaveBeenCalledWith(
      '/api/image-studio-audit',
      expect.objectContaining({
        params: expect.objectContaining({
          user_id: '89',
          before_id: undefined,
        }),
      })
    )
  )
  fireEvent.click(await screen.findByRole('button', { name: 'Older records' }))
  await waitFor(() =>
    expect(http.get).toHaveBeenCalledWith(
      '/api/image-studio-audit',
      expect.objectContaining({
        params: expect.objectContaining({ user_id: '89', before_id: 15 }),
      })
    )
  )
})
test('shows effective group policies and saves only an explicit administrator choice', async () => {
  mount()
  await screen.findByRole('option', { name: 'image-studio' })
  expect(screen.getByRole('button', { name: 'Save policy' })).toBeDisabled()
  fireEvent.change(screen.getByLabelText('Account group'), {
    target: { value: 'image-studio' },
  })
  expect(screen.getByLabelText('Review policy')).toHaveValue('required')
  fireEvent.change(screen.getByLabelText('Account group'), {
    target: { value: 'default' },
  })
  expect(screen.getByLabelText('Review policy')).toHaveValue('external')
  expect(http.put).not.toHaveBeenCalled()
  fireEvent.change(screen.getByLabelText('Review policy'), {
    target: { value: 'required' },
  })
  fireEvent.click(screen.getByRole('button', { name: 'Save policy' }))
  await screen.findByText('Policy saved.')
  expect(http.put).toHaveBeenCalledWith('/api/image-api-policy', {
    group: 'default',
    moderation: 'required',
  })
})
test('reports audit access failure instead of an empty successful audit', async () => {
  http.get.mockResolvedValue({
    data: { success: false, message: 'Access denied' },
  })
  mount()
  await waitFor(() =>
    expect(screen.getAllByRole('alert').length).toBeGreaterThan(0)
  )
  expect(
    screen.queryByText('No image audit records found.')
  ).not.toBeInTheDocument()
})
