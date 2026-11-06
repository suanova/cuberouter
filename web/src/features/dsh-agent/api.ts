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
  DshAttachment,
  DshModelOption,
  DshUploadSettings,
} from './types'

export type SubmitAgentTaskBody = {
  model: string
  prompt: string
  sessionId?: string
  preset?: string
  attachments?: DshAttachment[]
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
      ...(body.attachments?.length ? { attachments: body.attachments } : {}),
    },
    { signal: body.signal, skipErrorHandler: true } as Record<string, unknown>
  )
  return res.data
}

// 网关上传接口沿用服务端的 snake_case 字段，提交任务时再换成与 sessionId
// 一致的小驼峰形状。
type DshUploadSettingsResponse = {
  enabled?: boolean
  max_bytes?: number
  max_files?: number
  accept?: string[]
}

type DshAttachmentResponse = {
  name?: string
  path?: string
  mime_type?: string
  size?: number
}

/**
 * 附件能力依赖网关与 dsh 共享的数据卷；未配置时 enabled=false，页面不显示回形针。
 */
export async function fetchDshUploadSettings(): Promise<DshUploadSettings> {
  const res = await api.get<DshUploadSettingsResponse>(
    API_ENDPOINTS.DSH_UPLOAD_SETTINGS,
    { skipErrorHandler: true } as Record<string, unknown>
  )
  const data = res.data ?? {}
  return {
    enabled: data.enabled === true,
    maxBytes: Number(data.max_bytes) || 0,
    maxFiles: Number(data.max_files) || 1,
    accept: Array.isArray(data.accept) ? data.accept : [],
  }
}

/**
 * 单个文件一次请求：网关按用户目录落盘并返回 dsh 容器内的绝对路径，页面只负责
 * 在提交任务时把这份清单回传。
 */
export async function uploadDshAttachment(
  file: File,
  signal?: AbortSignal
): Promise<DshAttachment> {
  const form = new FormData()
  form.append('file', file)
  const res = await api.post<DshAttachmentResponse>(
    API_ENDPOINTS.DSH_UPLOADS,
    form,
    { signal, skipErrorHandler: true } as Record<string, unknown>
  )
  const data = res.data ?? {}
  if (!data.path) {
    throw new Error('Upload response has no path')
  }
  return {
    name: data.name || file.name,
    path: data.path,
    ...(data.mime_type ? { mimeType: data.mime_type } : {}),
    size: data.size ?? file.size,
  }
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
