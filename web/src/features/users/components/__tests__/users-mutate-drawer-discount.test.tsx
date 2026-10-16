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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  getGroups,
  getPermissionCatalog,
  getUser,
  getUserModelDiscounts,
  updateUserModelDiscounts,
} from '../../api'
import type { User, UserModelDiscounts } from '../../types'
import { UsersMutateDrawer } from '../users-mutate-drawer'
import { UsersProvider } from '../users-provider'

const { toastSuccess } = vi.hoisted(() => ({ toastSuccess: vi.fn() }))

// A failed write is reported by the axios interceptor's own toast, so the
// success toast is the only one this drawer raises itself.
vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: toastSuccess },
}))

// The editor's model options come from the pricing list; the pricing page
// itself has nothing to say here.
vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({
    models: [{ model_name: 'm' }, { model_name: 'm2' }],
  }),
}))

// Only the drawer's endpoints are stubbed. The request shape belongs to the api
// module, which runs for real.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  getGroups: vi.fn(),
  getPermissionCatalog: vi.fn(),
  getUser: vi.fn(),
  getUserModelDiscounts: vi.fn(),
  updateUserModelDiscounts: vi.fn(),
}))

const getUserMock = vi.mocked(getUser)
const getGroupsMock = vi.mocked(getGroups)
const getPermissionCatalogMock = vi.mocked(getPermissionCatalog)
const getUserModelDiscountsMock = vi.mocked(getUserModelDiscounts)
const updateUserModelDiscountsMock = vi.mocked(updateUserModelDiscounts)

const EDITED_USER_ID = 7
const STORED_DISCOUNTS: UserModelDiscounts = { m: 0.8 }

function buildUser(overrides: Partial<User> = {}): User {
  return {
    id: EDITED_USER_ID,
    username: 'discount-user',
    display_name: 'Discount User',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    ...overrides,
  }
}

function renderDrawer(currentRow?: User): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>
        <UsersMutateDrawer
          open
          onOpenChange={() => undefined}
          currentRow={currentRow}
        />
      </UsersProvider>
    </QueryClientProvider>
  )
}

// A request that never settles pins the drawer in its in-flight state.
function neverSettles<T>(): Promise<T> {
  return new Promise<T>(() => undefined)
}

beforeEach(() => {
  getUserMock.mockReset().mockResolvedValue({
    success: true,
    data: buildUser(),
  })
  getGroupsMock
    .mockReset()
    .mockResolvedValue({ success: true, data: ['default'] })
  getPermissionCatalogMock
    .mockReset()
    .mockResolvedValue({ resources: [], roles: [] })
  getUserModelDiscountsMock.mockReset().mockResolvedValue(STORED_DISCOUNTS)
  updateUserModelDiscountsMock.mockReset().mockResolvedValue(undefined)
  toastSuccess.mockReset()
})

describe('users mutate drawer model discounts', () => {
  test('shows the stored discounts of the edited user below Group & Quota', async () => {
    renderDrawer(buildUser())

    expect(await screen.findByDisplayValue('80')).toBeInTheDocument()

    const groupHeading = screen.getByText('Group & Quota')
    const discountsHeading = screen.getByText('Model Discounts')
    expect(
      groupHeading.compareDocumentPosition(discountsHeading) &
        Node.DOCUMENT_POSITION_FOLLOWING
    ).toBeTruthy()
  })

  test('saves the discounts that were loaded', async () => {
    renderDrawer(buildUser())

    fireEvent.click(
      await screen.findByRole('button', { name: 'Save discounts' })
    )

    await waitFor(() =>
      expect(updateUserModelDiscountsMock).toHaveBeenCalledWith(
        EDITED_USER_ID,
        {
          m: 0.8,
        }
      )
    )
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
  })

  test('saves the percent edited in the drawer', async () => {
    renderDrawer(buildUser())

    fireEvent.input(await screen.findByDisplayValue('80'), {
      target: { value: '50' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save discounts' }))

    await waitFor(() =>
      expect(updateUserModelDiscountsMock).toHaveBeenCalledWith(
        EDITED_USER_ID,
        {
          m: 0.5,
        }
      )
    )
  })

  test('disables the editor and the save button while the save is in flight', async () => {
    updateUserModelDiscountsMock.mockReturnValue(neverSettles<void>())
    renderDrawer(buildUser())

    const saveButton = await screen.findByRole('button', {
      name: 'Save discounts',
    })
    fireEvent.click(saveButton)

    await waitFor(() => expect(saveButton).toBeDisabled())
    expect(screen.getByDisplayValue('80')).toBeDisabled()
  })

  test('hides the discounts section when creating a user', async () => {
    renderDrawer()

    expect(await screen.findByText('Basic Information')).toBeInTheDocument()
    expect(screen.queryByText('Model Discounts')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Save discounts' })
    ).not.toBeInTheDocument()
    expect(getUserModelDiscountsMock).not.toHaveBeenCalled()
  })

  test('shows a placeholder while the discounts load', async () => {
    getUserModelDiscountsMock.mockReturnValue(
      neverSettles<UserModelDiscounts>()
    )
    renderDrawer(buildUser())

    expect(await screen.findByText('Loading')).toBeInTheDocument()
    expect(screen.queryByText('Model Discounts')).not.toBeInTheDocument()
  })

  test('replaces the editor with a failure notice when the discounts cannot be read', async () => {
    getUserModelDiscountsMock.mockRejectedValue(new Error('unavailable'))
    renderDrawer(buildUser())

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(screen.queryByText('Model Discounts')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Save discounts' })
    ).not.toBeInTheDocument()
  })
})
