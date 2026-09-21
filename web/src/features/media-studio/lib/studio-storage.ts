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
import type { WorkflowJob } from '../workflow-types'

const MAX_ENTRIES = 50
const MAX_BYTES = 100 * 1024 * 1024
// A separate database per signed-in account prevents history appearing after account switches.
async function database(owner: number): Promise<IDBDatabase> {
  if (!Number.isSafeInteger(owner) || owner <= 0) {
    throw new Error('Sign in to use Media Studio.')
  }
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(`cuberouter-studio-${owner}`, 1)
    request.onupgradeneeded = () =>
      request.result.createObjectStore('jobs', { keyPath: 'id' })
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}
export async function listStudioJobs(owner: number): Promise<WorkflowJob[]> {
  const db = await database(owner)
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction('jobs', 'readonly')
      const request = tx.objectStore('jobs').getAll()
      tx.oncomplete = () =>
        resolve(
          (request.result as WorkflowJob[]).sort(
            (a, b) => b.created_at - a.created_at
          )
        )
      tx.onabort = () => reject(tx.error)
      tx.onerror = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}
export async function saveStudioJob(
  owner: number,
  job: WorkflowJob
): Promise<void> {
  if (
    [...job.images, ...job.request.references].some(
      (asset) => !/^data:image\//i.test(asset.url)
    )
  ) {
    throw new Error(
      'This result could not be saved locally. Download it before leaving this page.'
    )
  }
  const size = JSON.stringify(job).length * 2
  if (size > MAX_BYTES) {
    throw new Error(
      'This result is too large for local history. Download it before leaving this page.'
    )
  }
  const db = await database(owner)
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction('jobs', 'readwrite'),
        store = tx.objectStore('jobs')
      store.put(job)
      const all = store.getAll()
      all.onsuccess = () => {
        let bytes = 0
        const entries = (all.result as WorkflowJob[]).sort(
          (a, b) => b.created_at - a.created_at
        )
        for (const [index, entry] of entries.entries()) {
          bytes += JSON.stringify(entry).length * 2
          if (index >= MAX_ENTRIES || bytes > MAX_BYTES) store.delete(entry.id)
        }
      }
      tx.oncomplete = () => resolve()
      tx.onabort = () =>
        reject(tx.error ?? new Error('Local history storage is unavailable.'))
      tx.onerror = () =>
        reject(tx.error ?? new Error('Local history storage is unavailable.'))
    })
  } finally {
    db.close()
  }
}
export async function deleteStudioJob(
  owner: number,
  id?: string
): Promise<void> {
  const db = await database(owner)
  try {
    await new Promise<void>((resolve, reject) => {
      const tx = db.transaction('jobs', 'readwrite'),
        store = tx.objectStore('jobs')
      if (id) store.delete(id)
      else store.clear()
      tx.oncomplete = () => resolve()
      tx.onabort = () => reject(tx.error)
      tx.onerror = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}
