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

import { fetchAgentTask, fetchDshOrganization, getUserModels, submitAgentTask, uploadDshAttachment } from '../api'

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

  // 每个组织一个容器：会话固定的组织必须随提交与上传一起发出去，否则后端只
  // 能按账号上下文猜，同一会话可能落到别的实例。
  it('sends the pinned organization header when submitting a task', async () => {
    vi.mocked(api.post).mockResolvedValue({ data: { task_id: 't3' } })

    await submitAgentTask({
      model: 'dsh-model',
      prompt: 'hello',
      organizationId: 12,
    })

    expect(api.post).toHaveBeenCalledWith(
      '/pg/agent/tasks',
      { model: 'dsh-model', prompt: 'hello' },
      {
        skipErrorHandler: true,
        headers: { 'X-DSH-Organization-Id': '12' },
      }
    )
  })

  it('sends the pinned organization header when uploading an attachment', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { name: 'a.txt', path: '/data/uploads/7/a.txt', size: 3 },
    })
    const file = new File(['abc'], 'a.txt', { type: 'text/plain' })

    const attachment = await uploadDshAttachment(file, { organizationId: 12 })

    expect(attachment.path).toBe('/data/uploads/7/a.txt')
    const config = vi.mocked(api.post).mock.calls[0]?.[2] as {
      headers?: Record<string, string>
    }
    expect(config.headers).toEqual({ 'X-DSH-Organization-Id': '12' })
  })

  it('resolves the organization used to stamp a new conversation', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: { organization_id: 12, organization_name: 'Zhongshan' },
      },
    })

    await expect(fetchDshOrganization()).resolves.toEqual({
      organizationId: 12,
      organizationName: 'Zhongshan',
    })
    expect(api.get).toHaveBeenCalledWith('/api/dsh/organization', {
      skipBusinessError: true,
      skipErrorHandler: true,
    })
  })

  it('returns null when the account has no DSH organization', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { success: false } })

    await expect(fetchDshOrganization()).resolves.toBeNull()
  })
})
