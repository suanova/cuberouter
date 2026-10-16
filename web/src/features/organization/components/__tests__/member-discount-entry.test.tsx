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
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  getOrganizationMemberModelDiscounts,
  listOrganizationMembers,
  updateOrganizationMemberModelDiscounts,
} from '../../api'
import type { OrganizationMemberRow } from '../../types'
import { OrganizationMembersSection } from '../organization-members-section'

/**
 * The members table's way into a member's model discounts.
 *
 * The entry is offered by the row's own menu, so it is only reachable for a row
 * the caller may act on — the same gate the disable and remove items sit behind.
 * What this file pins is the gate itself (the backend capability the section is
 * told about) and the round trip the dialog performs through the organization
 * surface, since the discounts of a member are stored on the user and reached
 * from two different consoles.
 */
const { toastSuccess } = vi.hoisted(() => ({ toastSuccess: vi.fn() }))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: toastSuccess },
}))

// The URL is the pages' state, not this section's; the harness supplies a fixed
// empty set of search params instead of standing up a route. Both values are
// one stable object — a fresh `search` per render would re-enter the pagination
// effect on every pass, which is a loop the real route never causes because it
// resolves the search params once per navigation.
const pageRoute = vi.hoisted(() => ({ search: {}, navigate: vi.fn() }))

vi.mock(
  '@/features/organization/components/organization-page-provider',
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import('@/features/organization/components/organization-page-provider')
    >()),
    useOrganizationSectionRoute: () => pageRoute,
  })
)

// The editor's model options come from the pricing list, which has nothing to
// say about this section.
vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({ models: [{ model_name: 'm' }] }),
}))

// Only the section's endpoints are stubbed; the request shape belongs to the api
// module, which runs for real.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  listOrganizationMembers: vi.fn(),
  getOrganizationMemberModelDiscounts: vi.fn(),
  updateOrganizationMemberModelDiscounts: vi.fn(),
}))

const listMembersMock = vi.mocked(listOrganizationMembers)
const getModelDiscountsMock = vi.mocked(getOrganizationMemberModelDiscounts)
const updateModelDiscountsMock = vi.mocked(
  updateOrganizationMemberModelDiscounts
)

const ORGANIZATION_ID = 7
const MEMBER_USER_ID = 9
const SECOND_MEMBER_USER_ID = 11
const ACTOR_USER_ID = 1
const STORED_DISCOUNTS = { m: 0.8 }
const SECOND_MEMBER_DISCOUNTS = { m: 0.4 }
const SECOND_MEMBER_ID = 4

function memberRow(
  overrides: Partial<OrganizationMemberRow> = {}
): OrganizationMemberRow {
  return {
    id: 3,
    organization_id: ORGANIZATION_ID,
    user_id: MEMBER_USER_ID,
    role: 'member',
    status: 'active',
    username: 'bob',
    display_name: 'Bob Member',
    email: 'bob@example.com',
    ...overrides,
  }
}

function secondMemberRow(): OrganizationMemberRow {
  return memberRow({
    id: SECOND_MEMBER_ID,
    user_id: SECOND_MEMBER_USER_ID,
    username: 'carol',
    display_name: 'Carol Member',
    email: 'carol@example.com',
  })
}

function renderSection(canManageMemberDiscounts: boolean): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={queryClient}>
      <OrganizationMembersSection
        organizationId={ORGANIZATION_ID}
        actorRole='owner'
        currentUserId={ACTOR_USER_ID}
        canView
        canManageMembers
        canAddMembersDirectly={false}
        canExitOrganization={false}
        canManageMemberDiscounts={canManageMemberDiscounts}
        isPlatformAdministrator={false}
        readOnly={false}
        onForbidden={() => undefined}
        onLeftOrganization={() => undefined}
      />
    </QueryClientProvider>
  )
}

/** Opens the menu of one member row the fixture returned. */
async function openRowMenu(rowIndex = 0): Promise<void> {
  const menus = await screen.findAllByRole('button', { name: 'Open menu' })
  await userEvent.click(menus[rowIndex])
}

/** Opens the discount dialog of one member row in the fixture. */
async function openDiscountDialog(rowIndex = 0): Promise<void> {
  await openRowMenu(rowIndex)
  await userEvent.click(
    screen.getByRole('menuitem', { name: 'Model Discounts' })
  )
}

beforeEach(() => {
  listMembersMock.mockReset().mockResolvedValue({
    success: true,
    data: { page: 1, page_size: 10, total: 1, items: [memberRow()] },
  })
  getModelDiscountsMock.mockReset().mockResolvedValue(STORED_DISCOUNTS)
  updateModelDiscountsMock.mockReset().mockResolvedValue(undefined)
  toastSuccess.mockReset()
})

describe('organization member model discount entry', () => {
  test('offers the model discount entry to a caller the backend allows', async () => {
    renderSection(true)

    await openRowMenu()

    expect(
      await screen.findByRole('menuitem', { name: 'Model Discounts' })
    ).toBeInTheDocument()
  })

  test('leaves the entry out for a caller who may not manage discounts', async () => {
    renderSection(false)

    await openRowMenu()

    // The menu itself is still there — an owner may disable or remove this
    // member — so what is asserted is the missing item, not a missing menu.
    expect(
      screen.queryByRole('menuitem', { name: 'Model Discounts' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Remove' })).toBeInTheDocument()
  })

  test('shows the stored discounts of the member the entry was opened on', async () => {
    renderSection(true)

    await openDiscountDialog()

    expect(await screen.findByDisplayValue('80')).toBeInTheDocument()
    expect(getModelDiscountsMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      'member'
    )
  })

  test('saves the shown discounts through the organization surface', async () => {
    renderSection(true)
    await openDiscountDialog()
    await screen.findByDisplayValue('80')

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelDiscountsMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        STORED_DISCOUNTS,
        'member'
      )
    )
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
    await waitFor(() =>
      expect(
        screen.queryByText('Member Model Discounts')
      ).not.toBeInTheDocument()
    )
  })

  test('opens the next member on their own discounts, not the previous edit', async () => {
    listMembersMock.mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 2,
        items: [memberRow(), secondMemberRow()],
      },
    })
    getModelDiscountsMock.mockImplementation(
      async (_organizationId: number, userId: number) =>
        userId === SECOND_MEMBER_USER_ID
          ? SECOND_MEMBER_DISCOUNTS
          : STORED_DISCOUNTS
    )
    renderSection(true)

    // An edit is left behind in the first member's dialog, which is then closed
    // without saving — as an operator who changes their mind does.
    await openDiscountDialog()
    fireEvent.input(await screen.findByDisplayValue('80'), {
      target: { value: '50' },
    })
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await openDiscountDialog(1)

    expect(await screen.findByDisplayValue('40')).toBeInTheDocument()
    expect(screen.queryByDisplayValue('50')).not.toBeInTheDocument()
    expect(updateModelDiscountsMock).not.toHaveBeenCalled()
  })
})
