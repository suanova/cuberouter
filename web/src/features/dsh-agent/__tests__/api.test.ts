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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import { fetchAgentTask, getUserModels, submitAgentTask } from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn() },
}))

describe('dsh agent api', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.post).mockReset()
  })

  it('omits sessionId on the first turn so dsh starts a fresh session', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { task_id: 't1' } })

    await submitAgentTask({ model: 'dsh-model', prompt: 'hello' })

    expect(api.post).toHaveBeenCalledWith(
      '/pg/agent/tasks',
      { model: 'dsh-model', prompt: 'hello' },
      { skipErrorHandler: true }
    )
  })

  it('sends the continuation fields once a session exists', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { task_id: 't2' } })

    await submitAgentTask({
      model: 'dsh-model',
      prompt: 'next',
      sessionId: 'sess-1',
      preset: 'ptc',
    })

    expect(api.post).toHaveBeenCalledWith(
      '/pg/agent/tasks',
      {
        model: 'dsh-model',
        prompt: 'next',
        sessionId: 'sess-1',
        preset: 'ptc',
      },
      { skipErrorHandler: true }
    )
  })

  it('fetches a task by id through the playground endpoint', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { code: 'success' } })

    await fetchAgentTask('task-1')

    expect(api.get).toHaveBeenCalledWith('/pg/agent/tasks/task-1', {
      signal: undefined,
      skipErrorHandler: true,
    })
  })

  it('maps the user model list and tolerates a failed response', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: true, data: ['dsh'] },
    })
    await expect(getUserModels()).resolves.toEqual([
      { label: 'dsh', value: 'dsh' },
    ])
    // 只列任务插件渠道托管的模型，否则下拉里会混进普通聊天模型。
    expect(api.get).toHaveBeenCalledWith('/api/user/models', {
      params: { endpoint: 'agent', plugin: 'dsh' },
    })

    vi.mocked(api.get).mockResolvedValue({ data: { success: false } })
    await expect(getUserModels()).resolves.toEqual([])
  })
})
