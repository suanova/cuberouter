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
})
