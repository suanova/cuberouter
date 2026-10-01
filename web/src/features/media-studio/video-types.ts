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
import type { StudioAsset } from './workflow-types'

/** 视频创作模式：文生视频或图生视频（首帧参考）。 */
export type VideoMode = 'text' | 'image'

export interface VideoDraft {
  mode: VideoMode
  model: string
  prompt: string
  /** 时长（秒），统一任务体字段 duration。 */
  duration: number
  /** 分辨率（如 "720p"），统一任务体字段 size。 */
  resolution: string
  /** 图生视频的首帧参考图；文生视频必须为空。 */
  image?: StudioAsset
}

/** 后端任务状态（与 model.TaskStatus 一致，见 /pg/video/generations 轮询响应）。 */
export type VideoTaskStatus =
  | 'NOT_START'
  | 'SUBMITTED'
  | 'QUEUED'
  | 'IN_PROGRESS'
  | 'SUCCESS'
  | 'FAILURE'
  | 'UNKNOWN'

/** 单次轮询归一化后的视频任务状态。 */
export interface VideoTaskState {
  status: VideoTaskStatus
  /** 成功时的视频地址（上游 creations 或网关 result_url）。 */
  url: string
  /** 上游进度串（如 "100%"），没有则为空。 */
  progress: string
  /** 失败原因（仅 FAILURE 时有值）。 */
  fail_reason: string
  raw: unknown
}

/** 本地视频历史条目：视频字节以 ArrayBuffer 存进 IndexedDB，跨会话可还原播放与下载。 */
export interface VideoJob {
  id: string
  task_id: string
  created_at: number
  request: VideoDraft
  elapsed_ms: number
  /** 完成时从上游下载的视频原始字节；缺失时只能用可能过期的上游网址。 */
  video?: ArrayBuffer
  /** 本地入库失败时的展示回退地址（上游临时网址，可能过期）。 */
  video_url?: string
  video_mime: string
}
