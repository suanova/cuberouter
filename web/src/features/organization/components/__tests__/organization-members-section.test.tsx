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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { listOrganizationMembers } from '../../api'
import type { OrganizationMemberRow } from '../../types'
import { OrganizationMembersSection } from '../organization-members-section'

// The URL is the page's state, not this section's; the harness supplies a fixed
// empty set of search params instead of standing up a route.
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

// The settings dialog's model options come from the pricing list, which has
// nothing to say about the entry under test.
vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({ models: [] }),
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  listOrganizationMembers: vi.fn(),
}))

const listMembersMock = vi.mocked(listOrganizationMembers)

const ORGANIZATION_ID = 7
const MEMBER_USER_ID = 9
const ACTOR_USER_ID = 1

function memberRow(): OrganizationMemberRow {
  return {
    id: 3,
    organization_id: ORGANIZATION_ID,
    user_id: MEMBER_USER_ID,
    role: 'member',
    status: 'active',
    username: 'bob',
    display_name: 'Bob Member',
    email: 'bob@example.com',
  }
}

function renderSection(props: {
  canManageMemberModels: boolean
  canManageMemberDiscounts: boolean
  canManageMemberQuotas: boolean
}): void {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <OrganizationMembersSection
        organizationId={ORGANIZATION_ID}
        actorRole='owner'
        currentUserId={ACTOR_USER_ID}
        canView
        canManageMembers
        canAddMembersDirectly={false}
        canExitOrganization={false}
        canManageMemberDiscounts={props.canManageMemberDiscounts}
        canManageMemberModels={props.canManageMemberModels}
        canManageMemberQuotas={props.canManageMemberQuotas}
        isPlatformAdministrator={false}
        readOnly={false}
        onForbidden={() => undefined}
        onLeftOrganization={() => undefined}
      />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  listMembersMock.mockReset().mockResolvedValue({
    success: true,
    data: { page: 1, page_size: 10, total: 1, items: [memberRow()] },
  })
})

describe('organization members section', () => {
  test('offers the settings entry when only the quota grant is held', async () => {
    renderSection({
      canManageMemberModels: false,
      canManageMemberDiscounts: false,
      canManageMemberQuotas: true,
    })

    const menus = await screen.findAllByRole('button', { name: 'Open menu' })
    await userEvent.click(menus[0])

    expect(
      await screen.findByRole('menuitem', { name: 'Model Settings' })
    ).toBeInTheDocument()
  })

  test('leaves the settings entry out when no member-management grant is held', async () => {
    renderSection({
      canManageMemberModels: false,
      canManageMemberDiscounts: false,
      canManageMemberQuotas: false,
    })

    const menus = await screen.findAllByRole('button', { name: 'Open menu' })
    await userEvent.click(menus[0])

    // The menu itself is still there — an owner may disable or remove this
    // member — so what is asserted is the missing item, not a missing menu.
    expect(
      screen.queryByRole('menuitem', { name: 'Model Settings' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Remove' })).toBeInTheDocument()
  })
})
