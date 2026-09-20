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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { AccountContext } from '@/lib/account-context'

import {
  ACCOUNT_CONTEXT_ID_HEADER,
  ACCOUNT_CONTEXT_TYPE_HEADER,
  getAccountContextCacheKey,
  getAccountContextHeaders,
  useAccountContextStore,
} from '../account-context-store'

function organizationContext(id = 7): AccountContext {
  return { type: 'organization', id, name: `Org ${id}`, role: 'member' }
}

describe('account context headers', () => {
  beforeEach(() => {
    useAccountContextStore.getState().accountContext.reset()
  })

  test('sends nothing for the personal context', () => {
    // 后端把「没有请求头」直接当成个人（middleware/account_context.go），所以个人
    // 上下文必须是不带请求头，而不是声明一次个人。
    expect(getAccountContextHeaders()).toEqual({})
    expect(getAccountContextCacheKey()).toBe('personal')
  })

  test('sends type and id for an organization context', () => {
    useAccountContextStore.getState().accountContext.setCurrent(organizationContext(7))

    expect(getAccountContextHeaders()).toEqual({
      [ACCOUNT_CONTEXT_TYPE_HEADER]: 'organization',
      [ACCOUNT_CONTEXT_ID_HEADER]: '7',
    })
    expect(getAccountContextCacheKey()).toBe('organization:7')
  })

  test('distinguishes organizations in the cache key', () => {
    const store = useAccountContextStore.getState()

    store.accountContext.setCurrent(organizationContext(7))
    const first = getAccountContextCacheKey()
    store.accountContext.setCurrent(organizationContext(8))

    expect(first).not.toBe(getAccountContextCacheKey())
  })
})

describe('account context state', () => {
  beforeEach(() => {
    useAccountContextStore.getState().accountContext.reset()
  })

  test('adopts the server current context and the switchable list', () => {
    const contexts = [organizationContext(1), organizationContext(2)]

    useAccountContextStore
      .getState()
      .accountContext.setContexts(contexts[1], contexts)

    const state = useAccountContextStore.getState().accountContext
    expect(state.current).toEqual(contexts[1])
    expect(state.contexts).toEqual(contexts)
    expect(state.status).toBe('ready')
  })

  test('falls back to personal when the server reports no current context', () => {
    // 存的组织被解散后，服务端会回退到个人；客户端必须跟着回退，否则每个请求
    // 都带着一个已经不可用的组织。
    useAccountContextStore
      .getState()
      .accountContext.setContexts(organizationContext(3), [])

    useAccountContextStore.getState().accountContext.setContexts(null, [])

    expect(getAccountContextHeaders()).toEqual({})
  })

  test('reset clears both the selection and the list', () => {
    useAccountContextStore
      .getState()
      .accountContext.setContexts(organizationContext(3), [
        organizationContext(3),
      ])

    useAccountContextStore.getState().accountContext.reset()

    expect(useAccountContextStore.getState().accountContext).toMatchObject({
      current: null,
      contexts: [],
      status: 'idle',
    })
  })
})

describe('account context persistence', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.resetModules()
  })

  test('restores the persisted current context and keeps the actions', async () => {
    localStorage.setItem(
      'account-context',
      JSON.stringify({
        state: { accountContext: { current: organizationContext(9) } },
        version: 0,
      })
    )

    const { useAccountContextStore: fresh } = await import(
      '../account-context-store'
    )

    const state = fresh.getState().accountContext
    expect(state.current).toEqual(organizationContext(9))
    // 列表不持久化，必须回到空，由服务端重新给。
    expect(state.contexts).toEqual([])
    // partialize 只留了 current，默认的浅合并会把 action 一起覆盖掉。
    expect(typeof state.setCurrent).toBe('function')
    expect(typeof state.reset).toBe('function')
  })

  test('starts personal when nothing was persisted', async () => {
    const { useAccountContextStore: fresh } = await import(
      '../account-context-store'
    )

    expect(fresh.getState().accountContext.current).toBeNull()
  })
})
