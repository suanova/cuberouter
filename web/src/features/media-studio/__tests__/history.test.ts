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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { HISTORY_KEY, HISTORY_LIMIT } from '../constants'
import { clearHistory, loadHistory, saveHistoryEntry } from '../lib/history'
import type { HistoryEntry, StudioParams } from '../types'

const baseParams: StudioParams = {
  prompt: 'a lighthouse',
  ratio: '16:9',
  count: 1,
  steps: 40,
  seed: 42,
  cfg: 1,
}

function makeEntry(index: number): HistoryEntry {
  return {
    id: `entry-${index}`,
    prompt: `prompt ${index}`,
    params: { ...baseParams, prompt: `prompt ${index}` },
    imageUrls: [`https://img.test/${index}.png`],
    elapsedMs: 40_000,
    createdAt: 1_700_000_000_000 + index,
  }
}

describe('media studio history storage', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  test('loadHistory returns an empty list when nothing is stored', () => {
    expect(loadHistory()).toEqual([])
  })

  test('saveHistoryEntry prepends the newest entry', () => {
    const first = makeEntry(1)
    const second = makeEntry(2)

    saveHistoryEntry(first)
    saveHistoryEntry(second)

    expect(loadHistory().map((entry) => entry.id)).toEqual([
      'entry-2',
      'entry-1',
    ])
  })

  test('saveHistoryEntry keeps at most HISTORY_LIMIT entries', () => {
    for (let i = 0; i < HISTORY_LIMIT + 3; i += 1) {
      saveHistoryEntry(makeEntry(i))
    }

    const entries = loadHistory()

    expect(entries).toHaveLength(HISTORY_LIMIT)
    expect(entries[0].id).toBe(`entry-${HISTORY_LIMIT + 2}`)
    expect(entries[HISTORY_LIMIT - 1].id).toBe('entry-3')
  })

  test('loadHistory returns an empty list when the stored value is corrupted', () => {
    window.localStorage.setItem(HISTORY_KEY, '{not-json')

    expect(loadHistory()).toEqual([])
  })

  test('loadHistory returns an empty list when the stored value is not an array', () => {
    window.localStorage.setItem(HISTORY_KEY, JSON.stringify({ id: 'x' }))

    expect(loadHistory()).toEqual([])
  })

  test('clearHistory removes the stored value', () => {
    saveHistoryEntry(makeEntry(1))

    clearHistory()

    expect(loadHistory()).toEqual([])
    expect(window.localStorage.getItem(HISTORY_KEY)).toBeNull()
  })

  test('saveHistoryEntry does not throw when storage writes fail', () => {
    vi.spyOn(window.localStorage, 'setItem').mockImplementation(() => {
      throw new Error('quota exceeded')
    })

    expect(() => saveHistoryEntry(makeEntry(1))).not.toThrow()
  })
})
