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
import type { HistoryEntry } from '../types'

const DB_NAME = 'media-studio-history'
const DB_VERSION = 1
const STORE_NAME = 'entries'

/**
 * 历史条目上限。单张 1024² PNG 的 base64 约 2MB，50 条 ≈ 100MB，
 * 在 IndexedDB 配额（通常为磁盘的 50%）内很宽裕。
 * 不用 localStorage API 的原因：其配额仅约 5MB，放不下两张原图。
 */
export const MAX_HISTORY_ENTRIES = 50

function openDatabase(): Promise<IDBDatabase> {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open(DB_NAME, DB_VERSION)
    request.onupgradeneeded = () => {
      const db = request.result
      if (!db.objectStoreNames.contains(STORE_NAME)) {
        db.createObjectStore(STORE_NAME, { keyPath: 'id' })
      }
    }
    request.onsuccess = () => resolve(request.result)
    request.onerror = () => {
      request.result?.close()
      reject(request.error ?? new Error('Failed to open history database'))
    }
  })
}

function requestToPromise<T>(request: IDBRequest<T>): Promise<T> {
  return new Promise((resolve, reject) => {
    request.onsuccess = () => resolve(request.result)
    request.onerror = () =>
      reject(request.error ?? new Error('History storage operation failed'))
  })
}

/**
 * 等待事务提交。注意：请求 success 早于事务 complete，
 * 在 complete 前 close 数据库会中止事务、丢失写入。
 */
function transactionComplete(tx: IDBTransaction): Promise<void> {
  return new Promise((resolve, reject) => {
    tx.oncomplete = () => resolve()
    tx.onerror = () =>
      reject(tx.error ?? new Error('History transaction failed'))
    tx.onabort = () =>
      reject(tx.error ?? new Error('History transaction aborted'))
  })
}

export function sortHistoryEntries(entries: HistoryEntry[]): HistoryEntry[] {
  return [...entries].sort((a, b) => b.createdAt - a.createdAt)
}

async function readAll(db: IDBDatabase): Promise<HistoryEntry[]> {
  const tx = db.transaction(STORE_NAME, 'readonly')
  const store = tx.objectStore(STORE_NAME)
  const entries = await requestToPromise(
    store.getAll() as IDBRequest<HistoryEntry[]>
  )
  await transactionComplete(tx)
  return sortHistoryEntries(entries)
}

/** 读取全部历史条目（新→旧）。 */
export async function listHistoryEntries(): Promise<HistoryEntry[]> {
  const db = await openDatabase()
  try {
    return await readAll(db)
  } finally {
    db.close()
  }
}

/**
 * 保存一条历史；超出 MAX_HISTORY_ENTRIES 时按时间裁剪最旧条目。
 * 返回保存后的完整列表（新→旧），供调用方直接更新状态。
 */
export async function saveHistoryEntry(
  entry: HistoryEntry
): Promise<HistoryEntry[]> {
  const db = await openDatabase()
  try {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    const store = tx.objectStore(STORE_NAME)
    store.put(entry)
    const count = await requestToPromise(store.count())
    if (count > MAX_HISTORY_ENTRIES) {
      const all = await requestToPromise(
        store.getAll() as IDBRequest<HistoryEntry[]>
      )
      const overflow = all
        .sort((a, b) => a.createdAt - b.createdAt)
        .slice(0, count - MAX_HISTORY_ENTRIES)
      for (const old of overflow) {
        store.delete(old.id)
      }
    }
    await transactionComplete(tx)
    return await readAll(db)
  } finally {
    db.close()
  }
}

/** 删除指定条目，返回剩余列表（新→旧）。 */
export async function deleteHistoryEntry(id: string): Promise<HistoryEntry[]> {
  const db = await openDatabase()
  try {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).delete(id)
    await transactionComplete(tx)
    return await readAll(db)
  } finally {
    db.close()
  }
}

/** 清空全部历史，返回空列表。 */
export async function clearHistoryEntries(): Promise<HistoryEntry[]> {
  const db = await openDatabase()
  try {
    const tx = db.transaction(STORE_NAME, 'readwrite')
    tx.objectStore(STORE_NAME).clear()
    await transactionComplete(tx)
    return []
  } finally {
    db.close()
  }
}
