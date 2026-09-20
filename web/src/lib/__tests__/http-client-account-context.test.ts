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
import type { AxiosRequestConfig, AxiosResponse } from 'axios'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '../http-client'
import { useAccountContextStore } from '@/stores/account-context-store'

const { toastError } = vi.hoisted(() => ({ toastError: vi.fn() }))

vi.mock('sonner', () => ({
  toast: { error: toastError, success: vi.fn() },
}))

function organizationContext(id = 7) {
  return { type: 'organization' as const, id, name: `Org ${id}`, role: 'member' }
}

function okResponse(config: AxiosRequestConfig): AxiosResponse {
  return {
    data: { success: true },
    status: 200,
    statusText: 'OK',
    headers: {},
    config: config as AxiosResponse['config'],
  }
}

function headerOf(
  config: AxiosRequestConfig | undefined,
  name: string
): unknown {
  const headers = config?.headers as Record<string, unknown> | undefined
  if (!headers) return undefined
  const lower = name.toLowerCase()
  for (const [key, value] of Object.entries(headers)) {
    if (key.toLowerCase() === lower) return value
  }
  return undefined
}

/** 装一个假的 adapter，把每次真正发出去的请求记下来。 */
function captureRequests(): AxiosRequestConfig[] {
  const seen: AxiosRequestConfig[] = []
  api.defaults.adapter = async (config) => {
    seen.push(config)
    return okResponse(config)
  }
  return seen
}

describe('account context request headers', () => {
  beforeEach(() => {
    toastError.mockReset()
    useAccountContextStore.getState().accountContext.reset()
  })

  test('attaches the organization context to business requests', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    const seen = captureRequests()

    await api.get('/api/organizations')

    expect(headerOf(seen[0], 'X-Account-Context-Type')).toBe('organization')
    expect(headerOf(seen[0], 'X-Account-Context-Id')).toBe('7')
  })

  test('omits the headers entirely in the personal context', async () => {
    const seen = captureRequests()

    await api.get('/api/user/self')

    expect(headerOf(seen[0], 'X-Account-Context-Type')).toBeUndefined()
    expect(headerOf(seen[0], 'X-Account-Context-Id')).toBeUndefined()
  })

  test('omits the headers for platform-admin requests', async () => {
    // /api/admin/* 是跨组织的，组织由路径里的 id 决定；带上上下文会让后端按另一个
    // 组织解析权限。
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    const seen = captureRequests()

    await api.get('/api/admin/organizations')

    expect(headerOf(seen[0], 'X-Account-Context-Type')).toBeUndefined()
  })

  test('honours an explicit opt-out', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    const seen = captureRequests()

    await api.get('/api/account-contexts', { skipAccountContext: true })

    expect(headerOf(seen[0], 'X-Account-Context-Type')).toBeUndefined()
  })
})

describe('get request deduplication', () => {
  beforeEach(() => {
    toastError.mockReset()
    useAccountContextStore.getState().accountContext.reset()
  })

  test('shares one in-flight request for the same context', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    const seen = captureRequests()

    const first = api.get('/api/user/self')
    const second = api.get('/api/user/self')

    expect(second).toBe(first)
    await first
    expect(seen).toHaveLength(1)
  })

  test('does not share an in-flight request across contexts', async () => {
    // 去重键原本只有 sid + url + params，请求头不在里面：切到组织后立刻发出的同 URL
    // 请求会命中个人上下文那次还在途中的响应。
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    const seen: AxiosRequestConfig[] = []
    const pendingResolvers: Array<(response: AxiosResponse) => void> = []
    api.defaults.adapter = (config) =>
      new Promise((resolve) => {
        seen.push(config)
        pendingResolvers.push(resolve)
      })

    const pending = api.get('/api/user/self')
    await vi.waitFor(() => expect(seen).toHaveLength(1))

    useAccountContextStore.getState().accountContext.setCurrent(null)
    const switched = api.get('/api/user/self')

    expect(switched).not.toBe(pending)
    await vi.waitFor(() => expect(seen).toHaveLength(2))
    for (const resolve of pendingResolvers) resolve(okResponse(seen[0]))
    await Promise.all([pending, switched])

    expect(headerOf(seen[0], 'X-Account-Context-Type')).toBe('organization')
    expect(headerOf(seen[1], 'X-Account-Context-Type')).toBeUndefined()
  })
})

describe('stale account context responses', () => {
  beforeEach(() => {
    toastError.mockReset()
    useAccountContextStore.getState().accountContext.reset()
  })

  function rejectWithCode(code: string, status: number): void {
    api.defaults.adapter = async (config) => {
      throw Object.assign(new Error('request failed'), {
        config,
        response: {
          status,
          statusText: 'Forbidden',
          data: { success: false, code, message: 'account context mismatch' },
          headers: {},
          config,
        },
      })
    }
  }

  test('drops the selected organization when the server rejects the context', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    rejectWithCode('organization_context_mismatch', 403)

    await expect(api.get('/api/organizations/7/members')).rejects.toBeTruthy()

    expect(useAccountContextStore.getState().accountContext.current).toBeNull()
    expect(toastError).toHaveBeenCalledWith(
      'Your account context changed. Reload the page and try again.'
    )
  })

  test('keeps the selected organization for unrelated business errors', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(7))
    rejectWithCode('organization_invite_already_sent', 409)

    await expect(api.post('/api/organizations/7/invitations')).rejects.toBeTruthy()

    expect(useAccountContextStore.getState().accountContext.current).toEqual(
      organizationContext(7)
    )
    expect(toastError).toHaveBeenCalledWith(
      'An invitation for this email address is already pending.'
    )
  })
})
