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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { ImageAudit } from '../components/image-audit'

beforeEach(() =>
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'root', role: ROLE.SUPER_ADMIN })
)
afterEach(() => useAuthStore.getState().auth.reset())

test.each(['0', '-1', '1.5'])(
  'invalid User ID %s cannot silently request all audit rows',
  async (value) => {
    const get = vi.spyOn(api, 'get').mockResolvedValue({ data: { data: [] } })
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={client}>
        <ImageAudit />
      </QueryClientProvider>
    )
    await screen.findByText('No image audit records.')
    fireEvent.change(screen.getByRole('spinbutton', { name: 'User ID' }), {
      target: { value },
    })
    expect(screen.getByRole('alert')).toHaveTextContent(
      'Enter a positive whole-number User ID.'
    )
    expect(screen.getByRole('button', { name: 'Refresh' })).toBeDisabled()
    expect(screen.queryByRole('status')).not.toBeInTheDocument()
    expect(get).toHaveBeenCalledTimes(1)
    client.clear()
  }
)

test('an audit-list request failure gives no claim about generated-image release', async () => {
  vi.spyOn(api, 'get').mockRejectedValue(new Error('offline'))
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ImageAudit />
    </QueryClientProvider>
  )
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Image auditing is unavailable. Try again later.'
  )
  expect(screen.queryByText(/No image was released/)).not.toBeInTheDocument()
  client.clear()
})

test('losing root access hides loaded audits and stops further queries', async () => {
  const get = vi.spyOn(api, 'get').mockResolvedValue({ data: { data: [] } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ImageAudit />
    </QueryClientProvider>
  )
  await screen.findByText('No image audit records.')
  act(() =>
    useAuthStore
      .getState()
      .auth.setUser({ id: 1, username: 'root', role: ROLE.ADMIN })
  )
  expect(screen.queryByText('Image output audit')).not.toBeInTheDocument()
  await client.invalidateQueries({ queryKey: ['studio-image-audit'] })
  expect(get).toHaveBeenCalledTimes(1)
  client.clear()
})

test('audit details expose the original instruction and blocked reason; filtering shows only the requested account', async () => {
  const get = vi
    .spyOn(api, 'get')
    .mockResolvedValueOnce({
      data: {
        data: [
          {
            job_id: 'reviewed-job',
            user_id: 7,
            model: 'qwen-image-2.1',
            user_prompt: '<b>Original instruction</b>',
            model_prompt: 'Prepared model instruction',
            parameters: '{"seed":0}',
            input_hashes: '[]',
            output_hash: 'final-png-hash',
            review: '{"decision":"block"}',
            channel_id: 10,
            state: 'failed',
            error: 'output_blocked',
            created_at: 1,
          },
        ],
      },
    })
    .mockResolvedValue({ data: { data: [] } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ImageAudit />
    </QueryClientProvider>
  )
  fireEvent.click(await screen.findByText(/reviewed-job/))
  expect(screen.getByText('<b>Original instruction</b>')).toBeVisible()
  expect(screen.getByText('Prepared model instruction')).toBeVisible()
  expect(screen.getByText('output_blocked')).toBeVisible()
  expect(screen.queryByRole('img')).not.toBeInTheDocument()
  expect(screen.getByRole('button', { name: 'Older records' })).toBeDisabled()
  fireEvent.change(screen.getByRole('spinbutton', { name: 'User ID' }), {
    target: { value: '8' },
  })
  await waitFor(() =>
    expect(get).toHaveBeenLastCalledWith('/api/image-studio-audit', {
      params: { before: undefined, before_job_id: undefined, user_id: '8' },
    })
  )
  expect(await screen.findByText('No image audit records.')).toBeVisible()
  expect(
    screen.queryByText('<b>Original instruction</b>')
  ).not.toBeInTheDocument()
  client.clear()
})
