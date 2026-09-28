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
import { isAxiosError } from 'axios'

import { api } from '@/lib/api'

import { blobDataURL } from '../workflow-api'
import type { WorkflowDraft, WorkflowJob } from '../workflow-types'

export interface QueueStatus {
  id: string
  state:
    | 'queued'
    | 'running'
    | 'completed'
    | 'failed'
    | 'expired'
    | 'released'
    | 'cancelled'
  ahead?: number
  elapsed_seconds: number
  error?: string
}
export interface PendingImageJob {
  key: string
  epoch: string
  draft: WorkflowDraft
  created: number
  id?: string
}

// One small account-scoped IndexedDB record includes the draft, never credentials.
export async function pendingJob(
  owner: number,
  value?: PendingImageJob | null
): Promise<PendingImageJob | undefined> {
  if (!Number.isSafeInteger(owner) || owner <= 0) {
    throw new Error('Sign in to use Media Studio.')
  }
  const db = await new Promise<IDBDatabase>((resolve, reject) => {
    const request = indexedDB.open(`cuberouter-qwen-pending-${owner}`, 1)
    request.onupgradeneeded = () => request.result.createObjectStore('pending')
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => reject(request.error)
  })
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(
        'pending',
        value === undefined ? 'readonly' : 'readwrite'
      )
      const store = tx.objectStore('pending')
      if (value === null) store.delete('active')
      else if (value !== undefined) store.put(value, 'active')
      const request = store.get('active')
      tx.oncomplete = () =>
        resolve(request.result as PendingImageJob | undefined)
      tx.onabort = () => reject(tx.error)
      tx.onerror = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}

export function qwenParameters(draft: WorkflowDraft) {
  const [width, height] = draft.size.split('x').map(Number)
  const images = draft.references.map((asset) => {
    if (!/^data:image\/(png|jpeg|webp);base64,/i.test(asset.url)) {
      throw new Error('Choose a reference image.')
    }
    return asset.url.slice(asset.url.indexOf(',') + 1)
  })
  return {
    model: '2.1',
    mode: draft.mode,
    prompt: draft.prompt,
    user_prompt: draft.user_prompt ?? draft.prompt,
    width,
    height,
    steps: draft.steps ?? 40,
    cfg: draft.cfg ?? 1,
    seed: draft.seed ?? 42,
    images: draft.mode === 'edit' ? images : [],
  }
}

export async function submitQwenJob(
  owner: number,
  draft: WorkflowDraft
): Promise<PendingImageJob> {
  if (await pendingJob(owner)) {
    throw new Error(
      'A previous image request needs review. Reconnect before submitting again.'
    )
  }
  const { data } = await api.get<{ epoch: string }>(
    '/api/image-studio/session',
    { skipErrorHandler: true }
  )
  const key = Array.from(crypto.getRandomValues(new Uint8Array(16)), (byte) =>
    byte.toString(16).padStart(2, '0')
  ).join('')
  const pending = {
    key,
    epoch: data.epoch,
    draft: structuredClone(draft),
    created: Date.now(),
  }
  const parameters = qwenParameters(draft)
  if (new Blob([JSON.stringify(parameters)]).size > 16 * 1024 * 1024) {
    throw new Error('Reference images exceed the 16 MB request limit.')
  }
  await pendingJob(owner, pending)
  // Persist before POST. On a lost response, reconnect looks up this key; it never resubmits.
  try {
    const response = await api.post<QueueStatus>(
      '/api/image-studio/jobs',
      parameters,
      {
        headers: { 'Idempotency-Key': key, 'X-Trial-Epoch': data.epoch },
        skipErrorHandler: true,
      }
    )
    const accepted = { ...pending, id: response.data.id }
    await pendingJob(owner, accepted)
    return accepted
  } catch (error) {
    // A definite rejection precedes acceptance. Transport/5xx errors remain recoverable.
    if (
      isAxiosError(error) &&
      [400, 401, 403, 409, 413, 415, 429].includes(error.response?.status ?? 0)
    ) {
      await pendingJob(owner, null)
    }
    throw error
  }
}

export async function collectQwenJob(
  pending: PendingImageJob,
  signal: AbortSignal,
  onStatus: (status: QueueStatus) => void
): Promise<WorkflowJob> {
  let response
  try {
    response = await api.get<QueueStatus>(
      `/api/image-studio/requests/${pending.key}`,
      { signal, skipErrorHandler: true }
    )
  } catch (error) {
    if (isAxiosError(error) && error.response?.status === 404) {
      onStatus({ id: pending.id ?? '', state: 'expired', elapsed_seconds: 0 })
    }
    throw error
  }
  let status = response.data
  while (true) {
    signal.throwIfAborted()
    onStatus(status)
    if (status.state === 'completed') break
    if (status.state !== 'queued' && status.state !== 'running') {
      throw new Error(
        'Image job ended. Your draft is kept; review it before trying again.'
      )
    }
    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        clearTimeout(timer)
        reject(new DOMException('Aborted', 'AbortError'))
      }
      const timer = setTimeout(() => {
        signal.removeEventListener('abort', abort)
        resolve()
      }, 1500)
      signal.addEventListener('abort', abort, { once: true })
    })
    status = (
      await api.get<QueueStatus>(`/api/image-studio/jobs/${status.id}`, {
        signal,
        skipErrorHandler: true,
      })
    ).data
  }
  const result = await api.get<Blob>(
    `/api/image-studio/jobs/${status.id}/result`,
    { signal, responseType: 'blob', skipErrorHandler: true }
  )
  return {
    id: status.id,
    created_at: pending.created,
    request: pending.draft,
    elapsed_ms: status.elapsed_seconds * 1000,
    request_id: status.id,
    images: [
      { id: status.id, mime: 'image/png', url: await blobDataURL(result.data) },
    ],
  }
}

export async function releaseQwenResult(
  owner: number,
  id: string
): Promise<void> {
  await api.post(
    `/api/image-studio/jobs/${id}/ack`,
    {},
    { skipErrorHandler: true }
  )
  await pendingJob(owner, null)
}
