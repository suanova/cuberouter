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
import { beforeEach, expect, test } from 'vitest'

import {
  clearPendingVideoTask,
  loadPendingVideoTask,
  savePendingVideoTask,
} from '../lib/video-pending'
import { initialVideoDraft } from '../lib/video-draft'

function record(owner: number) {
  return {
    task_id: `task_${owner}`,
    draft: {
      ...initialVideoDraft,
      model: 'viduq3-pro',
      prompt: 'A rabbit',
    },
    started_at: 1234,
  }
}

beforeEach(() => {
  clearPendingVideoTask(401)
  clearPendingVideoTask(402)
})

test('round-trips a pending task and isolates it by account', () => {
  savePendingVideoTask(401, record(401))

  expect(loadPendingVideoTask(401)).toEqual(record(401))
  expect(loadPendingVideoTask(402)).toBeUndefined()

  clearPendingVideoTask(401)
  expect(loadPendingVideoTask(401)).toBeUndefined()
})

test('drops the first frame bytes so the record fits the localStorage quota', () => {
  const withImage = {
    ...record(401),
    draft: {
      ...record(401).draft,
      mode: 'image' as const,
      image: {
        id: 'asset-1',
        mime: 'image/png',
        url: `data:image/png;base64,${'A'.repeat(10_000)}`,
      },
    },
  }
  savePendingVideoTask(401, withImage)

  const loaded = loadPendingVideoTask(401)
  expect(loaded?.draft.mode).toBe('image')
  expect(loaded?.draft.image).toBeUndefined()
  // 字节必须真的没进 storage，而不是只被读侧丢弃。
  expect(window.localStorage.getItem('cuberouter-video-pending-401')).not.toContain(
    'data:image/png'
  )
})

test('treats corrupt or malformed records as absent instead of throwing', () => {
  window.localStorage.setItem('cuberouter-video-pending-401', '{not json')
  expect(loadPendingVideoTask(401)).toBeUndefined()

  window.localStorage.setItem(
    'cuberouter-video-pending-401',
    JSON.stringify({ task_id: 't', draft: { mode: 'wat' } })
  )
  expect(loadPendingVideoTask(401)).toBeUndefined()
})

test('rejects entries without a signed-in account', () => {
  expect(() => savePendingVideoTask(0, record(0))).toThrow(
    'Sign in to use Media Studio.'
  )
  expect(() => loadPendingVideoTask(-1)).toThrow(
    'Sign in to use Media Studio.'
  )
  expect(() => clearPendingVideoTask(0)).toThrow(
    'Sign in to use Media Studio.'
  )
})
