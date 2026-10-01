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
import { beforeEach, describe, expect, test, vi } from 'vitest'

const { get, post } = vi.hoisted(() => ({ get: vi.fn(), post: vi.fn() }))

vi.mock('@/lib/api', () => ({
  api: { get, post },
}))

import {
  VIDEO_SUBMIT_TIMEOUT_MS,
  fetchVideoTask,
  normalizeTaskStatus,
  submitVideoTask,
  videoRequest,
} from '../video-api'
import type { VideoDraft } from '../video-types'

function textDraft(): VideoDraft {
  return {
    mode: 'text',
    model: 'viduq3-pro',
    prompt: '  a rabbit on the beach  ',
    duration: 5,
    resolution: '720p',
  }
}

function imageDraft(): VideoDraft {
  return {
    ...textDraft(),
    mode: 'image',
    model: 'kling-v2',
    image: {
      id: 'asset-1',
      mime: 'image/png',
      url: 'data:image/png;base64,iVBORw0KGgo',
    },
  }
}

describe('videoRequest', () => {
  test('sends the unified task body for text to video without an image field', () => {
    expect(videoRequest(textDraft())).toEqual({
      model: 'viduq3-pro',
      prompt: 'a rabbit on the beach',
      duration: 5,
      size: '720p',
    })
  })

  test('inlines the first frame as a data URL for image to video', () => {
    expect(videoRequest(imageDraft())).toEqual({
      model: 'kling-v2',
      prompt: 'a rabbit on the beach',
      duration: 5,
      size: '720p',
      image: 'data:image/png;base64,iVBORw0KGgo',
    })
  })

  test('drops the image field when image mode has no reference', () => {
    const draft: VideoDraft = {
      mode: 'image',
      model: 'kling-v2',
      prompt: 'x',
      duration: 5,
      resolution: '720p',
    }
    expect('image' in videoRequest(draft)).toBe(false)
  })
})

describe('submitVideoTask', () => {
  beforeEach(() => {
    post.mockReset()
  })

  test('posts to the session video endpoint and extracts the task id', async () => {
    post.mockResolvedValue({ data: { task_id: 'task_abc', status: 'queued' } })

    await expect(submitVideoTask(textDraft())).resolves.toBe('task_abc')
    expect(post).toHaveBeenCalledWith(
      '/pg/video/generations',
      expect.objectContaining({ model: 'viduq3-pro' }),
      expect.objectContaining({ timeout: VIDEO_SUBMIT_TIMEOUT_MS })
    )
  })

  test('accepts the id field as a task id fallback', async () => {
    post.mockResolvedValue({ data: { id: 'task_fallback' } })

    await expect(submitVideoTask(textDraft())).resolves.toBe('task_fallback')
  })

  test('rejects a submission response that carries no task id', async () => {
    post.mockResolvedValue({ data: { status: 'queued' } })

    await expect(submitVideoTask(textDraft())).rejects.toThrow(
      'The task was submitted without a task id.'
    )
  })
})

describe('normalizeTaskStatus', () => {
  test('maps gateway task statuses case-insensitively', () => {
    expect(normalizeTaskStatus('SUCCESS')).toBe('SUCCESS')
    expect(normalizeTaskStatus(' success ')).toBe('SUCCESS')
    expect(normalizeTaskStatus('IN_PROGRESS')).toBe('IN_PROGRESS')
    expect(normalizeTaskStatus('QUEUED')).toBe('QUEUED')
    expect(normalizeTaskStatus('SUBMITTED')).toBe('SUBMITTED')
    expect(normalizeTaskStatus('NOT_START')).toBe('NOT_START')
    expect(normalizeTaskStatus('FAILURE')).toBe('FAILURE')
  })

  test('maps common upstream aliases to terminal states', () => {
    expect(normalizeTaskStatus('FAILED')).toBe('FAILURE')
    expect(normalizeTaskStatus('COMPLETED')).toBe('SUCCESS')
    expect(normalizeTaskStatus('succeeded')).toBe('SUCCESS')
  })

  test('keeps unrecognized values as UNKNOWN so polling continues', () => {
    expect(normalizeTaskStatus('WEIRD')).toBe('UNKNOWN')
    expect(normalizeTaskStatus(undefined)).toBe('UNKNOWN')
    expect(normalizeTaskStatus(42)).toBe('UNKNOWN')
  })
})

describe('fetchVideoTask', () => {
  beforeEach(() => {
    get.mockReset()
  })

  test('prefers the upstream creations URL over the gateway result URL', async () => {
    get.mockResolvedValue({
      data: {
        code: 'success',
        data: {
          task_id: 'task_1',
          status: 'SUCCESS',
          result_url: 'https://cdn.example/fallback.mp4',
          progress: '100%',
          data: { creations: [{ url: 'https://cdn.example/creations.mp4' }] },
        },
      },
    })

    await expect(fetchVideoTask('task_1')).resolves.toEqual({
      status: 'SUCCESS',
      url: 'https://cdn.example/creations.mp4',
      progress: '100%',
      fail_reason: '',
      raw: expect.anything(),
    })
  })

  test('falls back to the gateway result URL when the task payload has no creations', async () => {
    get.mockResolvedValue({
      data: {
        code: 'success',
        data: {
          task_id: 'task_2',
          status: 'SUCCESS',
          result_url: 'https://cdn.example/fallback.mp4',
          data: null,
        },
      },
    })

    const state = await fetchVideoTask('task_2')
    expect(state.url).toBe('https://cdn.example/fallback.mp4')
    expect(state.status).toBe('SUCCESS')
  })

  test('surfaces the fail reason on failure tasks', async () => {
    get.mockResolvedValue({
      data: {
        code: 'success',
        data: {
          task_id: 'task_3',
          status: 'FAILURE',
          fail_reason: 'sensitive content detected',
        },
      },
    })

    const state = await fetchVideoTask('task_3')
    expect(state.status).toBe('FAILURE')
    expect(state.fail_reason).toBe('sensitive content detected')
  })

  test('treats a missing task payload as UNKNOWN rather than an error', async () => {
    get.mockResolvedValue({ data: { code: 'success', data: null } })

    await expect(fetchVideoTask('task_4')).resolves.toMatchObject({
      status: 'UNKNOWN',
      url: '',
    })
  })
})
