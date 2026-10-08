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
import type { VideoDraft } from '../video-types'

/** 提交时持久化的进行中任务，页面中途离开后凭 task_id 恢复轮询。 */
export interface PendingVideoTask {
  task_id: string
  draft: VideoDraft
  /** 提交时间戳，恢复后计时器从它继续走，elapsed 不虚标。 */
  started_at: number
}

function storageKey(owner: number): string {
  return `cuberouter-video-pending-${owner}`
}

function requireOwner(owner: number): void {
  if (!Number.isSafeInteger(owner) || owner <= 0) {
    throw new Error('Sign in to use Media Studio.')
  }
}

export function loadPendingVideoTask(owner: number): PendingVideoTask | undefined {
  requireOwner(owner)
  try {
    const raw = window.localStorage.getItem(storageKey(owner))
    if (!raw) return undefined
    const parsed = JSON.parse(raw) as Partial<PendingVideoTask>
    const draft = parsed.draft
    if (
      typeof parsed.task_id !== 'string' ||
      !parsed.task_id ||
      !draft ||
      typeof draft !== 'object' ||
      (draft.mode !== 'text' && draft.mode !== 'image') ||
      typeof draft.model !== 'string' ||
      typeof draft.prompt !== 'string' ||
      typeof draft.duration !== 'number' ||
      typeof draft.resolution !== 'string'
    ) {
      return undefined
    }
    return {
      task_id: parsed.task_id,
      // 持久化时本就不含首帧字节（见 savePendingVideoTask），这里显式置空。
      draft: {
        mode: draft.mode,
        model: draft.model,
        prompt: draft.prompt,
        duration: draft.duration,
        resolution: draft.resolution,
      },
      started_at:
        typeof parsed.started_at === 'number' ? parsed.started_at : Date.now(),
    }
  } catch {
    // 损坏的 JSON 或 storage 不可用：当作没有待恢复任务。
    return undefined
  }
}

/**
 * 按账号写入一条待恢复任务。draft 丢弃 image 字段：首帧 data URL 上限 10MB，
 * 超过 localStorage 约 5MB 配额会直接抛错；恢复后图生视频任务只少「首帧
 * 对比」展示，视频字节与入库不受影响。写满配额时静默放弃——本次会话内任务
 * 仍正常轮询，只是刷新后不能恢复。
 */
export function savePendingVideoTask(
  owner: number,
  record: PendingVideoTask
): void {
  requireOwner(owner)
  const safe: PendingVideoTask = {
    ...record,
    draft: { ...record.draft, image: undefined },
  }
  try {
    window.localStorage.setItem(storageKey(owner), JSON.stringify(safe))
  } catch {
    // 见函数注释：放弃持久化是可控降级。
  }
}

export function clearPendingVideoTask(owner: number): void {
  requireOwner(owner)
  try {
    window.localStorage.removeItem(storageKey(owner))
  } catch {
    // storage 不可用时无需处理。
  }
}
