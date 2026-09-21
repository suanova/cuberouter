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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { PlatformOrganizationDetail } from '../platform-organization-detail'

/**
 * The header's three actions are the only triggers for the page's three dialogs.
 *
 * This mounts the real page rather than a hand-built harness of its parts, and
 * that is the whole point: the defect being covered was that the dialogs were
 * mounted *inside* `SectionPageLayout`, which renders only the four slots it
 * knows by name (Title / Actions / Content / Breadcrumb) and silently drops
 * every other child. The state flipped on click and nothing ever appeared, which
 * reads as a dead button. Only the page's own tree can catch that — a harness
 * that assembles the button and the dialog itself would pass either way.
 */
const fixtures = vi.hoisted(() => ({
  refetch: vi.fn(),
  detail: {
    organization: {
      id: 7,
      name: 'Acme Research',
      slug: 'acme-research',
      status: 'active',
      group: 'default',
      owner_user_id: 42,
      created_at: 0,
    },
    actor: {
      user_id: 42,
      role: '',
      organization_role: 'owner',
      access_mode: 'admin',
      read_only: false,
      is_platform_admin: true,
      is_organization_member: false,
      capabilities: {
        can_view_organization: true,
        can_view_organization_wide_data: true,
        can_view_organization_usage: true,
        can_view_organization_tokens: true,
        can_view_organization_logs: true,
        can_update_organization: true,
        can_disable_organization: true,
        can_enable_organization: false,
        can_manage_members: true,
        can_transfer_owner: false,
        can_add_members_directly: true,
        can_exit_organization: false,
        can_view_invites: true,
        can_create_invites: true,
        can_revoke_invites: true,
        can_manage_all_tokens: true,
        can_modify_organization_group: true,
        can_dissolve_organization: true,
        can_view_audit: true,
        can_view_organization_billing_summary: true,
        show_return_organization_center: false,
        show_return_personal_center: false,
      },
    },
  },
}))

vi.mock('@tanstack/react-router', () => ({
  getRouteApi: () => ({
    useParams: () => ({ organizationId: '7', section: 'overview' }),
    useNavigate: () => vi.fn(),
    useSearch: () => ({}),
  }),
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}))

vi.mock('@/features/organization/hooks/use-organization-detail', () => ({
  useOrganizationDetail: () => ({
    detail: fixtures.detail,
    refetch: fixtures.refetch,
  }),
}))

// The sections are the body of the page and need queries of their own; they say
// nothing about whether the header's actions reach their dialogs.
vi.mock('@/features/organization/components/organization-sections', () => ({
  OrganizationSections: () => <div data-testid='organization-sections' />,
}))

vi.mock(
  '@/features/organization/components/organization-page-provider',
  () => ({
    OrganizationPageProvider: (props: { children: React.ReactNode }) =>
      props.children,
  })
)

vi.mock('../platform-organization-owner-repair', () => ({
  PlatformOrganizationOwnerRepair: () => null,
}))

// Each dialog stands in for its real counterpart by being observable exactly
// when it is open — which is the property under test. What happens inside them
// once open is their own tests' business.
vi.mock('../platform-organization-edit-drawer', () => ({
  PlatformOrganizationEditDrawer: (props: { open: boolean }) =>
    props.open ? <div data-testid='edit-drawer' /> : null,
}))

vi.mock('../platform-organization-status-dialog', () => ({
  PlatformOrganizationStatusDialog: (props: { open: boolean }) =>
    props.open ? <div data-testid='status-dialog' /> : null,
}))

vi.mock('../platform-organization-dissolve-dialog', () => ({
  PlatformOrganizationDissolveDialog: (props: { open: boolean }) =>
    props.open ? <div data-testid='dissolve-dialog' /> : null,
}))

describe('platform organization detail header actions', () => {
  test('opens the edit drawer from the Edit button', async () => {
    render(<PlatformOrganizationDetail />)

    expect(screen.queryByTestId('edit-drawer')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Edit' }))

    expect(screen.getByTestId('edit-drawer')).toBeInTheDocument()
  })

  test('opens the status dialog from the Disable button', async () => {
    render(<PlatformOrganizationDetail />)

    expect(screen.queryByTestId('status-dialog')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: 'Disable' }))

    expect(screen.getByTestId('status-dialog')).toBeInTheDocument()
  })

  test('opens the dissolve dialog from the Dissolve Organization button', async () => {
    render(<PlatformOrganizationDetail />)

    expect(screen.queryByTestId('dissolve-dialog')).not.toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Dissolve Organization' })
    )

    expect(screen.getByTestId('dissolve-dialog')).toBeInTheDocument()
  })
})
