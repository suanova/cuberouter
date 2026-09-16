/*
Copyright (C) 2023-2026 QuantumNous

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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

const apiFns = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: { get: apiFns.get, post: apiFns.post } }))

const storageFns = vi.hoisted(() => ({
  listHistoryEntries: vi.fn(),
  saveHistoryEntry: vi.fn(),
  deleteHistoryEntry: vi.fn(),
  clearHistoryEntries: vi.fn(),
}))
vi.mock('../lib/history-storage', () => ({
  ...storageFns,
  MAX_HISTORY_ENTRIES: 50,
  sortHistoryEntries: (entries: Array<{ createdAt: number }>) =>
    [...entries].sort((a, b) => b.createdAt - a.createdAt),
}))

import { MediaStudio } from '../index'
import type { HistoryEntry } from '../types'

describe('MediaStudio history persistence', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Media Studio': 'Media Studio',
      'Turn your ideas into images.': 'Turn your ideas into images.',
      Model: 'Model',
      Prompt: 'Prompt',
      'Image aspect ratio': 'Image aspect ratio',
      'Images per batch': 'Images per batch',
      '{{count}} image': '{{count}} image',
      'Advanced settings': 'Advanced settings',
      'Generate image': 'Generate image',
      'Image preview': 'Image preview',
      'Your images will appear here.': 'Your images will appear here.',
      'Generated image': 'Generated image',
      Download: 'Download',
      '{{count}} image · {{steps}} steps · elapsed {{time}}':
        '{{count}} image · {{steps}} steps · elapsed {{time}}',
      'Recent generations': 'Recent generations',
      'No generations in this browser yet.':
        'No generations in this browser yet.',
      'Request and response': 'Request and response',
      Copy: 'Copy',
    })
  })

  beforeEach(() => {
    vi.clearAllMocks()
    apiFns.get.mockResolvedValue({
      data: {
        success: true,
        data: {
          pricings: [
            {
              model_name: 'qwen-image-2512',
              supported_endpoint_types: ['image-generation'],
            },
          ],
        },
      },
    })
    apiFns.post.mockResolvedValue({
      data: { created: 7, data: [{ b64_json: 'iVBORw0KGgoAAAAB' }] },
    })
    storageFns.listHistoryEntries.mockResolvedValue([])
    storageFns.saveHistoryEntry.mockImplementation(
      async (entry: HistoryEntry) => [entry]
    )
  })

  test('saves a history entry with data URLs after a successful generation', async () => {
    render(<MediaStudio />)

    const modelSelect = screen.getByLabelText('Model')
    await waitFor(() => expect(modelSelect).toBeEnabled())

    fireEvent.change(screen.getByLabelText('Prompt'), {
      target: { value: 'a cat' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))

    await waitFor(() =>
      expect(screen.getByAltText('Generated image')).toBeDefined()
    )
    await waitFor(() =>
      expect(storageFns.saveHistoryEntry).toHaveBeenCalledTimes(1)
    )

    const [entry] = storageFns.saveHistoryEntry.mock.calls[0] as [HistoryEntry]
    expect(entry.imageUrls).toEqual(['data:image/png;base64,iVBORw0KGgoAAAAB'])
    expect(entry.model).toBe('qwen-image-2512')
    expect(entry.prompt).toBe('a cat')
    expect(entry.params.ratio).toBe('16:9')
    expect(entry.imageUrls.length).toBe(1)
  })

  test('does not save a duplicate entry when params change after success', async () => {
    render(<MediaStudio />)

    const modelSelect = screen.getByLabelText('Model')
    await waitFor(() => expect(modelSelect).toBeEnabled())

    const prompt = screen.getByLabelText('Prompt')
    fireEvent.change(prompt, { target: { value: 'a cat' } })
    fireEvent.click(screen.getByRole('button', { name: 'Generate image' }))

    await waitFor(() =>
      expect(storageFns.saveHistoryEntry).toHaveBeenCalledTimes(1)
    )

    // 成功后继续编辑参数会触发保存 effect 重新运行，不应重复保存
    fireEvent.change(prompt, { target: { value: 'a dog' } })
    await waitFor(() => expect(prompt).toHaveValue('a dog'))
    expect(storageFns.saveHistoryEntry).toHaveBeenCalledTimes(1)
  })
})
