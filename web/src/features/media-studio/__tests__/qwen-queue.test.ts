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
import 'fake-indexeddb/auto'
import { afterEach, expect, test, vi } from 'vitest'

import {
  pendingJob,
  submitQwenJob,
  collectQwenJob,
  releaseQwenResult,
} from '../lib/qwen-queue'
import { initialDraft } from '../lib/workflow'
const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))
afterEach(() => {
  vi.clearAllMocks()
})
test('a lost submit response retains a recoverable account-isolated draft and never auto-reposts', async () => {
  http.get.mockResolvedValue({ data: { epoch: 'epoch' } })
  http.post.mockRejectedValue(new Error('Network interrupted'))
  await expect(
    submitQwenJob(8701, { ...initialDraft, queued: true, prompt: 'A blue cup' })
  ).rejects.toThrow('Network interrupted')
  const pending = await pendingJob(8701)
  expect(pending?.draft.prompt).toBe('A blue cup')
  expect(await pendingJob(8702)).toBeUndefined()
  await expect(submitQwenJob(8701, initialDraft)).rejects.toThrow('Reconnect')
  expect(http.post).toHaveBeenCalledTimes(1)
  await pendingJob(8701, null)
})
test('reconnecting retrieves a finished job by request key and acknowledges only when explicitly released', async () => {
  const record = {
    key: 'k'.repeat(32),
    epoch: 'epoch',
    draft: { ...initialDraft, queued: true },
    created: 100,
  }
  await pendingJob(8703, record)
  http.get.mockImplementation(async (path: string) =>
    path.endsWith('/result')
      ? { data: new Blob(['png'], { type: 'image/png' }) }
      : { data: { id: 'a'.repeat(32), state: 'completed', elapsed_seconds: 8 } }
  )
  const status = vi.fn()
  const result = await collectQwenJob(
    record,
    new AbortController().signal,
    status
  )
  expect(result.request_id).toBe('a'.repeat(32))
  expect(http.post).not.toHaveBeenCalled()
  expect(status).toHaveBeenCalledWith(
    expect.objectContaining({ state: 'completed' })
  )
  http.post.mockResolvedValue({ data: {} })
  await releaseQwenResult(8703, result.id)
  expect(await pendingJob(8703)).toBeUndefined()
})
