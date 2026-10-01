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
import { api } from '@/lib/api'

import { API_ENDPOINTS } from './constants'
import type { VideoDraft, VideoTaskState, VideoTaskStatus } from './video-types'
import { newId } from './workflow-api'

/** 提交接口的超时只覆盖「排队 + 建任务」，生成本身是异步的，不需要 10 分钟。 */
export const VIDEO_SUBMIT_TIMEOUT_MS = 60 * 1000
export const VIDEO_FETCH_TIMEOUT_MS = 30 * 1000
/** 本地历史里单条视频字节的上限（含 base64 之外的原始字节），超过则不入库。 */
export const MAX_VIDEO_BYTES = 300 * 1024 * 1024

export interface VideoSubmitResponse {
  task_id?: unknown
  id?: unknown
  [key: string]: unknown
}

interface TaskData {
  status?: unknown
  fail_reason?: unknown
  progress?: unknown
  result_url?: unknown
  data?: unknown
}

export interface VideoFetchResponse {
  code?: unknown
  message?: unknown
  data?: TaskData
}

/**
 * 统一任务体（与 /v1/video/generations 的网关任务 API 一致）：
 * - model / prompt / duration(秒) / size(分辨率) 是所有渠道的公共字段
 * - 图生视频把首帧参考图作为内联 data URL 放进 image 字段（单图）
 */
export function videoRequest(draft: VideoDraft): Record<string, unknown> {
  const body: Record<string, unknown> = {
    model: draft.model,
    prompt: draft.prompt.trim(),
    duration: draft.duration,
    size: draft.resolution,
  }
  if (draft.mode === 'image' && draft.image) {
    body.image = draft.image.url
  }
  return body
}

/** 提交视频生成任务，返回可用于轮询的 task_id。 */
export async function submitVideoTask(draft: VideoDraft): Promise<string> {
  const res = await api.post<VideoSubmitResponse>(
    API_ENDPOINTS.VIDEO_GENERATIONS,
    videoRequest(draft),
    { timeout: VIDEO_SUBMIT_TIMEOUT_MS, skipErrorHandler: true }
  )
  const submitted = res.data
  const taskId =
    (typeof submitted?.task_id === 'string' && submitted.task_id) ||
    (typeof submitted?.id === 'string' && submitted.id) ||
    ''
  if (!taskId) {
    throw new Error('The task was submitted without a task id.')
  }
  return taskId
}

/**
 * 归一化任务状态：上游/网关状态值大小写不敏感地映射到统一枚举；
 * 无法识别的值按 UNKNOWN 处理（继续轮询，直到任务过期前给出最终态）。
 */
export function normalizeTaskStatus(value: unknown): VideoTaskStatus {
  if (typeof value !== 'string') return 'UNKNOWN'
  const status = value.trim().toUpperCase()
  if (
    status === 'NOT_START' ||
    status === 'SUBMITTED' ||
    status === 'QUEUED' ||
    status === 'IN_PROGRESS' ||
    status === 'SUCCESS' ||
    status === 'FAILURE'
  ) {
    return status
  }
  if (status === 'FAILED' || status === 'ERROR') return 'FAILURE'
  if (status === 'COMPLETED' || status === 'DONE' || status === 'SUCCEEDED') {
    return 'SUCCESS'
  }
  return 'UNKNOWN'
}

/**
 * 从轮询响应提取视频地址：优先上游 creations[0].url（各家任务载荷自带），
 * 回退网关 result_url。两者都取不到时 url 为空。
 */
function extractVideoUrl(data: TaskData | undefined): string {
  const creations: unknown =
    data && typeof data.data === 'object' && data.data !== null
      ? (data.data as Record<string, unknown>).creations
      : undefined
  if (Array.isArray(creations) && creations.length > 0) {
    const first = creations[0] as Record<string, unknown> | null
    if (first && typeof first.url === 'string' && first.url) {
      return first.url
    }
  }
  return typeof data?.result_url === 'string' ? data.result_url : ''
}

/** 轮询一次任务状态；code 非 success 或 404 按失败处理（错误信息透出 fail_reason）。 */
export async function fetchVideoTask(
  taskId: string
): Promise<VideoTaskState> {
  const res = await api.get<VideoFetchResponse>(
    `${API_ENDPOINTS.VIDEO_GENERATIONS}/${taskId}`,
    { timeout: VIDEO_FETCH_TIMEOUT_MS, skipErrorHandler: true }
  )
  const body = res.data
  const data = body?.data
  const rawStatus = normalizeTaskStatus(data?.status)
  const failReason =
    typeof data?.fail_reason === 'string' ? data.fail_reason : ''
  const message = typeof body?.message === 'string' ? body.message : ''
  return {
    status: rawStatus,
    url: extractVideoUrl(data),
    progress: typeof data?.progress === 'string' ? data.progress : '',
    fail_reason:
      rawStatus === 'FAILURE' ? failReason || message || 'Task failed.' : '',
    raw: body,
  }
}

/**
 * 把上游视频下载为自包含 Blob 供本地历史入库。
 * 只带凭据无关的 fetch（credentials: omit）：上游是签名临时地址，
 * 网关凭据过去也会被拒；体积超限直接抛错，调用方回退到「不入库」而不是失败。
 */
export async function downloadVideo(url: string): Promise<Blob> {
  let parsed: URL
  try {
    parsed = new URL(url, window.location.href)
  } catch {
    throw new Error('Unsupported video URL.')
  }
  if (!['https:', 'http:'].includes(parsed.protocol) || parsed.username) {
    throw new Error('Unsupported video URL.')
  }
  const response = await fetch(parsed.href, {
    credentials: 'omit',
    referrerPolicy: 'no-referrer',
  })
  if (!response.ok) {
    throw new Error('Could not download video for local history.')
  }
  const blob = await response.blob()
  if (blob.size > MAX_VIDEO_BYTES) {
    throw new Error('This video is too large for local history.')
  }
  if (!blob.size) {
    throw new Error('Could not download video for local history.')
  }
  return blob
}

/** 生成视频历史的本地主键（与图片历史同规则：非安全上下文不用 randomUUID）。 */
export function newVideoJobId(): string {
  return newId()
}
