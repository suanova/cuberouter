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
import { act, renderHook } from '@testing-library/react'
import { beforeEach, describe, expect, it } from 'vitest'

import { useDshConversations } from '../hooks/use-dsh-conversations'
import { TASK_STORAGE_KEY } from '../constants'

describe('useDshConversations organization stamping', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  it('backfills persisted conversations that predate the organization field', () => {
    window.localStorage.setItem(
      TASK_STORAGE_KEY,
      JSON.stringify({
        conversations: [
          {
            id: 'old',
            title: 'old',
            model: 'dsh-model',
            preset: 'standard',
            turns: [],
            updatedAt: 1,
          },
        ],
        activeId: 'old',
      })
    )
    const { result } = renderHook(() => useDshConversations())

    act(() => {
      result.current.assignOrganization(12)
    })

    expect(result.current.conversations[0]?.organizationId).toBe(12)
  })

  it('does not move a conversation that is already pinned to an organization', () => {
    const { result } = renderHook(() => useDshConversations())
    act(() => {
      result.current.assignOrganization(12)
      result.current.assignOrganization(13)
    })

    expect(result.current.conversations[0]?.organizationId).toBe(12)
  })
})
