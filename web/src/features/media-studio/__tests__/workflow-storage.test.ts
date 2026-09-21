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

import {
  deleteStudioJob,
  listStudioJobs,
  saveStudioJob,
} from '../lib/studio-storage'
import { initialDraft } from '../lib/workflow'
import type { WorkflowJob } from '../workflow-types'

const photo = {
  id: 'photo',
  url: 'data:image/png;base64,YQ==',
  mime: 'image/png',
}
const job: WorkflowJob = {
  id: 'created',
  created_at: 1,
  request: { ...initialDraft, model: 'image-model', prompt: 'Cat' },
  images: [photo],
  elapsed_ms: 100,
}
beforeEach(async () => {
  await deleteStudioJob(101)
  await deleteStudioJob(102)
})
test('local image bytes survive reopening IndexedDB and are isolated by signed-in account', async () => {
  await saveStudioJob(101, job)
  expect((await listStudioJobs(101))[0].images[0].url).toBe(photo.url)
  expect(await listStudioJobs(102)).toEqual([])
})
test('deleting a parent preserves a separately saved edit and its reference bytes', async () => {
  await saveStudioJob(101, job)
  await saveStudioJob(101, {
    ...job,
    id: 'edit',
    created_at: 2,
    request: {
      ...job.request,
      mode: 'edit',
      parent_id: 'created',
      references: [photo],
    },
  })
  await deleteStudioJob(101, 'created')
  const entries = await listStudioJobs(101)
  expect(entries.map((entry) => entry.id)).toEqual(['edit'])
  expect(entries[0].request.references[0].url).toBe(photo.url)
})
test('temporary provider URLs are never mistaken for persistent image bytes', async () => {
  await expect(
    saveStudioJob(101, {
      ...job,
      images: [{ ...photo, url: 'https://provider.example/temporary.png' }],
    })
  ).rejects.toThrow('could not be saved locally')
  expect(await listStudioJobs(101)).toEqual([])
})
test('clearing one account leaves another account untouched', async () => {
  await saveStudioJob(101, job)
  await saveStudioJob(102, job)
  await deleteStudioJob(101)
  expect(await listStudioJobs(101)).toEqual([])
  expect(await listStudioJobs(102)).toHaveLength(1)
})

test('the history limit evicts the oldest completed creation', async () => {
  for (let i = 1; i <= 51; i++) {
    await saveStudioJob(101, { ...job, id: String(i), created_at: i })
  }
  const entries = await listStudioJobs(101)
  expect(entries).toHaveLength(50)
  expect(entries[0].id).toBe('51')
  expect(entries.at(-1)?.id).toBe('2')
})
