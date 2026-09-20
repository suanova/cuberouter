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
import type { QueryClient } from '@tanstack/react-query'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { AccountContext } from '@/lib/account-context'
import { useAccountContextStore } from '@/stores/account-context-store'

import { getOrganization } from '../../api'
import { alignOrganizationAccountContext } from '../align-account-context'

vi.mock('../../api', () => ({
  getOrganization: vi.fn(),
}))

const switchAccountContext = vi.fn()
vi.mock('@/lib/account-context', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/account-context')>()),
  switchAccountContext: (...args: unknown[]) => switchAccountContext(...args),
}))

const getOrganizationMock = vi.mocked(getOrganization)

/** The error shape the interceptor and `getServerErrorCode` both understand. */
function serverError(code: string) {
  return { response: { data: { success: false, message: code, code } } }
}

function makeQueryClient(): QueryClient {
  return { invalidateQueries: vi.fn().mockResolvedValue(undefined) } as unknown as QueryClient
}

function setCurrentContext(context: AccountContext | null) {
  useAccountContextStore.getState().accountContext.setCurrent(context)
}

/** Only `type` and `id` decide whether alignment is needed. */
function organizationContext(id: number): AccountContext {
  return { type: 'organization', id, name: `org-${id}` }
}

beforeEach(() => {
  vi.clearAllMocks()
  setCurrentContext(null)
})

describe('alignOrganizationAccountContext', () => {
  test('ignores an unusable id without probing', async () => {
    for (const id of [0, -1, Number.NaN, Number.POSITIVE_INFINITY]) {
      await expect(
        alignOrganizationAccountContext(id, makeQueryClient())
      ).resolves.toBe(false)
    }
    expect(getOrganizationMock).not.toHaveBeenCalled()
    expect(switchAccountContext).not.toHaveBeenCalled()
  })

  test('does nothing when the context already names the organization', async () => {
    setCurrentContext(organizationContext(42))

    await expect(
      alignOrganizationAccountContext(42, makeQueryClient())
    ).resolves.toBe(false)
    expect(getOrganizationMock).not.toHaveBeenCalled()
    expect(switchAccountContext).not.toHaveBeenCalled()
  })

  test('keeps a readable organization rather than switching to it', async () => {
    // A disabled or dissolved organization answers reads from any context, so
    // the probe succeeding is exactly the signal not to switch.
    setCurrentContext(organizationContext(7))
    getOrganizationMock.mockResolvedValue({} as never)

    await expect(
      alignOrganizationAccountContext(42, makeQueryClient())
    ).resolves.toBe(false)
    expect(getOrganizationMock).toHaveBeenCalledWith(42, { silent: true })
    expect(switchAccountContext).not.toHaveBeenCalled()
  })

  test('switches when the probe reports a context mismatch', async () => {
    setCurrentContext(organizationContext(7))
    getOrganizationMock.mockRejectedValue(
      serverError('organization_context_mismatch')
    )
    switchAccountContext.mockResolvedValue({ type: 'organization', id: 42 })
    const queryClient = makeQueryClient()

    await expect(
      alignOrganizationAccountContext(42, queryClient)
    ).resolves.toBe(true)
    expect(switchAccountContext).toHaveBeenCalledWith('organization', 42)
    expect(queryClient.invalidateQueries).toHaveBeenCalled()
  })

  test('switches from the personal context too', async () => {
    // The common deep link: the browser last held the personal context.
    setCurrentContext(null)
    getOrganizationMock.mockRejectedValue(
      serverError('organization_context_mismatch')
    )
    switchAccountContext.mockResolvedValue({ type: 'organization', id: 42 })

    await expect(
      alignOrganizationAccountContext(42, makeQueryClient())
    ).resolves.toBe(true)
    expect(switchAccountContext).toHaveBeenCalledWith('organization', 42)
  })

  test('reports any other failure to the caller instead of switching', async () => {
    setCurrentContext(null)
    getOrganizationMock.mockRejectedValue(serverError('organization_dissolved'))

    await expect(
      alignOrganizationAccountContext(42, makeQueryClient())
    ).resolves.toBe(false)
    expect(switchAccountContext).not.toHaveBeenCalled()
  })

  test('stays quiet when the switch itself is refused', async () => {
    setCurrentContext(null)
    getOrganizationMock.mockRejectedValue(
      serverError('organization_context_mismatch')
    )
    switchAccountContext.mockRejectedValue(serverError('organization_dissolved'))
    const queryClient = makeQueryClient()

    // The page renders its own "not available" state; the navigation must not
    // fail with it.
    await expect(
      alignOrganizationAccountContext(42, queryClient)
    ).resolves.toBe(false)
    expect(queryClient.invalidateQueries).not.toHaveBeenCalled()
  })
})
