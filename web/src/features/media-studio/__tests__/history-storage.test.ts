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
import 'fake-indexeddb/auto'
import { beforeEach, describe, expect, test } from 'vitest'

import {
  MAX_HISTORY_ENTRIES,
  clearHistoryEntries,
  deleteHistoryEntry,
  listHistoryEntries,
  saveHistoryEntry,
} from '../lib/history-storage'
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
    imageUrls: ['data:image/png;base64,iVBORw0KGgo'],
    elapsedMs: 13000,
    createdAt,
  }
}

describe('history storage (IndexedDB)', () => {
  beforeEach(async () => {
    await clearHistoryEntries()
  })

  test('returns saved entries newest first', async () => {
    await saveHistoryEntry(makeEntry('old', 1000))
    await saveHistoryEntry(makeEntry('new', 2000))

    const entries = await listHistoryEntries()

    expect(entries.map((entry) => entry.id)).toEqual(['new', 'old'])
  })

  test('overwrites an entry with the same id instead of duplicating', async () => {
    await saveHistoryEntry({ ...makeEntry('a', 1000), prompt: 'first' })
    await saveHistoryEntry({ ...makeEntry('a', 1000), prompt: 'second' })

    const entries = await listHistoryEntries()

    expect(entries).toHaveLength(1)
    expect(entries[0].prompt).toBe('second')
  })

  test('trims the oldest entries beyond MAX_HISTORY_ENTRIES', async () => {
    const total = MAX_HISTORY_ENTRIES + 3
    for (let i = 0; i < total; i++) {
      await saveHistoryEntry(makeEntry(`e${i}`, i))
    }

    const entries = await listHistoryEntries()

    expect(entries).toHaveLength(MAX_HISTORY_ENTRIES)
    expect(entries.map((entry) => entry.id)).toEqual(
      Array.from({ length: MAX_HISTORY_ENTRIES }, (_, i) => `e${total - 1 - i}`)
    )
  })

  test('removes only the given entry on delete', async () => {
    await saveHistoryEntry(makeEntry('a', 1000))
    await saveHistoryEntry(makeEntry('b', 2000))

    const entries = await deleteHistoryEntry('a')

    expect(entries.map((entry) => entry.id)).toEqual(['b'])
    expect(await listHistoryEntries()).toHaveLength(1)
  })

  test('clears all entries on clear', async () => {
    await saveHistoryEntry(makeEntry('a', 1000))
    await saveHistoryEntry(makeEntry('b', 2000))

    expect(await clearHistoryEntries()).toEqual([])
    expect(await listHistoryEntries()).toEqual([])
  })
})
