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
import { fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, describe, expect, test, vi } from 'vitest'

import { HistoryList } from '../components/history-list'
import type { HistoryEntry, StudioParams } from '../types'

function makeEntry(
  id: string,
  createdAt: number,
  prompt = 'a cat',
  count = 1
): HistoryEntry {
  const params: StudioParams = {
    prompt,
    ratio: '1:1',
    count,
    quality: 'standard',
    seed: 42,
    cfg: 1,
  }
  return {
    id,
    prompt,
    model: 'qwen-image-2512',
    params,
    imageUrls: Array.from(
      { length: count },
      (_, i) => `data:image/png;base64,iVBOR${id}${i}`
    ),
    elapsedMs: 45000,
    createdAt,
  }
}

describe('HistoryList', () => {
  beforeAll(() => {
    i18next.addResourceBundle('en', 'translation', {
      'Recent generations': 'Recent generations',
      Clear: 'Clear',
      'Clear history': 'Clear history',
      Delete: 'Delete',
      'No generations in this browser yet.':
        'No generations in this browser yet.',
      'History saving is not available in this browser.':
        'History saving is not available in this browser.',
      '{{count}} image · {{ratio}} · {{model}} · {{elapsed}}':
        '{{count}} image · {{ratio}} · {{model}} · {{elapsed}}',
      'Generated image': 'Generated image',
      Download: 'Download',
    })
  })

  test('renders an empty state when there are no entries', () => {
    render(<HistoryList entries={[]} onDelete={vi.fn()} onClear={vi.fn()} />)

    expect(
      screen.getByText('No generations in this browser yet.')
    ).toBeDefined()
  })

  test('renders nothing while loading', () => {
    render(
      <HistoryList entries={[]} loading onDelete={vi.fn()} onClear={vi.fn()} />
    )

    expect(
      screen.queryByRole('region', { name: 'Recent generations' })
    ).toBeNull()
  })

  test('shows a notice when storage is unavailable', () => {
    render(
      <HistoryList
        entries={[]}
        storageAvailable={false}
        onDelete={vi.fn()}
        onClear={vi.fn()}
      />
    )

    expect(
      screen.getByText('History saving is not available in this browser.')
    ).toBeDefined()
  })

  test('lists entries with prompt, metadata and date', () => {
    render(
      <HistoryList
        entries={[
          makeEntry('new', 1700000000000, 'a dog'),
          makeEntry('old', 1699900000000, 'a cat'),
        ]}
        onDelete={vi.fn()}
        onClear={vi.fn()}
      />
    )

    expect(screen.getByText('a dog')).toBeDefined()
    expect(screen.getByText('a cat')).toBeDefined()
    const row = screen.getByRole('button', { name: /a cat/ })
    expect(row.textContent).toContain('1 image · 1:1 · qwen-image-2512 · 45s')
  })

  test('selecting an entry shows its gallery and selecting again hides it', () => {
    render(
      <HistoryList
        entries={[makeEntry('a', 1700000000000)]}
        onDelete={vi.fn()}
        onClear={vi.fn()}
      />
    )

    const row = screen.getByRole('button', { name: /a cat/ })
    expect(screen.queryByRole('link', { name: 'Download' })).toBeNull()

    fireEvent.click(row)
    expect(screen.getByRole('link', { name: 'Download' })).toBeDefined()

    fireEvent.click(row)
    expect(screen.queryByRole('link', { name: 'Download' })).toBeNull()
  })

  test('deleting an entry calls onDelete with its id', () => {
    const onDelete = vi.fn()
    render(
      <HistoryList
        entries={[makeEntry('a', 1700000000000)]}
        onDelete={onDelete}
        onClear={vi.fn()}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Delete' }))

    expect(onDelete).toHaveBeenCalledWith('a')
  })

  test('clearing calls onClear and hides the button when empty', () => {
    const onClear = vi.fn()
    const { rerender } = render(
      <HistoryList
        entries={[makeEntry('a', 1700000000000)]}
        onDelete={vi.fn()}
        onClear={onClear}
      />
    )

    fireEvent.click(screen.getByRole('button', { name: 'Clear history' }))
    expect(onClear).toHaveBeenCalledTimes(1)

    rerender(<HistoryList entries={[]} onDelete={vi.fn()} onClear={onClear} />)
    expect(screen.queryByRole('button', { name: 'Clear history' })).toBeNull()
  })
})
