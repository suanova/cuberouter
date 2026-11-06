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
export type DshTurnStatus =
  | 'running'
  | 'completed'
  | 'max-tokens'
  | 'failed'
  | 'canceled'

export type DshToolCall = {
  name: string
  callId?: string
}

export type DshTranscriptEntry = {
  role: string
  text: string
}

// 已落到网关共享卷的附件：path 是 dsh 容器内可直接读取的绝对路径，智能体
// 靠提示词里的这份清单找到文件。
export type DshAttachment = {
  name: string
  path: string
  mimeType?: string
  size?: number
}

export type DshUploadSettings = {
  enabled: boolean
  maxBytes: number
  maxFiles: number
  accept: string[]
}

export type DshTurn = {
  id: string
  taskId: string
  prompt: string
  status: DshTurnStatus
  reply: string
  toolCalls: DshToolCall[]
  transcript: DshTranscriptEntry[]
  attachments?: DshAttachment[]
  error?: string
  turnEnd?: string
  createdAt: number
  finishedAt?: number
}

export type DshConversation = {
  id: string
  title: string
  model: string
  preset: string
  sessionId?: string
  turns: DshTurn[]
  updatedAt: number
}

// dsh-api 的任务载荷：提交时是 { ok, data }，轮询时被网关原样写入 Task.Data。
export type DshTaskPayload = {
  taskId?: string
  sessionId?: string
  status?: string
  reply?: string
  transcript?: DshTranscriptEntry[]
  toolCalls?: DshToolCall[]
  turnEnd?: string
  error?: { code?: string; message?: string } | string
}

export type AgentTaskSubmitResponse = {
  id?: string
  task_id?: string
  status?: string
  model?: string
  created_at?: number
}

export type AgentTaskData = {
  data?: DshTaskPayload | null
}

// GET /pg/agent/tasks/:task_id 的响应：data 是统一 TaskDto，data.data 是插件
// 原始任务数据。
export type AgentTaskFetchResponse = {
  code?: string
  data?: {
    task_id?: string
    status?: string
    progress?: string
    fail_reason?: string
    data?: DshTaskPayload | { ok?: boolean; data?: DshTaskPayload } | null
    finished_time?: number
  }
}

export type DshModelOption = {
  label: string
  value: string
}

export type DshConversationsState = {
  conversations: DshConversation[]
  activeId: string | null
}
