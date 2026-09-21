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

import { OrganizationDetail } from '../organization-detail'

/**
 * The header actions of the organization workspace.
 *
 * Mounts the real page, for the same reason the platform page's equivalent test
 * does: the dissolve confirmation used to be mounted *inside*
 * `SectionPageLayout`, which renders only its four named slots and drops every
 * other child. It never mounted, so the button looked broken. A harness
 * assembling the button and the confirm itself would pass whether or not that
 * defect is present.
 *
 * The leave action is covered here for a related reason: it is a member's only
 * way out of an organization, and it used to live solely inside the members
 * section, which a member no longer has.
 */
const fixtures = vi.hoisted(() => ({
  refetch: vi.fn(),
  /** The section the URL is on; tests move it to the tab a caller lands on. */
  section: 'settings',
  /** The owner's real capability set; tests replace it to play other callers. */
  ownerCapabilities: {
    can_view_organization: true,
    can_view_organization_wide_data: true,
    can_view_organization_usage: true,
    can_view_organization_tokens: true,
    can_view_organization_logs: true,
    can_update_organization: true,
    can_disable_organization: true,
    can_enable_organization: false,
    can_manage_members: true,
    can_transfer_owner: true,
    can_add_members_directly: false,
    can_exit_organization: true,
    can_view_invites: true,
    can_create_invites: true,
    can_revoke_invites: true,
    can_manage_all_tokens: true,
    can_modify_organization_group: true,
    can_dissolve_organization: true,
    can_view_audit: true,
    can_view_organization_billing_summary: true,
    show_return_organization_center: false,
    show_return_personal_center: true,
  },
  capabilities: {},
  detail: {
    organization: {
      id: 7,
      name: 'Acme Research',
      slug: 'acme-research',
      status: 'active',
      group: 'default',
      created_at: 0,
    },
    actor: {
      user_id: 42,
      role: 'owner',
      organization_role: 'owner',
      access_mode: 'workspace',
      read_only: false,
      is_platform_admin: false,
      is_organization_member: true,
    },
  },
}))

// The dissolve action is only offered on the settings section, so the page has
// to be entered there for the button to exist at all.
vi.mock('@tanstack/react-router', () => ({
  getRouteApi: () => ({
    useParams: () => ({ organizationId: '7', section: fixtures.section }),
    useNavigate: () => vi.fn(),
    useSearch: () => ({}),
  }),
}))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}))

vi.mock('@/lib/account-context', () => ({
  refreshAccountContexts: vi.fn(),
}))

vi.mock('@/features/organization/hooks/use-organization-detail', () => ({
  useOrganizationDetail: () => ({
    detail: {
      ...fixtures.detail,
      actor: { ...fixtures.detail.actor, capabilities: fixtures.capabilities },
    },
    refetch: fixtures.refetch,
  }),
}))

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

// Observable exactly while it has an organization to confirm, which is how the
// page opens it; the confirm's own behaviour is its own test's business.
vi.mock(
  '@/features/organization/components/organization-dissolve-confirm',
  () => ({
    OrganizationDissolveConfirm: (props: { organization: unknown }) =>
      props.organization ? <div data-testid='dissolve-confirm' /> : null,
  })
)

// Observable exactly while it is open, which is how the header button opens it.
// The dialog's own request belongs to its own test.
vi.mock(
  '@/features/organization/components/organization-member-dialogs',
  () => ({
    OrganizationExitDialog: (props: { open: boolean }) =>
      props.open ? <div data-testid='exit-dialog' /> : null,
  })
)

/** A plain member of an active organization: no roster, no wide data. */
const MEMBER_CAPABILITIES = {
  ...fixtures.ownerCapabilities,
  can_view_organization_wide_data: false,
  can_update_organization: false,
  can_disable_organization: false,
  can_manage_members: false,
  can_transfer_owner: false,
  can_view_invites: false,
  can_create_invites: false,
  can_revoke_invites: false,
  can_manage_all_tokens: false,
  can_modify_organization_group: false,
  can_dissolve_organization: false,
  can_view_audit: false,
  can_view_organization_billing_summary: false,
  show_return_organization_center: false,
}

function renderDetail() {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <OrganizationDetail />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  fixtures.section = 'settings'
  fixtures.capabilities = { ...fixtures.ownerCapabilities }
})

describe('organization detail header actions', () => {
  test('opens the dissolve confirmation from the Dissolve Organization button', async () => {
    renderDetail()

    expect(screen.queryByTestId('dissolve-confirm')).not.toBeInTheDocument()
    await userEvent.click(
      screen.getByRole('button', { name: 'Dissolve Organization' })
    )

    expect(screen.getByTestId('dissolve-confirm')).toBeInTheDocument()
  })

  test('offers a member the way out that the members section used to hold', async () => {
    fixtures.section = 'tokens'
    fixtures.capabilities = MEMBER_CAPABILITIES
    renderDetail()

    // The section that carried the Leave button is gone for this caller. Both
    // halves are asserted together because it is the coupling that matters: the
    // action may not disappear along with the tab that used to host it.
    expect(
      screen.queryByRole('tab', { name: 'Members' })
    ).not.toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('button', { name: 'Leave organization' })
    )

    expect(screen.getByTestId('exit-dialog')).toBeInTheDocument()
  })

  test('keeps the leave action out of the header for a caller with a members section', () => {
    renderDetail()

    // An owner reaches the roster, so the button stays where it has always been
    // rather than appearing in two places at once.
    expect(screen.getByRole('tab', { name: 'Members' })).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Leave organization' })
    ).not.toBeInTheDocument()
  })
})
