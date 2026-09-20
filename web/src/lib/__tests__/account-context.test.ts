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

import {
  isOrganizationContext,
  refreshAccountContexts,
  switchAccountContext,
} from '../account-context'
import { useAccountContextStore } from '@/stores/account-context-store'

const { get, put } = vi.hoisted(() => ({ get: vi.fn(), put: vi.fn() }))

vi.mock('@/lib/http-client', () => ({
  api: { get, put },
}))

function organizationContext(id = 7) {
  return { type: 'organization' as const, id, name: `Org ${id}`, role: 'member' }
}

function contextsResponse(current: unknown, contexts: unknown[]) {
  return { data: { success: true, data: { current, contexts } } }
}

describe('refreshAccountContexts', () => {
  beforeEach(() => {
    get.mockReset()
    useAccountContextStore.getState().accountContext.reset()
  })

  test('adopts the context the server reports as current', async () => {
    get.mockResolvedValue(
      contextsResponse(organizationContext(7), [organizationContext(7)])
    )

    const payload = await refreshAccountContexts()

    expect(get).toHaveBeenCalledWith('/api/account-contexts', {
      skipErrorHandler: true,
      skipBusinessError: true,
      skipAccountContext: true,
    })
    expect(payload.current).toEqual(organizationContext(7))
    expect(isOrganizationContext()).toBe(true)
  })

  test('overrides a locally selected organization with the server current', async () => {
    // 本地可能还存着一个已经被解散 / 已退出的组织，服务端才是权威值。
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(99))
    get.mockResolvedValue(contextsResponse(null, []))

    await refreshAccountContexts()

    expect(useAccountContextStore.getState().accountContext.current).toBeNull()
    expect(isOrganizationContext()).toBe(false)
  })

  test('marks the store as errored and rethrows when the request fails', async () => {
    const failure = new Error('network down')
    get.mockRejectedValue(failure)

    await expect(refreshAccountContexts()).rejects.toBe(failure)

    expect(useAccountContextStore.getState().accountContext.status).toBe('error')
  })

  test('tolerates a response without a data envelope', async () => {
    get.mockResolvedValue({ data: { success: false, message: 'nope' } })

    const payload = await refreshAccountContexts()

    expect(payload).toEqual({ current: null, contexts: [] })
  })
})

describe('switchAccountContext', () => {
  beforeEach(() => {
    put.mockReset()
    useAccountContextStore.getState().accountContext.reset()
  })

  test('persists the choice and stores the confirmed context', async () => {
    put.mockResolvedValue({
      data: { success: true, data: organizationContext(5) },
    })

    const context = await switchAccountContext('organization', 5)

    expect(put).toHaveBeenCalledWith(
      '/api/account-contexts/current',
      { type: 'organization', id: 5 },
      { skipAccountContext: true }
    )
    expect(context).toEqual(organizationContext(5))
    expect(useAccountContextStore.getState().accountContext.current).toEqual(
      organizationContext(5)
    )
  })

  test('switching back to personal clears the selected organization', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(5))
    put.mockResolvedValue({
      data: {
        success: true,
        data: { type: 'personal', id: 12, name: 'me' },
      },
    })

    await switchAccountContext('personal', 12)

    expect(isOrganizationContext()).toBe(false)
  })

  test('keeps the previous context when the server confirms nothing', async () => {
    useAccountContextStore
      .getState()
      .accountContext.setCurrent(organizationContext(5))
    put.mockResolvedValue({ data: { success: false, message: 'denied' } })

    await expect(switchAccountContext('organization', 6)).rejects.toThrow(
      'denied'
    )

    expect(useAccountContextStore.getState().accountContext.current).toEqual(
      organizationContext(5)
    )
  })
})
