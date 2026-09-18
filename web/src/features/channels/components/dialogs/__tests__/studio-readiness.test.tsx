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
import { fireEvent, render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { channelSchema } from '../../../types'
import { ChannelTestDialog } from '../channel-test-dialog'

vi.mock('@/lib/api', () => ({ api: { get: vi.fn() } }))
vi.mock('../../channels-provider', () => ({
  useChannels: () => ({
    currentRow: channelSchema.parse({
      id: 1,
      type: 1,
      key: '',
      status: 1,
      name: 'Qwen Studio',
      created_time: 0,
      test_time: 0,
      response_time: 0,
      balance_updated_time: 0,
      models: 'qwen-image-2512',
    }),
  }),
}))

test('readiness result shows connected and says no image was generated', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: { success: true, time: 0.25, test_mode: 'studio-readiness' },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelTestDialog open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
  fireEvent.click(
    screen.getByRole('button', { name: 'Test Connection' })
  )
  expect(await screen.findByText('Connected', { exact: true })).toBeVisible()
  expect(
    screen.getByText(
      'Model service is ready. No image was generated; test generation and editing in Image Studio.'
    )
  ).toBeVisible()
  expect(screen.getByText(/Connection check:/)).toBeVisible()
  client.clear()
})

test('failed readiness stays failed and shows the upstream diagnosis', async () => {
  vi.mocked(api.get).mockResolvedValue({
    data: {
      success: false,
      test_mode: 'studio-readiness',
      message: 'Studio model is unavailable',
    },
  })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelTestDialog open onOpenChange={vi.fn()} />
    </QueryClientProvider>
  )
  fireEvent.click(
    screen.getByRole('button', { name: 'Test Connection' })
  )
  expect(await screen.findByText('Studio model is unavailable')).toBeVisible()
  expect(
    screen.queryByText('Connected', { exact: true })
  ).not.toBeInTheDocument()
  client.clear()
})
