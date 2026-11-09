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
import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  createConversation,
  loadConversations,
  saveConversations,
} from '../lib/conversations'
import type { DshConversation, DshConversationsState } from '../types'

type ConversationUpdater = (conversation: DshConversation) => DshConversation

function getInitialState(): DshConversationsState {
  const stored = loadConversations()
  if (stored) {
    return stored
  }
  const conversation = createConversation('')
  return { conversations: [conversation], activeId: conversation.id }
}

export function useDshConversations() {
  const [state, setState] = useState<DshConversationsState>(getInitialState)

  useEffect(() => {
    saveConversations(state)
  }, [state])

  const activeConversation = useMemo(
    () =>
      state.conversations.find((item) => item.id === state.activeId) ?? null,
    [state.activeId, state.conversations]
  )

  const updateConversation = useCallback(
    (id: string, updater: ConversationUpdater) => {
      setState((prev) => ({
        ...prev,
        conversations: prev.conversations.map((item) =>
          item.id === id ? { ...updater(item), updatedAt: Date.now() } : item
        ),
      }))
    },
    []
  )

  const updateActive = useCallback((updater: ConversationUpdater) => {
    setState((prev) => {
      if (!prev.activeId) return prev
      return {
        ...prev,
        conversations: prev.conversations.map((item) =>
          item.id === prev.activeId
            ? { ...updater(item), updatedAt: Date.now() }
            : item
        ),
      }
    })
  }, [])

  const createNew = useCallback((model: string, organizationId?: number) => {
    const conversation = createConversation(model, organizationId)
    setState((prev) => ({
      conversations: [conversation, ...prev.conversations],
      activeId: conversation.id,
    }))
    return conversation
  }, [])

  // 旧版本持久化的会话没有 organizationId：拿到当前解析结果后统一补写，
  // 让历史会话也固定在同一个组织实例上。
  const assignOrganization = useCallback((organizationId: number) => {
    if (!Number.isInteger(organizationId) || organizationId <= 0) {
      return
    }
    setState((prev) => {
      let changed = false
      const conversations = prev.conversations.map((item) => {
        if (item.organizationId && item.organizationId > 0) {
          return item
        }
        changed = true
        return { ...item, organizationId }
      })
      return changed ? { ...prev, conversations } : prev
    })
  }, [])

  const selectConversation = useCallback((id: string) => {
    setState((prev) => ({ ...prev, activeId: id }))
  }, [])

  const removeConversation = useCallback((id: string) => {
    setState((prev) => {
      const conversations = prev.conversations.filter((item) => item.id !== id)
      if (conversations.length === 0) {
        const conversation = createConversation('')
        return { conversations: [conversation], activeId: conversation.id }
      }
      return {
        conversations,
        activeId: prev.activeId === id ? conversations[0].id : prev.activeId,
      }
    })
  }, [])

  return {
    conversations: state.conversations,
    activeConversation,
    updateConversation,
    updateActive,
    createNew,
    assignOrganization,
    selectConversation,
    removeConversation,
  }
}
