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
import type {
  DshTaskPayload,
  DshToolCall,
  DshTranscriptEntry,
  DshTurnStatus,
} from '../types'

/**
 * 网关把插件提交时返回的 taskData 和轮询时的上游响应都原样写进 Task.Data。
 * 两种形状分别是 `{taskId, sessionId}` 和 `{ok, data}`；统一取到内层任务载荷。
 */
export function unwrapTaskPayload(value: unknown): DshTaskPayload | undefined {
  if (!value || typeof value !== 'object') {
    return undefined
  }
  const record = value as Record<string, unknown>
  if (record.data && typeof record.data === 'object') {
    return record.data as DshTaskPayload
  }
  return record as DshTaskPayload
}

export function resolveTurnStatus(
  gatewayStatus: string | undefined,
  payload: DshTaskPayload | undefined
): DshTurnStatus {
  if (gatewayStatus === 'FAILURE') {
    return 'failed'
  }
  if (gatewayStatus === 'SUCCESS') {
    if (payload?.status === 'max-tokens') {
      return 'max-tokens'
    }
    if (payload?.status === 'canceled') {
      return 'canceled'
    }
    return 'completed'
  }
  return 'running'
}

export function readToolCalls(
  payload: DshTaskPayload | undefined
): DshToolCall[] {
  if (!Array.isArray(payload?.toolCalls)) {
    return []
  }
  return payload.toolCalls.filter(
    (item): item is DshToolCall =>
      Boolean(item) && typeof (item as DshToolCall).name === 'string'
  )
}

export function readTranscript(
  payload: DshTaskPayload | undefined
): DshTranscriptEntry[] {
  if (!Array.isArray(payload?.transcript)) {
    return []
  }
  return payload.transcript.filter(
    (item): item is DshTranscriptEntry =>
      Boolean(item) &&
      typeof (item as DshTranscriptEntry).role === 'string' &&
      typeof (item as DshTranscriptEntry).text === 'string'
  )
}

/**
 * dsh-api 只在内存里保存 sessionId → handle 的映射；harness 重启、任务超过
 * 24h 被清理或换到别的实例后，续聊会以 unknown_session 失败，页面需要提示
 * 用户另开一个会话而不是反复重试。
 */
export function isSessionExpiredError(message: string | undefined): boolean {
  if (!message) {
    return false
  }
  const normalized = message.toLowerCase()
  return (
    normalized.includes('unknown_session') ||
    normalized.includes('unknown session') ||
    normalized.includes('session not found')
  )
}

export function readErrorMessage(
  payload: DshTaskPayload | undefined,
  fallback?: string
): string | undefined {
  if (typeof payload?.error === 'string' && payload.error) {
    return payload.error
  }
  if (payload?.error && typeof payload.error === 'object') {
    if (typeof payload.error.message === 'string' && payload.error.message) {
      return payload.error.message
    }
  }
  return fallback
}
