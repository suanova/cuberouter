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
import { useCallback, useEffect, useRef, useState } from 'react'

import { fetchAgentTask, getRequestErrorMessage, submitAgentTask } from '../api'
import { TASK_POLL_INTERVAL_MS } from '../constants'
import { conversationTitle } from '../lib/conversations'
import {
  readErrorMessage,
  readToolCalls,
  readTranscript,
  resolveTurnStatus,
  unwrapTaskPayload,
} from '../lib/payload'
import type { DshAttachment, DshConversation, DshTurn } from '../types'

const MAX_CONSECUTIVE_POLL_ERRORS = 5

type ConversationUpdater = (
  id: string,
  updater: (conversation: DshConversation) => DshConversation
) => void

function newTurnId(): string {
  return `turn-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`
}

function wait(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms)
  })
}

function updateTurn(
  conversation: DshConversation,
  turnId: string,
  updater: (turn: DshTurn) => DshTurn
): DshConversation {
  return {
    ...conversation,
    turns: conversation.turns.map((turn) =>
      turn.id === turnId ? updater(turn) : turn
    ),
  }
}

export function useDshTask(updateConversation: ConversationUpdater) {
  const controllersRef = useRef(new Map<string, AbortController>())
  const [runningIds, setRunningIds] = useState<string[]>([])

  useEffect(
    () => () => {
      controllersRef.current.forEach((controller) => controller.abort())
      controllersRef.current.clear()
    },
    []
  )

  const setRunning = useCallback((conversationId: string, running: boolean) => {
    setRunningIds((prev) => {
      if (running) {
        return prev.includes(conversationId) ? prev : [...prev, conversationId]
      }
      return prev.filter((id) => id !== conversationId)
    })
  }, [])

  const pollUntilSettled = useCallback(
    async (
      conversationId: string,
      turnId: string,
      taskId: string,
      controller: AbortController
    ) => {
      let pollErrors = 0
      while (!controller.signal.aborted) {
        let response
        try {
          response = await fetchAgentTask(taskId, controller.signal)
          pollErrors = 0
        } catch (error) {
          if (controller.signal.aborted) return
          pollErrors += 1
          if (pollErrors >= MAX_CONSECUTIVE_POLL_ERRORS) {
            const message = getRequestErrorMessage(error)
            updateConversation(conversationId, (current) =>
              updateTurn(current, turnId, (turn) => ({
                ...turn,
                status: 'failed',
                error: message,
                finishedAt: Date.now(),
              }))
            )
            return
          }
          await wait(TASK_POLL_INTERVAL_MS)
          continue
        }

        const task = response.data
        const payload = unwrapTaskPayload(task?.data)
        const status = resolveTurnStatus(task?.status, payload)
        const finished = status !== 'running'
        updateConversation(conversationId, (current) => {
          const withTurn = updateTurn(current, turnId, (turn) => ({
            ...turn,
            status,
            reply: payload?.reply ?? turn.reply,
            toolCalls: readToolCalls(payload),
            transcript: readTranscript(payload),
            turnEnd: payload?.turnEnd ?? turn.turnEnd,
            error: readErrorMessage(payload, task?.fail_reason),
            finishedAt: finished ? Date.now() : turn.finishedAt,
          }))
          return payload?.sessionId
            ? { ...withTurn, sessionId: payload.sessionId }
            : withTurn
        })

        if (finished) {
          return
        }
        await wait(TASK_POLL_INTERVAL_MS)
      }
    },
    [updateConversation]
  )

  const sendPrompt = useCallback(
    async (
      conversation: DshConversation,
      prompt: string,
      attachments: DshAttachment[] = []
    ) => {
      if (!conversation.model || controllersRef.current.has(conversation.id)) {
        return
      }

      const controller = new AbortController()
      controllersRef.current.set(conversation.id, controller)
      setRunning(conversation.id, true)

      const turnId = newTurnId()
      const createdAt = Date.now()
      updateConversation(conversation.id, (current) => ({
        ...current,
        title: current.title || conversationTitle(prompt),
        turns: [
          ...current.turns,
          {
            id: turnId,
            taskId: '',
            prompt,
            status: 'running',
            reply: '',
            toolCalls: [],
            transcript: [],
            ...(attachments.length ? { attachments } : {}),
            createdAt,
          },
        ],
      }))

      try {
        const submitted = await submitAgentTask({
          model: conversation.model,
          prompt,
          sessionId: conversation.sessionId,
          preset: conversation.preset || undefined,
          ...(attachments.length ? { attachments } : {}),
          signal: controller.signal,
        })
        const taskId = submitted.task_id ?? submitted.id ?? ''
        if (!taskId) {
          throw new Error('Task submission returned no task id')
        }
        updateConversation(conversation.id, (current) =>
          updateTurn(current, turnId, (turn) => ({ ...turn, taskId }))
        )

        await pollUntilSettled(conversation.id, turnId, taskId, controller)
      } catch (error) {
        if (controller.signal.aborted) {
          return
        }
        const message = getRequestErrorMessage(error)
        updateConversation(conversation.id, (current) =>
          updateTurn(current, turnId, (turn) => ({
            ...turn,
            status: 'failed',
            error: message,
            finishedAt: Date.now(),
          }))
        )
      } finally {
        // StrictMode 下挂载会执行两遍：只有当映射仍属于本轮控制器时才清理，
        // 否则会把第二轮刚设上的运行中状态误清掉。
        if (controllersRef.current.get(conversation.id) === controller) {
          controllersRef.current.delete(conversation.id)
          setRunning(conversation.id, false)
        }
      }
    },
    [pollUntilSettled, setRunning, updateConversation]
  )

  // 页面刷新会丢掉轮询循环，但网关仍在跟踪任务：带着已保存的 taskId 重新
  // 挂上轮询，就能拿回这一轮最终结果。
  const resumeTask = useCallback(
    async (conversationId: string, turn: DshTurn) => {
      if (!turn.taskId || controllersRef.current.has(conversationId)) {
        return
      }
      const controller = new AbortController()
      controllersRef.current.set(conversationId, controller)
      setRunning(conversationId, true)
      try {
        await pollUntilSettled(conversationId, turn.id, turn.taskId, controller)
      } catch (error) {
        if (controller.signal.aborted) {
          return
        }
        const message = getRequestErrorMessage(error)
        updateConversation(conversationId, (current) =>
          updateTurn(current, turn.id, (item) => ({
            ...item,
            status: 'failed',
            error: message,
            finishedAt: Date.now(),
          }))
        )
      } finally {
        if (controllersRef.current.get(conversationId) === controller) {
          controllersRef.current.delete(conversationId)
          setRunning(conversationId, false)
        }
      }
    },
    [pollUntilSettled, setRunning, updateConversation]
  )

  // 网关没有取消任务插件的接口：卡住的轮次只能停止等待——中止本地轮询并把它标成
  // 已取消，解锁输入框，用户可以去开新会话。
  const cancel = useCallback(
    (conversationId: string) => {
      const controller = controllersRef.current.get(conversationId)
      if (!controller) {
        return
      }
      controller.abort()
      controllersRef.current.delete(conversationId)
      setRunning(conversationId, false)
      updateConversation(conversationId, (current) => ({
        ...current,
        turns: current.turns.map((turn) =>
          turn.status === 'running'
            ? { ...turn, status: 'canceled' as const, finishedAt: Date.now() }
            : turn
        ),
      }))
    },
    [setRunning, updateConversation]
  )

  return {
    sendPrompt,
    resumeTask,
    cancel,
    isRunning: (conversationId: string | null | undefined) =>
      Boolean(conversationId) && runningIds.includes(conversationId as string),
  }
}
