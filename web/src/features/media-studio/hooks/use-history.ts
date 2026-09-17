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
import { useCallback, useEffect, useState } from 'react'

import {
  clearHistoryEntries,
  deleteHistoryEntry,
  listHistoryEntries,
  saveHistoryEntry,
  sortHistoryEntries,
} from '../lib/history-storage'
import type { HistoryEntry } from '../types'

interface UseHistoryReturn {
  entries: HistoryEntry[]
  loading: boolean
  storageAvailable: boolean
  saveEntry: (entry: HistoryEntry) => void
  removeEntry: (id: string) => void
  clearEntries: () => void
}

/**
 * 本地生成历史状态：挂载时从 IndexedDB 加载；
 * 写操作先乐观更新内存状态，再用存储层返回的列表校准。
 * 存储不可用（如隐私模式禁用 IndexedDB）时静默降级为仅内存历史，
 * storageAvailable 置 false 供 UI 提示。
 */
export function useHistory(): UseHistoryReturn {
  const [entries, setEntries] = useState<HistoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [storageAvailable, setStorageAvailable] = useState(true)

  useEffect(() => {
    let cancelled = false
    const load = async () => {
      try {
        const list = await listHistoryEntries()
        if (!cancelled) {
          setEntries(list)
        }
      } catch {
        if (!cancelled) {
          setStorageAvailable(false)
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [])

  const saveEntry = useCallback((entry: HistoryEntry) => {
    setEntries((current) => sortHistoryEntries([entry, ...current]))
    saveHistoryEntry(entry)
      .then(setEntries)
      .catch(() => setStorageAvailable(false))
  }, [])

  const removeEntry = useCallback((id: string) => {
    setEntries((current) => current.filter((entry) => entry.id !== id))
    deleteHistoryEntry(id)
      .then(setEntries)
      .catch(() => setStorageAvailable(false))
  }, [])

  const clearEntries = useCallback(() => {
    setEntries([])
    clearHistoryEntries()
      .then(setEntries)
      .catch(() => setStorageAvailable(false))
  }, [])

  return {
    entries,
    loading,
    storageAvailable,
    saveEntry,
    removeEntry,
    clearEntries,
  }
}
