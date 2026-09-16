/*
Copyright (C) 2023-2026 QuantumNous

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
import { HISTORY_KEY, HISTORY_LIMIT } from '../constants'
import type { HistoryEntry } from '../types'

function readAll(): HistoryEntry[] {
  if (typeof window === 'undefined') {
    return []
  }
  try {
    const raw = window.localStorage.getItem(HISTORY_KEY)
    if (!raw) {
      return []
    }
    const parsed: unknown = JSON.parse(raw)
    if (!Array.isArray(parsed)) {
      return []
    }
    return parsed as HistoryEntry[]
  } catch {
    return []
  }
}

function writeAll(entries: HistoryEntry[]): void {
  try {
    window.localStorage.setItem(HISTORY_KEY, JSON.stringify(entries))
  } catch {
    // 存储不可用（隐私模式、配额等）：历史仅存在于当前会话内存
  }
}

/** 读取本浏览器保存的最近生成记录（新→旧，最多 HISTORY_LIMIT 条）。 */
export function loadHistory(): HistoryEntry[] {
  return readAll().slice(0, HISTORY_LIMIT)
}

/** 追加一条记录并返回更新后的列表（超出上限的旧记录被丢弃）。 */
export function saveHistoryEntry(entry: HistoryEntry): HistoryEntry[] {
  const next = [entry, ...readAll()].slice(0, HISTORY_LIMIT)
  writeAll(next)
  return next
}

/** 清空本地历史记录。 */
export function clearHistory(): HistoryEntry[] {
  try {
    window.localStorage.removeItem(HISTORY_KEY)
  } catch {
    // noop
  }
  return []
}
