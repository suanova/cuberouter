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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import type { UserOrganization } from '../../types'
import { useOrganizationsColumns } from '../organizations-columns'

const { navigate, switchAccountContext, toastError } = vi.hoisted(() => ({
  navigate: vi.fn(),
  switchAccountContext: vi.fn(),
  toastError: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { error: toastError, success: vi.fn() },
}))

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  useNavigate: () => navigate,
}))

// Selecting the context is a server round trip; the switch itself is covered by
// the account-context tests, so only the call is observed here.
vi.mock('@/lib/account-context', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/lib/account-context')>()),
  switchAccountContext,
}))

const noOrganizationCapabilities = {
  can_view_organization: false,
  can_view_organization_wide_data: false,
  can_view_organization_usage: false,
  can_view_organization_tokens: false,
  can_view_organization_logs: false,
  can_update_organization: false,
  can_disable_organization: false,
  can_enable_organization: false,
  can_manage_members: false,
  can_transfer_owner: false,
  can_add_members_directly: false,
  can_exit_organization: false,
  can_view_invites: false,
  can_create_invites: false,
  can_revoke_invites: false,
  can_manage_all_tokens: false,
  can_modify_organization_group: false,
  can_dissolve_organization: false,
  can_view_audit: false,
  can_view_organization_billing_summary: false,
  show_return_organization_center: false,
  show_return_personal_center: false,
}

function userOrganization(
  overrides: Partial<Omit<UserOrganization, 'capabilities'>> & {
    capabilities?: Partial<UserOrganization['capabilities']>
  } = {}
): UserOrganization {
  const { capabilities, ...rest } = overrides
  return {
    id: 7,
    name: 'Acme Research',
    slug: 'acme-research',
    description: 'Shared inference budget',
    status: 'active',
    quota: 1000,
    used_quota: 100,
    request_count: 3,
    owner_user_id: 1,
    created_by: 1,
    created_at: 0,
    updated_at: 0,
    role: 'owner',
    disable_state: {
      active_sources: [],
      effective_source: '',
      can_self_enable: true,
    },
    can_self_enable: true,
    access_mode: 'workspace',
    ...rest,
    capabilities: {
      ...noOrganizationCapabilities,
      can_view_organization: true,
      ...capabilities,
    },
  }
}

/**
 * Renders just the name cell through the real table API, so the assertion covers
 * the column definition the list mounts rather than a copy of its rendering.
 */
function OrganizationNameCell(props: { organization: UserOrganization }) {
  const columns = useOrganizationsColumns()
  const table = useReactTable({
    data: [props.organization],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getVisibleCells()
    .find((candidate) => candidate.column.id === 'name')

  if (!cell) return null

  return <>{flexRender(cell.column.columnDef.cell, cell.getContext())}</>
}

function renderNameCell(organization: UserOrganization) {
  return render(
    <QueryClientProvider client={new QueryClient()}>
      <OrganizationNameCell organization={organization} />
    </QueryClientProvider>
  )
}

describe('organization list name cell', () => {
  beforeEach(() => {
    navigate.mockReset()
    switchAccountContext.mockReset()
    toastError.mockReset()
    switchAccountContext.mockResolvedValue(undefined)
    navigate.mockResolvedValue(undefined)
  })

  test('opens the organization as a context before navigating to it', async () => {
    renderNameCell(userOrganization())

    await userEvent.click(screen.getByRole('button', { name: 'Acme Research' }))

    expect(switchAccountContext).toHaveBeenCalledWith('organization', 7)
    expect(navigate).toHaveBeenCalledWith({
      to: '/organizations/$organizationId/$section',
      params: { organizationId: '7', section: 'overview' },
    })
    // The page's own reads compare the request's context against the id in the
    // path, so the switch has to be settled before the destination renders.
    expect(switchAccountContext.mock.invocationCallOrder[0]).toBeLessThan(
      navigate.mock.invocationCallOrder[0]
    )
  })

  test('opens a non-active organization without touching the account context', async () => {
    renderNameCell(userOrganization({ status: 'disabled' }))

    await userEvent.click(screen.getByRole('button', { name: 'Acme Research' }))

    expect(switchAccountContext).not.toHaveBeenCalled()
    expect(navigate).toHaveBeenCalledTimes(1)
  })

  test('leaves the name unclickable when the caller may not view the organization', () => {
    renderNameCell(
      userOrganization({ capabilities: { can_view_organization: false } })
    )

    expect(screen.getByText('Acme Research')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Acme Research' })
    ).not.toBeInTheDocument()
  })

  test('reports a refused context switch and stays on the list', async () => {
    switchAccountContext.mockRejectedValue(new Error('organization not found'))
    renderNameCell(userOrganization())

    await userEvent.click(screen.getByRole('button', { name: 'Acme Research' }))

    expect(toastError).toHaveBeenCalledWith('organization not found')
    expect(navigate).not.toHaveBeenCalled()
  })
})
