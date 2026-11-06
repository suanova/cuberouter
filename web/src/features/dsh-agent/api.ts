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
import { parseTaskArtifactsResponse } from '@/features/usage-logs/lib/task-artifacts'
import type {
  TaskArtifactProjection,
  TaskArtifactsResponse,
} from '@/features/usage-logs/types'
import { api } from '@/lib/api'

import { AGENT_MODEL_FILTER, API_ENDPOINTS } from './constants'
import type {
  AgentTaskFetchResponse,
  AgentTaskSubmitResponse,
  DshModelOption,
} from './types'

export type SubmitAgentTaskBody = {
  model: string
  prompt: string
  sessionId?: string
  preset?: string
  signal?: AbortSignal
}

/**
 * 提交一轮 Agent 任务。响应只保证有 task_id；sessionId 需要再查一次任务详情，
 * 因为网关对任务插件统一返回精简的提交回执。
 */
export async function submitAgentTask(
  body: SubmitAgentTaskBody
): Promise<AgentTaskSubmitResponse> {
  const res = await api.post(
    API_ENDPOINTS.AGENT_TASKS,
    {
      model: body.model,
      prompt: body.prompt,
      ...(body.sessionId ? { sessionId: body.sessionId } : {}),
      ...(body.preset ? { preset: body.preset } : {}),
    },
    { signal: body.signal, skipErrorHandler: true } as Record<string, unknown>
  )
  return res.data
}

/**
 * 查询任务详情。data.data 是网关保存的上游原始响应（含 reply/transcript/
 * toolCalls/sessionId）。
 */
export async function fetchAgentTask(
  taskId: string,
  signal?: AbortSignal
): Promise<AgentTaskFetchResponse> {
  const res = await api.get(`${API_ENDPOINTS.AGENT_TASKS}/${taskId}`, {
    signal,
    skipErrorHandler: true,
  } as Record<string, unknown>)
  return res.data
}

/**
 * 本轮产物清单。网关把插件上报的文件投影成带签名下载地址的条目，页面直接渲染
 * 成附件卡片；未结算或没有产物的任务返回空数组。
 */
export async function fetchTaskArtifacts(
  taskId: string
): Promise<TaskArtifactProjection> {
  const response = await api.get<TaskArtifactsResponse>(
    `${API_ENDPOINTS.TASK_ARTIFACTS}/${encodeURIComponent(taskId)}/artifacts`,
    { skipBusinessError: true, skipErrorHandler: true } as Record<
      string,
      unknown
    >
  )
  return parseTaskArtifactsResponse(response.data)
}

export async function getUserModels(): Promise<DshModelOption[]> {
  const res = await api.get(API_ENDPOINTS.USER_MODELS, {
    params: AGENT_MODEL_FILTER,
  })
  const { data } = res
  if (!data.success || !Array.isArray(data.data)) {
    return []
  }
  return data.data.map((model: string) => ({ label: model, value: model }))
}

export function getRequestErrorMessage(error: unknown): string {
  const candidate = error as {
    response?: { data?: { error?: { message?: string }; message?: string } }
    message?: string
  }
  return (
    candidate.response?.data?.error?.message ??
    candidate.response?.data?.message ??
    candidate.message ??
    'Request failed'
  )
}
