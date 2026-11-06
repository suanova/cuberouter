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
import { beforeEach, describe, expect, it } from 'vitest'

import { TASK_STORAGE_KEY } from '../constants'
import {
  conversationTitle,
  createConversation,
  loadConversations,
  saveConversations,
} from '../lib/conversations'

describe('conversationTitle', () => {
  it('collapses whitespace before truncating', () => {
    expect(conversationTitle('  fix   the\n\nbug  ')).toBe('fix the bug')
  })

  it('truncates long prompts with an ellipsis', () => {
    const title = conversationTitle('a'.repeat(40))
    expect(title).toHaveLength(29)
    expect(title.endsWith('…')).toBe(true)
  })
})

describe('conversation storage', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('round-trips conversations and the active id', () => {
    const first = createConversation('model-a')
    const second = createConversation('model-b')
    saveConversations({
      conversations: [first, second],
      activeId: second.id,
    })

    expect(loadConversations()).toEqual({
      conversations: [first, second],
      activeId: second.id,
    })
  })

  it('falls back to the first conversation when the stored active id is gone', () => {
    const conversation = createConversation('model-a')
    saveConversations({
      conversations: [conversation],
      activeId: 'missing',
    })

    expect(loadConversations()?.activeId).toBe(conversation.id)
  })

  it('ignores corrupted or non-conversation entries', () => {
    window.localStorage.setItem(TASK_STORAGE_KEY, '{not json')
    expect(loadConversations()).toBeNull()

    window.localStorage.setItem(
      TASK_STORAGE_KEY,
      JSON.stringify({ conversations: [{ id: 'x' }], activeId: 'x' })
    )
    expect(loadConversations()).toEqual({ conversations: [], activeId: null })
  })
})
