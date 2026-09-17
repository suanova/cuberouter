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
import { act, renderHook, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

const storage = vi.hoisted(() => ({
  listHistoryEntries: vi.fn(),
  saveHistoryEntry: vi.fn(),
  deleteHistoryEntry: vi.fn(),
  clearHistoryEntries: vi.fn(),
}))

vi.mock('../lib/history-storage', () => ({
  ...storage,
  MAX_HISTORY_ENTRIES: 50,
  sortHistoryEntries: (entries: Array<{ createdAt: number }>) =>
    [...entries].sort((a, b) => b.createdAt - a.createdAt),
}))

import { useHistory } from '../hooks/use-history'
import type { HistoryEntry, StudioParams } from '../types'

const params: StudioParams = {
  prompt: 'a cat',
  ratio: '1:1',
  count: 1,
  quality: 'standard',
  seed: 42,
  cfg: 1,
}

function makeEntry(id: string, createdAt: number): HistoryEntry {
  return {
    id,
    prompt: 'a cat',
    model: 'qwen-image-2512',
    params,
    imageUrls: [`data:image/png;base64,iVBOR${id}`],
    elapsedMs: 13000,
    createdAt,
  }
}

describe('useHistory', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    storage.listHistoryEntries.mockResolvedValue([])
  })

  test('loads the stored entries on mount', async () => {
    storage.listHistoryEntries.mockResolvedValue([
      makeEntry('b', 2000),
      makeEntry('a', 1000),
    ])

    const { result } = renderHook(() => useHistory())

    expect(result.current.loading).toBe(true)
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.entries.map((entry) => entry.id)).toEqual(['b', 'a'])
  })

  test('saveEntry prepends the entry immediately and reconciles with storage', async () => {
    storage.saveHistoryEntry.mockImplementation(async (entry: HistoryEntry) => [
      entry,
    ])
    const { result } = renderHook(() => useHistory())
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => {
      result.current.saveEntry(makeEntry('a', 1000))
    })

    expect(result.current.entries.map((entry) => entry.id)).toEqual(['a'])
    await waitFor(() =>
      expect(storage.saveHistoryEntry).toHaveBeenCalledWith(
        expect.objectContaining({ id: 'a' })
      )
    )
  })

  test('removes an entry from state and storage', async () => {
    storage.listHistoryEntries.mockResolvedValue([
      makeEntry('b', 2000),
      makeEntry('a', 1000),
    ])
    storage.deleteHistoryEntry.mockResolvedValue([makeEntry('b', 2000)])
    const { result } = renderHook(() => useHistory())
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => {
      result.current.removeEntry('a')
    })

    expect(result.current.entries.map((entry) => entry.id)).toEqual(['b'])
    await waitFor(() =>
      expect(storage.deleteHistoryEntry).toHaveBeenCalledWith('a')
    )
  })

  test('clears the list and storage', async () => {
    storage.listHistoryEntries.mockResolvedValue([makeEntry('a', 1000)])
    storage.clearHistoryEntries.mockResolvedValue([])
    const { result } = renderHook(() => useHistory())
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => {
      result.current.clearEntries()
    })

    expect(result.current.entries).toEqual([])
    await waitFor(() => expect(storage.clearHistoryEntries).toHaveBeenCalled())
  })

  test('marks storage unavailable when loading fails', async () => {
    storage.listHistoryEntries.mockRejectedValue(new Error('denied'))

    const { result } = renderHook(() => useHistory())

    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.entries).toEqual([])
    expect(result.current.storageAvailable).toBe(false)
  })

  test('keeps in-memory entries and marks storage unavailable when saving fails', async () => {
    storage.saveHistoryEntry.mockRejectedValue(new Error('quota'))
    const { result } = renderHook(() => useHistory())
    await waitFor(() => expect(result.current.loading).toBe(false))

    act(() => {
      result.current.saveEntry(makeEntry('a', 1000))
    })
    await waitFor(() => expect(result.current.storageAvailable).toBe(false))

    expect(result.current.entries.map((entry) => entry.id)).toEqual(['a'])
  })
})
