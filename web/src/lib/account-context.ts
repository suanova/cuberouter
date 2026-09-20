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
// 账号上下文（个人 / 组织）是跨模块的应用级状态，和 auth-session 同一层次：
// 每个请求都要带上它，每个页面都要能读它，所以放在 lib 而不是某个 feature 里。
// 状态本身在 stores/account-context-store.ts，这里放「怎么和它说话」。
import { z } from 'zod'

import { api } from '@/lib/http-client'
import {
  ACCOUNT_CONTEXT_TYPE,
  useAccountContextStore,
} from '@/stores/account-context-store'

export {
  ACCOUNT_CONTEXT_ID_HEADER,
  ACCOUNT_CONTEXT_TYPE,
  ACCOUNT_CONTEXT_TYPE_HEADER,
  getAccountContextCacheKey,
  getAccountContextHeaders,
} from '@/stores/account-context-store'

export const accountContextTypeSchema = z.enum(['personal', 'organization'])
export type AccountContextType = z.infer<typeof accountContextTypeSchema>

// 和后端 OrganizationActorCapabilities 逐字段对应。界面按这些开关决定按钮和区块的
// 显隐，规则本身仍然只在后端（service/organization_policy.go）算一次。
export const organizationCapabilitiesSchema = z.object({
  can_view_organization: z.boolean(),
  can_view_organization_wide_data: z.boolean(),
  can_view_organization_usage: z.boolean(),
  can_view_members_limited: z.boolean(),
  can_view_organization_tokens: z.boolean(),
  can_view_organization_logs: z.boolean(),
  can_update_organization: z.boolean(),
  can_disable_organization: z.boolean(),
  can_enable_organization: z.boolean(),
  can_manage_members: z.boolean(),
  can_transfer_owner: z.boolean(),
  can_add_members_directly: z.boolean(),
  can_exit_organization: z.boolean(),
  can_view_invites: z.boolean(),
  can_create_invites: z.boolean(),
  can_revoke_invites: z.boolean(),
  can_manage_all_tokens: z.boolean(),
  can_modify_organization_group: z.boolean(),
  can_dissolve_organization: z.boolean(),
  can_view_audit: z.boolean(),
  can_view_organization_billing_summary: z.boolean(),
  show_return_organization_center: z.boolean(),
  show_return_personal_center: z.boolean(),
})
export type OrganizationCapabilities = z.infer<
  typeof organizationCapabilitiesSchema
>

export const accountContextSchema = z.object({
  type: accountContextTypeSchema,
  id: z.number(),
  name: z.string(),
  slug: z.string().optional(),
  role: z.string().optional(),
  status: z.string().optional(),
  access_mode: z.string().optional(),
  capabilities: organizationCapabilitiesSchema.optional(),
})
export type AccountContext = z.infer<typeof accountContextSchema>

export interface AccountContextsPayload {
  current: AccountContext | null
  contexts: AccountContext[]
}

export interface AccountContextsResponse {
  success: boolean
  message?: string
  data?: AccountContextsPayload
}

export interface AccountContextResponse {
  success: boolean
  message?: string
  code?: string
  data?: AccountContext
}

/** 当前是否处在组织上下文里。个人上下文下 current 为 null。 */
export function isOrganizationContext(): boolean {
  const current = useAccountContextStore.getState().accountContext.current
  return current?.type === ACCOUNT_CONTEXT_TYPE.ORGANIZATION
}

/**
 * 服务端返回的 `current` 是权威值：列表只包含我仍在的活跃组织，而
 * ResolveCurrentAccountContext 在存的组织不可用时会自己回退到个人并把结果持久化。
 * 所以这里直接采纳它，而不是保留本地选中的组织——组织被解散后不会留下一个
 * 每次都 403 的选中项。
 */
export async function refreshAccountContexts(): Promise<AccountContextsPayload> {
  const { accountContext } = useAccountContextStore.getState()
  accountContext.setStatus('loading')
  try {
    const res = await api.get<AccountContextsResponse>('/api/account-contexts', {
      // 上下文接口自身不该再触发上下文错误处理，否则失败会绕回自己。
      skipErrorHandler: true,
      skipBusinessError: true,
      skipAccountContext: true,
    })
    const payload: AccountContextsPayload = {
      current: res.data?.data?.current ?? null,
      contexts: res.data?.data?.contexts ?? [],
    }
    useAccountContextStore
      .getState()
      .accountContext.setContexts(payload.current, payload.contexts)
    return payload
  } catch (error) {
    useAccountContextStore.getState().accountContext.setStatus('error')
    throw error
  }
}

/** 切换当前上下文。服务端确认成功后才更新本地状态。 */
export async function switchAccountContext(
  type: AccountContextType,
  id: number
): Promise<AccountContext> {
  const res = await api.put<AccountContextResponse>(
    '/api/account-contexts/current',
    { type, id },
    { skipAccountContext: true }
  )
  const context = res.data?.data
  if (!context) {
    throw new Error(res.data?.message || 'Failed to switch account context')
  }
  useAccountContextStore.getState().accountContext.setCurrent(context)
  return context
}
