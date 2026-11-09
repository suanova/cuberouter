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
import { DEFAULT_PRESET, TASK_STORAGE_KEY } from '../constants'
import type { DshConversation, DshConversationsState } from '../types'

const TITLE_MAX_LENGTH = 28

function newId(): string {
  // crypto.randomUUID 在 http://<ip>:<port> 的非安全上下文里不存在。
  if (
    typeof crypto !== 'undefined' &&
    typeof crypto.randomUUID === 'function'
  ) {
    return crypto.randomUUID()
  }
  return `dsh-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
}

export function createConversation(
  model: string,
  organizationId?: number
): DshConversation {
  const now = Date.now()
  return {
    id: newId(),
    title: '',
    model,
    preset: DEFAULT_PRESET,
    ...(organizationId && organizationId > 0 ? { organizationId } : {}),
    turns: [],
    updatedAt: now,
  }
}

export function conversationTitle(prompt: string): string {
  const title = prompt.replaceAll(/\s+/g, ' ').trim()
  return title.length > TITLE_MAX_LENGTH
    ? `${title.slice(0, TITLE_MAX_LENGTH)}…`
    : title
}

export function loadConversations(): DshConversationsState | null {
  if (typeof window === 'undefined') {
    return null
  }
  try {
    const raw = window.localStorage.getItem(TASK_STORAGE_KEY)
    if (!raw) {
      return null
    }
    const parsed = JSON.parse(raw) as Partial<DshConversationsState>
    if (!Array.isArray(parsed.conversations)) {
      return null
    }
    const conversations = parsed.conversations.filter(
      (item): item is DshConversation =>
        Boolean(item) &&
        typeof item.id === 'string' &&
        typeof item.model === 'string' &&
        Array.isArray(item.turns)
    )
    const activeId =
      typeof parsed.activeId === 'string' &&
      conversations.some((item) => item.id === parsed.activeId)
        ? parsed.activeId
        : (conversations[0]?.id ?? null)
    return { conversations, activeId }
  } catch {
    return null
  }
}

export function saveConversations(state: DshConversationsState): void {
  if (typeof window === 'undefined') {
    return
  }
  try {
    window.localStorage.setItem(TASK_STORAGE_KEY, JSON.stringify(state))
  } catch {
    // 存储配额满或隐私模式禁用时不阻塞页面使用。
  }
}
