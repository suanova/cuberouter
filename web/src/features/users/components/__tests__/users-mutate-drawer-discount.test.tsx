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
import { useState, type ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  getGroups,
  getPermissionCatalog,
  getUser,
  getUserModelDiscounts,
  updateUser,
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
  updateUser: vi.fn(),
  updateUserModelDiscounts: vi.fn(),
}))

const getUserMock = vi.mocked(getUser)
const getGroupsMock = vi.mocked(getGroups)
const getPermissionCatalogMock = vi.mocked(getPermissionCatalog)
const getUserModelDiscountsMock = vi.mocked(getUserModelDiscounts)
const updateUserMock = vi.mocked(updateUser)
const updateUserModelDiscountsMock = vi.mocked(updateUserModelDiscounts)

const EDITED_USER_ID = 7
const SECOND_USER_ID = 8
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

function renderWithProviders(ui: ReactNode): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <UsersProvider>{ui}</UsersProvider>
    </QueryClientProvider>
  )
}

function renderDrawer(currentRow?: User): void {
  renderWithProviders(
    <UsersMutateDrawer
      open
      onOpenChange={() => undefined}
      currentRow={currentRow}
    />
  )
}

// The users page keeps the drawer mounted and swaps `currentRow` when another
// row is opened (dropping it while the drawer is closed), so the harness mirrors
// that lifecycle for the cases where state must not survive a close.
function DrawerHarness(props: { onOpenChange?: (open: boolean) => void }) {
  const [open, setOpen] = useState(true)
  const [currentRow, setCurrentRow] = useState<User | undefined>(buildUser())

  return (
    <>
      <button type='button' onClick={() => setOpen(false)}>
        Close from page
      </button>
      <button
        type='button'
        onClick={() => {
          setCurrentRow(
            buildUser({ id: SECOND_USER_ID, username: 'second-user' })
          )
          setOpen(true)
        }}
      >
        Edit second user
      </button>
      <UsersMutateDrawer
        open={open}
        onOpenChange={(next) => {
          props.onOpenChange?.(next)
          setOpen(next)
        }}
        currentRow={open ? currentRow : undefined}
      />
    </>
  )
}

function renderHarness(onOpenChange?: (open: boolean) => void): void {
  renderWithProviders(<DrawerHarness onOpenChange={onOpenChange} />)
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
  updateUserMock.mockReset().mockResolvedValue({ success: true, data: {} })
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

  test('treats a read refused as HTTP 200 as a failure, not an empty record', async () => {
    const realUsersApi =
      await vi.importActual<typeof import('../../api')>('../../api')
    getUserModelDiscountsMock.mockImplementation(
      realUsersApi.getUserModelDiscounts
    )
    // The server reports a failed read (unknown user, database error) as HTTP
    // 200 with `success: false`; the api module is what turns that envelope into
    // a rejection the query can surface. An empty record here would let the
    // drawer save a full replacement that clears what was never read.
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: false, message: 'database error' },
    } as never)

    renderDrawer(buildUser())

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Save discounts' })
    ).not.toBeInTheDocument()
  })

  test('writes pending discount edits before the footer saves the form', async () => {
    const onOpenChange = vi.fn()
    renderHarness(onOpenChange)

    fireEvent.input(await screen.findByDisplayValue('80'), {
      target: { value: '50' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() => expect(updateUserMock).toHaveBeenCalled())
    expect(updateUserModelDiscountsMock).toHaveBeenCalledWith(EDITED_USER_ID, {
      m: 0.5,
    })
    // Discounts first: the form save is what closes the drawer.
    expect(
      updateUserModelDiscountsMock.mock.invocationCallOrder[0]
    ).toBeLessThan(updateUserMock.mock.invocationCallOrder[0])
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('keeps a failed discount write from saving the form', async () => {
    const onOpenChange = vi.fn()
    updateUserModelDiscountsMock.mockRejectedValue(new Error('rejected'))
    renderHarness(onOpenChange)

    fireEvent.input(await screen.findByDisplayValue('80'), {
      target: { value: '50' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(updateUserModelDiscountsMock).toHaveBeenCalledWith(
        EDITED_USER_ID,
        {
          m: 0.5,
        }
      )
    )
    expect(updateUserMock).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    // The edit survives and the footer is usable again.
    expect(screen.getByDisplayValue('50')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save changes' })).toBeEnabled()
  })

  test('keeps a discount write refused as HTTP 200 from reporting success', async () => {
    const realUsersApi =
      await vi.importActual<typeof import('../../api')>('../../api')
    updateUserModelDiscountsMock.mockImplementation(
      realUsersApi.updateUserModelDiscounts
    )
    // A business refusal is answered as HTTP 200 with `success: false`; the api
    // module is what turns that envelope into a rejection, so the footer stays
    // with the edit instead of reporting the write as stored.
    vi.spyOn(api, 'put').mockResolvedValue({
      data: { success: false, message: 'Discount factor out of range' },
    } as never)
    const onOpenChange = vi.fn()
    renderHarness(onOpenChange)

    fireEvent.input(await screen.findByDisplayValue('80'), {
      target: { value: '50' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Save changes' }))

    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith(
        `/api/user/${EDITED_USER_ID}/model-discounts`,
        { discounts: { m: 0.5 } }
      )
    )
    expect(updateUserMock).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(toastSuccess).not.toHaveBeenCalled()
    // The rejected edit stays on screen for a retry.
    expect(screen.getByDisplayValue('50')).toBeInTheDocument()
  })

  test('shows nothing of the previous user after the drawer closes and reopens', async () => {
    getUserModelDiscountsMock.mockImplementation(
      async (userId): Promise<UserModelDiscounts> =>
        userId === SECOND_USER_ID ? { m2: 0.4 } : STORED_DISCOUNTS
    )
    renderHarness()

    fireEvent.input(await screen.findByDisplayValue('80'), {
      target: { value: '50' },
    })
    expect(screen.getByDisplayValue('50')).toBeInTheDocument()

    // The open drawer hides the page behind it from the accessibility tree, so
    // the harness's own controls are queried as hidden ones.
    fireEvent.click(
      screen.getByRole('button', { name: 'Close from page', hidden: true })
    )
    fireEvent.click(
      screen.getByRole('button', { name: 'Edit second user', hidden: true })
    )

    // The second user's own record is what the reopened editor shows.
    expect(await screen.findByDisplayValue('40')).toBeInTheDocument()
    expect(screen.queryByDisplayValue('50')).not.toBeInTheDocument()
    expect(updateUserModelDiscountsMock).not.toHaveBeenCalled()
  })
})
