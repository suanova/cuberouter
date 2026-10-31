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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { IDBFactory } from 'fake-indexeddb'
import type { ReactNode } from 'react'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { useWorkflow } from '../hooks/use-workflow'
import {
  IMAGE_JOBS,
  pendingStudioImage,
  type PendingStudioImage,
} from '../lib/image-jobs'
import * as storage from '../lib/studio-storage'
import { initialDraft } from '../lib/workflow'
import { workflowAPI } from '../workflow-api'

beforeEach(() => vi.stubGlobal('indexedDB', new IDBFactory()))

test.each(['queue enabled after initial load', 'configuration refresh fails'])(
  '%s never routes a fresh draft with stale configuration',
  async (scenario) => {
    let refresh = false
    const draft = { ...initialDraft, model: 'qwen-image-2.1', prompt: 'a cat' }
    vi.spyOn(api, 'get').mockImplementation(async (url) => {
      if (url === '/api/media-studio/config') {
        if (refresh && scenario === 'configuration refresh fails') {
          throw new Error('offline')
        }
        return {
          data: {
            image_queue_enabled: refresh,
            image_queue_models: refresh ? [draft.model] : [],
          },
        }
      }
      if (url.endsWith('/session')) return { data: { epoch: 'boot-a' } }
      if (url.endsWith('/result')) {
        return { data: new Blob(['approved'], { type: 'image/png' }) }
      }
      return {
        data: {
          id: 'fresh-job',
          epoch: 'boot-a',
          state: 'completed',
          ahead: 0,
          created_at: 1,
        },
      }
    })
    const post = vi.spyOn(api, 'post').mockResolvedValue({ data: {} })
    const generate = vi.spyOn(workflowAPI, 'generate')
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    })
    const wrapper = (props: { children: ReactNode }) => (
      <QueryClientProvider client={client}>
        {props.children}
      </QueryClientProvider>
    )
    const hook = renderHook(() => useWorkflow(1), { wrapper })
    await waitFor(() =>
      expect(hook.result.current.configuration.isSuccess).toBe(true)
    )
    refresh = true
    await act(async () => {
      const result = hook.result.current.generation.mutateAsync(draft)
      if (scenario === 'configuration refresh fails') {
        await expect(result).rejects.toThrow('configuration')
      } else {
        await expect(result).resolves.toMatchObject({
          job: { id: 'fresh-job' },
        })
      }
    })
    if (scenario === 'configuration refresh fails') {
      expect(post).not.toHaveBeenCalled()
    } else {
      expect(post).toHaveBeenCalledWith(
        IMAGE_JOBS,
        expect.objectContaining({
          request: expect.objectContaining({ model: draft.model }),
        }),
        expect.anything()
      )
    }
    expect(generate).not.toHaveBeenCalled()
    hook.unmount()
    client.clear()
  }
)

test('unavailable configuration withholds generation until a successful config fetch', async () => {
  const get = vi.spyOn(api, 'get').mockRejectedValue(new Error('offline'))
  const generate = vi.spyOn(workflowAPI, 'generate')
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
  const wrapper = (props: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
  const hook = renderHook(() => useWorkflow(1), { wrapper })
  await waitFor(() =>
    expect(hook.result.current.configuration.isError).toBe(true)
  )
  await act(async () => {
    await expect(
      hook.result.current.generation.mutateAsync(initialDraft)
    ).rejects.toThrow('configuration')
  })
  expect(generate).not.toHaveBeenCalled()
  expect(get).toHaveBeenCalledTimes(2)
  hook.unmount()
  client.clear()
})

test('a resumed result is acknowledged only after browser history commits', async () => {
  const record: PendingStudioImage = {
    key: 'recovery-key',
    epoch: 'boot-a',
    started: 1,
    draft: { ...initialDraft, model: 'qwen-image-2.1', prompt: 'a cat' },
  }
  await pendingStudioImage(1, record)
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/media-studio/config') {
      return {
        data: {
          image_queue_enabled: true,
          image_queue_models: [record.draft.model],
        },
      }
    }
    if (url.endsWith('/result')) {
      return { data: new Blob(['approved'], { type: 'image/png' }) }
    }
    return {
      data: {
        id: 'job-a',
        key: record.key,
        epoch: record.epoch,
        state: 'completed',
        ahead: 0,
        created_at: 1,
      },
    }
  })
  const post = vi.spyOn(api, 'post').mockResolvedValue({ data: {} })
  let releaseSave!: () => void
  const ready = new Promise<void>((resolve) => {
    releaseSave = resolve
  })
  const originalSave = storage.saveStudioJob
  const save = vi
    .spyOn(storage, 'saveStudioJob')
    .mockImplementation(async (...args) => {
      await ready
      return originalSave(...args)
    })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = (props: { children: ReactNode }) => (
    <QueryClientProvider client={client}>{props.children}</QueryClientProvider>
  )
  const hook = renderHook(() => useWorkflow(1), { wrapper })
  await waitFor(() => expect(save).toHaveBeenCalled())
  expect(post).not.toHaveBeenCalled()
  await act(async () => releaseSave())
  await waitFor(() =>
    expect(post).toHaveBeenCalledWith(
      `${IMAGE_JOBS}/job-a/ack`,
      {},
      { skipErrorHandler: true }
    )
  )
  expect((await storage.listStudioJobs(1))[0].id).toBe('job-a')
  hook.unmount()
  client.clear()
})
