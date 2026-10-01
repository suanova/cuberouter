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
import type { VideoJob } from '../video-types'

const MAX_ENTRIES = 20
const MAX_BYTES = 400 * 1024 * 1024

// 与图片历史相同：独立数据库按登录账号隔离，避免切换账号后看到别人的视频。
// 视频字节以 ArrayBuffer 入库（structured clone 原生支持，无 base64 膨胀，
// 也兼容测试环境的 fake-indexeddb）；显示时由调用方包成 Blob 生成 object URL。
async function database(owner: number): Promise<IDBDatabase> {
  if (!Number.isSafeInteger(owner) || owner <= 0) {
    throw new Error('Sign in to use Media Studio.')
  }
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(`cuberouter-video-studio-${owner}`, 1)
    request.onupgradeneeded = () =>
      request.result.createObjectStore('jobs', { keyPath: 'id' })
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
}

/** 条目体积 = 视频原始字节 + 元数据 JSON 的 UTF-16 上界估算。 */
function jobSize(job: VideoJob): number {
  const meta = {
    id: job.id,
    task_id: job.task_id,
    created_at: job.created_at,
    request: job.request,
    elapsed_ms: job.elapsed_ms,
    video_mime: job.video_mime,
  }
  return (job.video?.byteLength ?? 0) + JSON.stringify(meta).length * 2
}

export async function listVideoJobs(owner: number): Promise<VideoJob[]> {
  const db = await database(owner)
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction('jobs', 'readonly')
      const request = tx.objectStore('jobs').getAll()
      tx.oncomplete = () =>
        resolve(
          (request.result as VideoJob[]).sort(
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

export async function saveVideoJob(
  owner: number,
  job: VideoJob
): Promise<void> {
  if (jobSize(job) > MAX_BYTES) {
    throw new Error('This video is too large for local history.')
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
        const entries = (all.result as VideoJob[]).sort(
          (a, b) => b.created_at - a.created_at
        )
        for (const [index, entry] of entries.entries()) {
          bytes += jobSize(entry)
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

export async function deleteVideoJob(
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
