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

import { serverImageSteps } from '../constants'
import { blobDataURL, newId, prepareWorkflowRequest } from '../workflow-api'
import type { WorkflowDraft, WorkflowJob } from '../workflow-types'
import { studioDatabase } from './studio-storage'

export interface ImageQueueConfig {
  image_queue_enabled?: boolean
  image_queue_models?: string[]
  image_queue_access?: boolean
}
export interface ImageJobStatus {
  id: string
  key: string
  epoch: string
  state:
    | 'queued'
    | 'running'
    | 'cancelling'
    | 'cancelled'
    | 'completed'
    | 'failed'
    | 'expired'
    | 'released'
    | 'reconnecting'
  ahead: number
  error?: string
  recovery_pending: boolean
  cancel_requested: boolean
  result_committed?: boolean
  created_at: number
}
export interface PendingStudioImage {
  key: string
  epoch: string
  draft: WorkflowDraft
  started: number
  cancelRequested?: boolean
}
export const IMAGE_JOBS = '/api/media-studio/image-jobs'

export async function pendingStudioImage(
  owner: number,
  value?: PendingStudioImage | null,
  options: { claim?: boolean; expectedKey?: string } = {}
): Promise<PendingStudioImage | undefined> {
  const db = await studioDatabase(owner)
  try {
    return await new Promise((resolve, reject) => {
      const tx = db.transaction(
        'pending',
        value === undefined ? 'readonly' : 'readwrite'
      )
      const store = tx.objectStore('pending')
      let result: PendingStudioImage | undefined
      let conflict = false
      const read = store.get('image')
      read.onsuccess = () => {
        result = read.result as PendingStudioImage | undefined
        if (
          value === undefined ||
          (options.expectedKey && result?.key !== options.expectedKey)
        ) {
          return
        }
        if (options.claim && result) {
          conflict = true
          tx.abort()
          return
        }
        if (value === null) {
          store.delete('image')
          result = undefined
        } else {
          if (options.claim) store.add(value, 'image')
          else store.put(value, 'image')
          result = value
        }
      }
      tx.oncomplete = () => resolve(result)
      tx.onabort = () =>
        reject(
          conflict
            ? new Error(
                'Reconnect to your previous image job before generating again.'
              )
            : tx.error
        )
      tx.onerror = () => reject(tx.error)
    })
  } finally {
    db.close()
  }
}

export function imageJobMessage(code?: string): string {
  switch (code) {
    case 'output_blocked':
      return 'This image did not pass output review. Try a different idea.'
    case 'output_check_unavailable':
      return 'Image review is unavailable. No image was released.'
    case 'studio_audit_unavailable':
      return 'Image auditing is unavailable. No image was released.'
    case 'studio_account_busy':
      return 'Your account already has a pending image job. Reconnect before trying again.'
    case 'studio_access_denied':
      return 'Your account does not have access to the image queue.'
    case 'studio_capacity_unavailable':
      return 'Image workers are temporarily unavailable. Try again later.'
    case 'studio_scope_changed':
      return 'Your image access or billing account changed. Review your draft and try again.'
    case 'studio_epoch_changed':
      return 'Could not reach the gateway for this image job. Reconnect to recover it.'
    case 'studio_queue_full':
      return 'The image queue is full. Try again later.'
    default:
      return 'The image job could not complete. Your draft has been kept.'
  }
}

export async function submitStudioImage(
  owner: number,
  draft: WorkflowDraft
): Promise<PendingStudioImage> {
  if (await pendingStudioImage(owner)) {
    throw new Error(
      'Reconnect to your previous image job before generating again.'
    )
  }
  const effective = {
    ...draft,
    steps: draft.steps ?? serverImageSteps(draft.quality),
    cfg: draft.cfg ?? 1,
    seed: draft.seed ?? 42,
  }
  const request = await prepareWorkflowRequest(effective, true)
  const body = { mode: draft.mode, user_prompt: draft.prompt, request }
  if (new Blob([JSON.stringify(body)]).size > 16 * 1024 * 1024) {
    throw new Error('Reference images are too large in total.')
  }
  const session = await api.get<{ epoch: string }>(`${IMAGE_JOBS}/session`, {
    skipErrorHandler: true,
  })
  const record: PendingStudioImage = {
    key: newId(),
    epoch: session.data.epoch,
    draft: structuredClone(effective),
    started: Date.now(),
  }
  // Save the identity before POST. A lost response must be looked up, never retried.
  await pendingStudioImage(owner, record, { claim: true })
  try {
    await api.post(IMAGE_JOBS, body, {
      headers: { 'Idempotency-Key': record.key, 'X-Trial-Epoch': record.epoch },
      skipErrorHandler: true,
    })
    return record
  } catch (error) {
    if (isAxiosError(error)) {
      const code: unknown = error.response?.data?.error?.code
      if (
        [400, 401, 403, 409, 413, 429].includes(error.response?.status ?? 0) ||
        code === 'studio_capacity_unavailable' ||
        code === 'studio_access_unavailable' ||
        code === 'output_check_unavailable'
      ) {
        await pendingStudioImage(owner, null, { expectedKey: record.key })
      }
      if (typeof code === 'string') throw new Error(imageJobMessage(code))
    }
    throw error
  }
}

export async function cancelStudioImage(
  owner: number,
  record: PendingStudioImage,
  status: ImageJobStatus
): Promise<void> {
  await pendingStudioImage(
    owner,
    { ...record, cancelRequested: true },
    { expectedKey: record.key }
  )
  await sendStudioCancellation(status.id, record.epoch)
}

function sendStudioCancellation(
  id: string,
  epoch: string,
  signal?: AbortSignal
) {
  return api.post(
    `${IMAGE_JOBS}/${id}/cancel`,
    {},
    {
      headers: { 'X-Trial-Epoch': epoch },
      signal,
      skipErrorHandler: true,
    }
  )
}

export async function collectStudioImage(
  owner: number,
  record: PendingStudioImage,
  signal: AbortSignal,
  onStatus: (status: ImageJobStatus) => void
): Promise<WorkflowJob | undefined> {
  const headers = { 'X-Trial-Epoch': record.epoch }
  const unavailable = (state: 'expired' | 'reconnecting') => {
    onStatus({
      id: '',
      key: record.key,
      epoch: record.epoch,
      state,
      ahead: 0,
      recovery_pending: false,
      cancel_requested: false,
      created_at: record.started,
    })
  }
  const handleLookupError = (error: unknown): never => {
    if (isAxiosError(error) && error.response?.status === 409) {
      unavailable('reconnecting')
      throw new Error(imageJobMessage('studio_epoch_changed'))
    }
    if (isAxiosError(error) && error.response?.status === 404) {
      unavailable('expired')
    }
    throw error
  }
  for (;;) {
    const response = await api
      .get<ImageJobStatus>(`${IMAGE_JOBS}/${encodeURIComponent(record.key)}`, {
        headers,
        signal,
        skipErrorHandler: true,
      })
      .catch(handleLookupError)
    const status = response.data
    if (status.epoch !== record.epoch) {
      unavailable('reconnecting')
      throw new Error(imageJobMessage('studio_epoch_changed'))
    }
    onStatus(status)
    if (status.state === 'cancelled' && !status.recovery_pending) return
    const latest = await pendingStudioImage(owner)
    if (
      latest?.key === record.key &&
      latest.cancelRequested &&
      !status.result_committed &&
      !status.cancel_requested &&
      ['queued', 'running', 'completed'].includes(status.state)
    ) {
      await sendStudioCancellation(status.id, record.epoch, signal).catch(
        handleLookupError
      )
      continue
    }
    if (status.state === 'completed') {
      const result = await api
        .get<Blob>(`${IMAGE_JOBS}/${status.id}/result`, {
          headers,
          responseType: 'blob',
          signal,
          skipErrorHandler: true,
        })
        .catch(handleLookupError)
      if (
        result.data.type !== 'image/png' ||
        result.data.size > 12 * 1024 * 1024
      ) {
        throw new Error('The model returned no images.')
      }
      const imageURL = await blobDataURL(result.data)
      const recovery = await pendingStudioImage(owner)
      if (recovery?.key !== record.key) {
        throw new Error(imageJobMessage())
      }
      if (recovery.cancelRequested && !status.result_committed) {
        await sendStudioCancellation(status.id, record.epoch, signal).catch(
          handleLookupError
        )
        continue
      }
      return {
        id: status.id,
        created_at: status.created_at,
        request: record.draft,
        images: [
          {
            id: newId(),
            mime: 'image/png',
            url: imageURL,
          },
        ],
        elapsed_ms: Date.now() - record.started,
        request_id: status.id,
      }
    }
    if (
      !['queued', 'running', 'cancelling', 'cancelled'].includes(status.state)
    ) {
      throw new Error(imageJobMessage(status.error))
    }
    await new Promise<void>((resolve, reject) => {
      const abort = () => {
        clearTimeout(timer)
        reject(signal.reason)
      }
      const timer = setTimeout(() => {
        signal.removeEventListener('abort', abort)
        resolve()
      }, 5000)
      signal.addEventListener('abort', abort, { once: true })
      if (signal.aborted) abort()
    })
  }
}

export async function acknowledgeStudioImage(
  owner: number,
  id: string,
  key: string,
  epoch: string
): Promise<void> {
  await api.post(
    `${IMAGE_JOBS}/${id}/ack`,
    {},
    {
      headers: { 'X-Trial-Epoch': epoch },
      skipErrorHandler: true,
    }
  )
  await pendingStudioImage(owner, null, { expectedKey: key })
}
