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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { MediaStudio } from '../index'

const http = vi.hoisted(() => ({
  get: vi.fn(),
  post: vi.fn(),
  delete: vi.fn(),
}))
vi.mock('@/lib/api', () => ({ api: http }))

let query: QueryClient
beforeEach(() => {
  vi.clearAllMocks()
  query = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  http.get.mockImplementation(async (url: string) => {
    if (url.endsWith('/config')) return { data: { enabled: false } }
    if (url.includes('pricing')) {
      return { data: { success: true, data: { pricings: [] } } }
    }
    throw new Error(`Unexpected request: ${url}`)
  })
})
afterEach(() => query.clear())

describe('Studio without a configured image service', () => {
  test('keeps templates and image-to-image settings visible without uploading or submitting work', async () => {
    render(
      <QueryClientProvider client={query}>
        <MediaStudio />
      </QueryClientProvider>
    )
    expect(
      await screen.findByText('Image service is not connected')
    ).toBeInTheDocument()
    expect(screen.getByAltText('Pet comic strip')).toBeInTheDocument()
    fireEvent.click(
      screen.getAllByRole('button', { name: 'Image to image' })[0]
    )
    fireEvent.change(screen.getByLabelText('Describe your changes'), {
      target: { value: 'Make the coat blue' },
    })
    expect(screen.getByLabelText('Upload reference images')).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Upload' })).toBeDisabled()
    expect(
      screen.getByRole('button', { name: 'Connect image service to generate' })
    ).toBeDisabled()
    fireEvent.click(screen.getAllByRole('button', { name: 'Text to image' })[0])
    const form = screen.getByLabelText('Prompt').closest('form')
    expect(form).not.toBeNull()
    await act(async () => {
      if (form) fireEvent.submit(form)
    })
    expect(
      screen.getByRole('button', { name: /Creation history/ })
    ).toBeDisabled()
    expect(http.get.mock.calls.every(([url]) => url.endsWith('/config'))).toBe(
      true
    )
    expect(http.post).not.toHaveBeenCalled()
  })

  test('offers basic generation only after the user chooses it and can return to the new studio', async () => {
    render(
      <QueryClientProvider client={query}>
        <MediaStudio />
      </QueryClientProvider>
    )
    fireEvent.click(
      await screen.findByRole('button', { name: 'Use basic image generation' })
    )
    expect(
      await screen.findByText('Turn your ideas into images.')
    ).toBeInTheDocument()
    fireEvent.click(
      screen.getByRole('button', { name: 'Back to image studio' })
    )
    expect(
      await screen.findByText('Image service is not connected')
    ).toBeInTheDocument()
  })

  test('enables the connected workflow after setup and reconnect without a new frontend build', async () => {
    render(
      <QueryClientProvider client={query}>
        <MediaStudio />
      </QueryClientProvider>
    )
    await screen.findByText('Image service is not connected')
    http.get.mockImplementation(async (url: string) => ({
      data: url.endsWith('/config')
        ? {
            enabled: true,
            models: {
              create: 'qwen-image-2512',
              edit: 'qwen-image-edit-2511',
              regional: 'qwen-image-edit-2511',
            },
            health: { create: 'ready', edit: 'ready', tools: 'ready' },
            retention_days: 30,
          }
        : [],
    }))
    fireEvent.click(screen.getByRole('button', { name: 'Reconnect' }))
    await waitFor(() =>
      expect(
        screen.queryByText('Image service is not connected')
      ).not.toBeInTheDocument()
    )
    fireEvent.click(
      screen.getAllByRole('button', { name: 'Image to image' })[0]
    )
    expect(screen.getByLabelText('Upload reference images')).toBeEnabled()
    await waitFor(() =>
      expect(http.get).toHaveBeenCalledWith(
        '/api/v1/media-studio/jobs',
        expect.anything()
      )
    )
  })
})
