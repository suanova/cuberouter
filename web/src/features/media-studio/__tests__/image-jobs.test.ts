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
import { IDBFactory } from 'fake-indexeddb'
import { beforeEach, expect, test, vi } from 'vitest'

import {
  cancelStudioImage,
  acknowledgeStudioImage,
  collectStudioImage,
  IMAGE_JOBS,
  pendingStudioImage,
  submitStudioImage,
  type ImageJobStatus,
  type PendingStudioImage,
} from '../lib/image-jobs'
import { listStudioJobs, saveStudioJob } from '../lib/studio-storage'
import { initialDraft } from '../lib/workflow'

const http = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))
vi.mock('@/lib/api', () => ({ api: http }))
const record: PendingStudioImage = {
  key: 'unique-request-1234',
  epoch: 'boot-a',
  started: 1,
  draft: { ...initialDraft, model: 'qwen-image-2.1', prompt: 'a cat' },
}
const status: ImageJobStatus = {
  id: 'job-a',
  key: record.key,
  epoch: record.epoch,
  state: 'running',
  ahead: 0,
  recovery_pending: false,
  cancel_requested: false,
  created_at: 1,
}
beforeEach(() => {
  vi.stubGlobal('indexedDB', new IDBFactory())
  http.get.mockReset()
  http.post.mockReset()
})

test('pending drafts are isolated by account and reject an invalid owner', async () => {
  await pendingStudioImage(1, record)
  expect(await pendingStudioImage(2)).toBeUndefined()
  expect((await pendingStudioImage(1))?.draft.prompt).toBe('a cat')
  await expect(pendingStudioImage(0, record)).rejects.toThrow('Sign in')
})

test('a lost submit response keeps its persisted identity and blocks a second inference', async () => {
  http.get.mockResolvedValue({ data: { epoch: 'boot-a' } })
  http.post.mockImplementation(async () => {
    expect(await pendingStudioImage(1)).toBeDefined()
    throw new Error('connection lost')
  })
  await expect(submitStudioImage(1, record.draft)).rejects.toThrow(
    'connection lost'
  )
  expect(await pendingStudioImage(1)).toBeDefined()
  await expect(submitStudioImage(1, record.draft)).rejects.toThrow('Reconnect')
  expect(http.post).toHaveBeenCalledTimes(1)
  expect(http.post).toHaveBeenCalledWith(
    IMAGE_JOBS,
    expect.objectContaining({
      request: expect.objectContaining({
        num_inference_steps: 40,
        true_cfg_scale: 1,
      }),
    }),
    expect.anything()
  )
})

test('server image defaults preserve explicit zero CFG and seed values', async () => {
  http.get.mockResolvedValue({ data: { epoch: 'boot-a' } })
  http.post.mockResolvedValue({ data: status })
  await submitStudioImage(1, { ...record.draft, steps: 8, cfg: 0, seed: 0 })
  expect(http.post).toHaveBeenCalledWith(
    IMAGE_JOBS,
    expect.objectContaining({
      request: expect.objectContaining({
        num_inference_steps: 8,
        true_cfg_scale: 0,
        seed: 0,
      }),
    }),
    expect.anything()
  )
})

test('reconnecting collects the reviewed PNG without resubmitting or acknowledging', async () => {
  await pendingStudioImage(1, record)
  http.get
    .mockResolvedValueOnce({ data: { ...status, state: 'completed' } })
    .mockResolvedValueOnce({
      data: new Blob(['reviewed'], { type: 'image/png' }),
    })
  const output = await collectStudioImage(
    1,
    record,
    new AbortController().signal,
    vi.fn()
  )
  expect(output.id).toBe('job-a')
  expect(output.images[0].url).toMatch(/^data:image\/png;base64,/)
  expect(http.get).toHaveBeenLastCalledWith(
    `${IMAGE_JOBS}/job-a/result`,
    expect.objectContaining({ responseType: 'blob' })
  )
  expect(http.post).not.toHaveBeenCalled()
})

test.each(['studio_access_unavailable', 'output_check_unavailable'])(
  'a pre-acceptance %s rejection clears pending identity for a later submit',
  async (code) => {
    http.get.mockResolvedValue({ data: { epoch: 'boot-a' } })
    http.post.mockRejectedValue({
      isAxiosError: true,
      response: { status: 503, data: { error: { code } } },
    })
    await expect(submitStudioImage(1, record.draft)).rejects.toThrow()
    expect(await pendingStudioImage(1)).toBeUndefined()
  }
)

test('an audit-write 503 after acceptance keeps the pending identity for recovery', async () => {
  http.get.mockResolvedValue({ data: { epoch: 'boot-a' } })
  http.post.mockRejectedValue({
    isAxiosError: true,
    response: {
      status: 503,
      data: { error: { code: 'studio_audit_unavailable' } },
    },
  })
  await expect(submitStudioImage(1, record.draft)).rejects.toThrow()
  expect(await pendingStudioImage(1)).toBeDefined()
})

test('a blocked job never requests image bytes or creates browser artwork', async () => {
  await pendingStudioImage(1, record)
  http.get.mockResolvedValue({
    data: { ...status, state: 'failed', error: 'output_blocked' },
  })
  await expect(
    collectStudioImage(1, record, new AbortController().signal, vi.fn())
  ).rejects.toThrow('did not pass output review')
  expect(http.get).toHaveBeenCalledTimes(1)
  expect(http.post).not.toHaveBeenCalled()
})

test('lost cancellation is remembered and reconnect retries only the cancellation', async () => {
  await pendingStudioImage(1, record)
  http.post.mockRejectedValue(new Error('lost cancel response'))
  await expect(cancelStudioImage(1, record, status)).rejects.toThrow(
    'lost cancel response'
  )
  expect((await pendingStudioImage(1))?.cancelRequested).toBe(true)
  http.get.mockResolvedValue({ data: status })
  http.post.mockRejectedValue(new Error('cancel retry'))
  await expect(
    collectStudioImage(1, record, new AbortController().signal, vi.fn())
  ).rejects.toThrow('cancel retry')
  expect(http.post).toHaveBeenLastCalledWith(
    `${IMAGE_JOBS}/job-a/cancel`,
    {},
    expect.anything()
  )
  expect(
    http.post.mock.calls.every((call) => call[0].endsWith('/cancel'))
  ).toBe(true)
})

test('a changed gateway epoch marks the draft recoverable without replaying generation', async () => {
  http.get.mockResolvedValue({ data: { ...status, epoch: 'boot-b' } })
  const progress = vi.fn()
  await expect(
    collectStudioImage(1, record, new AbortController().signal, progress)
  ).rejects.toThrow('restarted')
  expect(progress).toHaveBeenLastCalledWith(
    expect.objectContaining({ state: 'expired' })
  )
  expect(http.post).not.toHaveBeenCalled()
})

test('two tabs racing to submit claim one recovery record and make one inference request', async () => {
  http.get.mockResolvedValue({ data: { epoch: 'boot-a' } })
  http.post.mockResolvedValue({ data: status })
  const attempts = await Promise.allSettled([
    submitStudioImage(1, record.draft),
    submitStudioImage(1, record.draft),
  ])
  expect(attempts.filter((value) => value.status === 'fulfilled')).toHaveLength(
    1
  )
  expect(http.post).toHaveBeenCalledTimes(1)
})

test('cancellation intent wins over a completed response before any result is downloaded', async () => {
  await pendingStudioImage(1, { ...record, cancelRequested: true })
  http.get
    .mockResolvedValueOnce({ data: { ...status, state: 'completed' } })
    .mockResolvedValueOnce({
      data: { ...status, state: 'cancelled', cancel_requested: true },
    })
  http.post.mockResolvedValue({ data: { ...status, state: 'cancelled' } })
  await expect(
    collectStudioImage(1, record, new AbortController().signal, vi.fn())
  ).rejects.toThrow()
  expect(http.post).toHaveBeenCalledWith(
    `${IMAGE_JOBS}/job-a/cancel`,
    {},
    expect.anything()
  )
  expect(http.get.mock.calls.some((call) => call[0].endsWith('/result'))).toBe(
    false
  )
})

test('a late acknowledgement from an old tab cannot erase a newer recovery record', async () => {
  await pendingStudioImage(1, { ...record, key: 'newer-request-1234' })
  http.post.mockResolvedValue({})
  await acknowledgeStudioImage(1, 'old-job', record.key)
  expect((await pendingStudioImage(1))?.key).toBe('newer-request-1234')
})

test('a stale tab cancelling an acknowledged job cannot recreate its cleared recovery record', async () => {
  await pendingStudioImage(1, record)
  http.post.mockResolvedValue({})
  await acknowledgeStudioImage(1, status.id, record.key)
  expect(await pendingStudioImage(1)).toBeUndefined()
  await cancelStudioImage(1, record, status)
  expect(await pendingStudioImage(1)).toBeUndefined()
})

test('cancellation recorded while downloading discards the received PNG before creating artwork', async () => {
  await pendingStudioImage(1, record)
  http.get
    .mockResolvedValueOnce({ data: { ...status, state: 'completed' } })
    .mockImplementationOnce(async () => {
      await pendingStudioImage(1, { ...record, cancelRequested: true })
      return { data: new Blob(['reviewed'], { type: 'image/png' }) }
    })
    .mockResolvedValueOnce({
      data: { ...status, state: 'cancelled', cancel_requested: true },
    })
  http.post.mockResolvedValue({})
  await expect(
    collectStudioImage(1, record, new AbortController().signal, vi.fn())
  ).rejects.toThrow()
  expect(http.post).toHaveBeenCalledWith(
    `${IMAGE_JOBS}/job-a/cancel`,
    {},
    expect.anything()
  )
})

test.each([
  { ...record, cancelRequested: true },
  { ...record, key: 'another-request-1234' },
])(
  'history commits reject cancelled or replaced recovery records atomically: $key $cancelRequested',
  async (current) => {
    await pendingStudioImage(1, current)
    const saved = await saveStudioJob(
      1,
      {
        id: 'job-a',
        created_at: 1,
        request: record.draft,
        elapsed_ms: 10,
        images: [
          {
            id: 'approved',
            mime: 'image/png',
            url: 'data:image/png;base64,YQ==',
          },
        ],
      },
      record.key
    )
    expect(saved).toBe(false)
    expect(await listStudioJobs(1)).toEqual([])
    expect((await pendingStudioImage(1))?.key).toBe(current.key)
  }
)
