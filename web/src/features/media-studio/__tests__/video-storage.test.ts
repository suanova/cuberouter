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
import { beforeEach, expect, test } from 'vitest'

import { initialVideoDraft } from '../lib/video-draft'
import {
  deleteVideoJob,
  listVideoJobs,
  saveVideoJob,
} from '../lib/video-storage'
import type { VideoJob } from '../video-types'

function makeJob(
  id: string,
  createdAt: number,
  video = new Uint8Array([1, 1, 1]).buffer
): VideoJob {
  return {
    id,
    task_id: `task_${id}`,
    created_at: createdAt,
    request: {
      ...initialVideoDraft,
      model: 'viduq3-pro',
      prompt: `A rabbit ${id}`,
    },
    elapsed_ms: 100,
    video,
    video_mime: 'video/mp4',
  }
}

beforeEach(async () => {
  await deleteVideoJob(201)
  await deleteVideoJob(202)
})

test('video bytes survive reopening IndexedDB and are isolated by signed-in account', async () => {
  await saveVideoJob(
    201,
    makeJob('v1', 1, new Uint8Array([1, 2, 3, 4]).buffer)
  )

  const jobs = await listVideoJobs(201)
  expect(jobs).toHaveLength(1)
  const stored = jobs[0].video
  // fake-indexeddb 按规范在隔离上下文里做 structured clone，返回的
  // ArrayBuffer 跨 realm，instanceof 在测试里不可靠；用构造标签+字节内容断言。
  expect(stored).toBeDefined()
  if (stored === undefined) return
  expect(Object.prototype.toString.call(stored)).toBe('[object ArrayBuffer]')
  expect(new Uint8Array(stored)).toEqual(new Uint8Array([1, 2, 3, 4]))
  expect(await listVideoJobs(202)).toEqual([])
})

test('lists jobs newest first', async () => {
  await saveVideoJob(201, makeJob('old', 1))
  await saveVideoJob(201, makeJob('new', 2))
  await saveVideoJob(201, makeJob('middle', 3))

  expect((await listVideoJobs(201)).map((job) => job.id)).toEqual([
    'middle',
    'new',
    'old',
  ])
})

test('deletes one job or clears the whole account history', async () => {
  await saveVideoJob(201, makeJob('a', 1))
  await saveVideoJob(201, makeJob('b', 2))

  await deleteVideoJob(201, 'a')
  expect((await listVideoJobs(201)).map((job) => job.id)).toEqual(['b'])

  await deleteVideoJob(201)
  expect(await listVideoJobs(201)).toEqual([])
})

test('evicts the oldest job beyond the entry cap', async () => {
  for (let index = 1; index <= 21; index++) {
    await saveVideoJob(201, makeJob(`v${index}`, index))
  }
  const jobs = await listVideoJobs(201)
  expect(jobs).toHaveLength(20)
  expect(jobs.map((job) => job.id)[0]).toBe('v21')
  expect(jobs.map((job) => job.id)).not.toContain('v1')
})

test('rejects entries without a signed-in account', async () => {
  await expect(saveVideoJob(0, makeJob('x', 1))).rejects.toThrow(
    'Sign in to use Media Studio.'
  )
  await expect(listVideoJobs(-1)).rejects.toThrow(
    'Sign in to use Media Studio.'
  )
})

test('keeps a job whose video bytes are missing for display-only fallback', async () => {
  await saveVideoJob(201, {
    ...makeJob('v1', 1),
    video: undefined,
    video_url: 'https://cdn.example/temporary.mp4',
  })

  const jobs = await listVideoJobs(201)
  expect(jobs[0].video).toBeUndefined()
  expect(jobs[0].video_url).toBe('https://cdn.example/temporary.mp4')
})
