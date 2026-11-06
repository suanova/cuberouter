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
import { act, renderHook } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'

import { fetchAgentTask, submitAgentTask } from '../api'
import { useDshTask } from '../hooks/use-dsh-task'
import { createConversation } from '../lib/conversations'
import type { DshConversation } from '../types'

vi.mock('../api', () => ({
  submitAgentTask: vi.fn(),
  fetchAgentTask: vi.fn(),
  getRequestErrorMessage: vi.fn((error: unknown) => String(error)),
}))

function completedPoll(sessionId: string, reply: string) {
  return {
    data: {
      task_id: 'task-1',
      status: 'SUCCESS',
      data: { ok: true, data: { status: 'completed', reply, sessionId } },
    },
  }
}

function renderTaskHook() {
  let conversation = createConversation('dsh-model')
  const updateConversation = vi.fn(
    (_id: string, updater: (current: DshConversation) => DshConversation) => {
      conversation = updater(conversation)
    }
  )
  const hook = renderHook(() => useDshTask(updateConversation))
  return {
    ...hook,
    updateConversation,
    getConversation: () => conversation,
  }
}

describe('useDshTask', () => {
  it('submits the first turn without a session and keeps the one echoed by dsh', async () => {
    vi.mocked(submitAgentTask).mockResolvedValue({ task_id: 'task-1' })
    vi.mocked(fetchAgentTask).mockResolvedValue(
      completedPoll('sess-1', 'done') as never
    )
    const hook = renderTaskHook()

    await act(async () => {
      await hook.result.current.sendPrompt(hook.getConversation(), 'hello')
    })

    const request = vi.mocked(submitAgentTask).mock.calls[0][0]
    expect(request.model).toBe('dsh-model')
    expect(request.prompt).toBe('hello')
    // 首轮续传字段必须是空值，否则 dsh 会尝试恢复一个不存在的会话。
    expect(request.sessionId).toBeUndefined()

    const conversation = hook.getConversation()
    expect(conversation.sessionId).toBe('sess-1')
    expect(conversation.turns).toHaveLength(1)
    expect(conversation.turns[0].taskId).toBe('task-1')
    expect(conversation.turns[0].status).toBe('completed')
    expect(conversation.turns[0].reply).toBe('done')
  })

  it('passes the stored session id and preset on the next turn', async () => {
    vi.mocked(submitAgentTask).mockResolvedValue({ task_id: 'task-2' })
    vi.mocked(fetchAgentTask).mockResolvedValue(
      completedPoll('sess-1', 'again') as never
    )
    const hook = renderTaskHook()

    await act(async () => {
      await hook.result.current.sendPrompt(hook.getConversation(), 'first')
    })
    hook.getConversation().preset = 'code'

    await act(async () => {
      await hook.result.current.sendPrompt(hook.getConversation(), 'second')
    })

    const request = vi.mocked(submitAgentTask).mock.calls[1][0]
    expect(request.sessionId).toBe('sess-1')
    expect(request.preset).toBe('code')
  })

  it('marks the turn failed when the gateway reports FAILURE', async () => {
    vi.mocked(submitAgentTask).mockResolvedValue({ task_id: 'task-3' })
    vi.mocked(fetchAgentTask).mockResolvedValue({
      data: {
        task_id: 'task-3',
        status: 'FAILURE',
        fail_reason: 'upstream exploded',
        data: null,
      },
    } as never)
    const hook = renderTaskHook()

    await act(async () => {
      await hook.result.current.sendPrompt(hook.getConversation(), 'hello')
    })

    const turn = hook.getConversation().turns[0]
    expect(turn.status).toBe('failed')
    expect(turn.error).toBe('upstream exploded')
  })

  it('resumes polling a running turn left over from before a refresh', async () => {
    vi.mocked(fetchAgentTask).mockResolvedValue(
      completedPoll('sess-2', 'resumed') as never
    )
    const hook = renderTaskHook()
    const conversation = hook.getConversation()
    const turn = {
      id: 'turn-left-over',
      taskId: 'task-9',
      prompt: 'still running',
      status: 'running' as const,
      reply: '',
      toolCalls: [],
      transcript: [],
      createdAt: Date.now(),
    }
    conversation.turns = [turn]

    await act(async () => {
      await hook.result.current.resumeTask(conversation.id, turn)
    })

    const [updated] = hook.getConversation().turns
    expect(updated.status).toBe('completed')
    expect(updated.reply).toBe('resumed')
    expect(hook.getConversation().sessionId).toBe('sess-2')
    // 续轮询不能重新提交一次任务。
    expect(submitAgentTask).not.toHaveBeenCalled()
  })

  it('sends uploaded attachments with the turn and keeps them for display', async () => {
    vi.mocked(submitAgentTask).mockClear()
    vi.mocked(submitAgentTask).mockResolvedValue({ task_id: 'task-5' })
    vi.mocked(fetchAgentTask).mockResolvedValue(
      completedPoll('sess-3', 'read it') as never
    )
    const hook = renderTaskHook()
    const attachment = {
      name: 'report.pdf',
      path: '/data/uploads/1/ab-report.pdf',
      mimeType: 'application/pdf',
      size: 8,
    }

    await act(async () => {
      await hook.result.current.sendPrompt(hook.getConversation(), '看看附件', [
        attachment,
      ])
    })

    expect(vi.mocked(submitAgentTask).mock.calls[0][0].attachments).toEqual([
      attachment,
    ])
    expect(hook.getConversation().turns[0].attachments).toEqual([attachment])
  })

  it('leaves attachments off the request when none were uploaded', async () => {
    vi.mocked(submitAgentTask).mockClear()
    vi.mocked(submitAgentTask).mockResolvedValue({ task_id: 'task-6' })
    vi.mocked(fetchAgentTask).mockResolvedValue(
      completedPoll('sess-4', 'ok') as never
    )
    const hook = renderTaskHook()

    await act(async () => {
      await hook.result.current.sendPrompt(hook.getConversation(), 'hi')
    })

    expect(vi.mocked(submitAgentTask).mock.calls[0][0]).not.toHaveProperty(
      'attachments'
    )
    expect(hook.getConversation().turns[0].attachments).toBeUndefined()
  })

  it('stops waiting without resubmitting when the user cancels a stuck turn', async () => {
    vi.mocked(submitAgentTask).mockClear()
    vi.mocked(submitAgentTask).mockResolvedValue({ task_id: 'task-4' })
    // 卡在 dsh 里的轮次：轮询一直不返回，只有 abort 才能结束。
    vi.mocked(fetchAgentTask).mockImplementation(
      ((_taskId: string, signal?: AbortSignal) =>
        new Promise((_resolve, reject) => {
          signal?.addEventListener('abort', () =>
            reject(new DOMException('aborted', 'AbortError'))
          )
        })) as never
    )
    const hook = renderTaskHook()

    let sending: Promise<void> | undefined
    await act(async () => {
      sending = hook.result.current.sendPrompt(hook.getConversation(), '慢慢来')
      await Promise.resolve()
      await Promise.resolve()
    })

    expect(hook.getConversation().turns[0].status).toBe('running')
    expect(hook.result.current.isRunning(hook.getConversation().id)).toBe(true)

    await act(async () => {
      hook.result.current.cancel(hook.getConversation().id)
      await sending
    })

    const turn = hook.getConversation().turns[0]
    expect(turn.status).toBe('canceled')
    expect(turn.finishedAt).toBeGreaterThan(0)
    expect(hook.result.current.isRunning(hook.getConversation().id)).toBe(false)
    // 停止等待只中止本地轮询，不能再提交一次任务。
    expect(vi.mocked(submitAgentTask).mock.calls).toHaveLength(1)
  })
})
