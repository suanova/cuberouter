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
import { create } from 'zustand'
import { persist } from 'zustand/middleware'

import type { AccountContext } from '@/lib/account-context'

// 上下文在请求上的表示形式跟着状态走：http-client 的拦截器要在没有 React 环境的
// 情况下同步取到它，而 lib/account-context.ts 又依赖 http-client 发请求。把这份
// 约定放在 store 里，两边就都只依赖 store，不会有模块环。
export const ACCOUNT_CONTEXT_TYPE_HEADER = 'X-Account-Context-Type'
export const ACCOUNT_CONTEXT_ID_HEADER = 'X-Account-Context-Id'

export const ACCOUNT_CONTEXT_TYPE = {
  PERSONAL: 'personal',
  ORGANIZATION: 'organization',
} as const

export type AccountContextsStatus = 'idle' | 'loading' | 'ready' | 'error'

interface AccountContextState {
  accountContext: {
    /** 当前生效的上下文。个人上下文下为 null，等价于「没声明」，后端按个人处理。 */
    current: AccountContext | null
    /** 可切换的上下文（个人 + 我仍在的活跃组织），由服务端给出。 */
    contexts: AccountContext[]
    status: AccountContextsStatus
    setStatus: (status: AccountContextsStatus) => void
    setContexts: (
      current: AccountContext | null,
      contexts: AccountContext[]
    ) => void
    setCurrent: (current: AccountContext | null) => void
    reset: () => void
  }
}

/**
 * 当前上下文要持久化，因为请求头是在 axios 拦截器里同步读出来的：等
 * `GET /api/account-contexts` 回来再补，页面刚打开时的几个请求会以个人上下文发出，
 * 组织写接口会直接 403。列表和状态不持久化，每次启动都以服务端返回的为准。
 */
export const useAccountContextStore = create<AccountContextState>()(
  persist(
    (set) => ({
      accountContext: {
        current: null,
        contexts: [],
        status: 'idle',
        setStatus: (status) =>
          set((state) => ({
            ...state,
            accountContext: { ...state.accountContext, status },
          })),
        setContexts: (current, contexts) =>
          set((state) => ({
            ...state,
            accountContext: {
              ...state.accountContext,
              current,
              contexts,
              status: 'ready',
            },
          })),
        setCurrent: (current) =>
          set((state) => ({
            ...state,
            accountContext: { ...state.accountContext, current },
          })),
        reset: () =>
          set((state) => ({
            ...state,
            accountContext: {
              ...state.accountContext,
              current: null,
              contexts: [],
              status: 'idle',
            },
          })),
      },
    }),
    {
      name: 'account-context',
      partialize: (state) => ({
        accountContext: { current: state.accountContext.current },
      }),
      // 默认合并是浅拷贝，会把上面那份 action 一起覆盖掉（partialize 只留了 current），
      // 所以这里手写合并，只取回 current。
      merge: (persistedState, currentState) => {
        const persisted = persistedState as
          | { accountContext?: { current?: AccountContext | null } }
          | undefined
        return {
          ...currentState,
          accountContext: {
            ...currentState.accountContext,
            current: persisted?.accountContext?.current ?? null,
          },
        }
      },
    }
  )
)

/**
 * 当前上下文对应的请求头。
 *
 * 个人上下文下返回空对象：后端 `AccountContext` 中间件把「没带请求头」直接当成个人
 * （middleware/account_context.go 的 resolveRequestAccountContext），显式声明个人反而
 * 要额外保证 id 等于自己的 user id，没有必要。
 */
export function getAccountContextHeaders(): Record<string, string> {
  const current = useAccountContextStore.getState().accountContext.current
  if (current?.type !== ACCOUNT_CONTEXT_TYPE.ORGANIZATION) return {}
  return {
    [ACCOUNT_CONTEXT_TYPE_HEADER]: ACCOUNT_CONTEXT_TYPE.ORGANIZATION,
    [ACCOUNT_CONTEXT_ID_HEADER]: String(current.id),
  }
}

/**
 * 请求去重用的上下文标识。
 *
 * GET 去重是按 `sid:url?params` 做的（见 lib/http-client.ts），而上下文走请求头、
 * 不进这个键。不并进去，切到组织后立刻发起的同 URL 请求会拿到上一个上下文还在
 * 途中的响应。
 */
export function getAccountContextCacheKey(): string {
  const current = useAccountContextStore.getState().accountContext.current
  if (current?.type !== ACCOUNT_CONTEXT_TYPE.ORGANIZATION) {
    return ACCOUNT_CONTEXT_TYPE.PERSONAL
  }
  return `${ACCOUNT_CONTEXT_TYPE.ORGANIZATION}:${current.id}`
}
