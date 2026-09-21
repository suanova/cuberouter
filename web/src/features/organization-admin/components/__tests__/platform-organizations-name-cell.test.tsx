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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, test, vi } from 'vitest'

import type { OrganizationManagementView } from '@/features/organization/types'

import { usePlatformOrganizationsColumns } from '../platform-organizations-columns'

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  // The router's own link resolution is not what this test is about; the anchor
  // below turns the route and params the column passes into the URL a browser
  // would end up on.
  Link: (props: {
    to: string
    params: Record<string, string>
    className?: string
    children?: ReactNode
  }) => {
    const href = Object.entries(props.params).reduce(
      (path, [name, value]) => path.replace(`$${name}`, value),
      props.to
    )
    return (
      <a href={href} className={props.className}>
        {props.children}
      </a>
    )
  },
}))

function platformOrganization(
  overrides: Partial<OrganizationManagementView> = {}
): OrganizationManagementView {
  return {
    id: 7,
    name: 'Acme Research',
    slug: 'acme-research',
    description: 'Shared inference budget',
    group: 'default',
    status: 'active',
    quota: 1000,
    used_quota: 100,
    request_count: 3,
    owner_user_id: 1,
    created_by: 1,
    created_at: 0,
    updated_at: 0,
    dissolved_at: 0,
    owner_username: 'alice',
    owner_display_name: 'Alice',
    owner_email: 'alice@example.com',
    active_member_count: 2,
    disabled_member_count: 0,
    total_member_count: 2,
    enabled_token_count: 1,
    disabled_token_count: 0,
    total_token_count: 1,
    ...overrides,
  }
}

/**
 * Renders just the name cell through the real table API, so the assertion covers
 * the column definition the platform list mounts rather than a copy of its
 * rendering.
 */
function OrganizationNameCell(props: {
  organization: OrganizationManagementView
}) {
  const columns = usePlatformOrganizationsColumns({ isRoot: true })
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

describe('platform organization list name cell', () => {
  test('links the name to the organization it opens', () => {
    render(<OrganizationNameCell organization={platformOrganization()} />)

    expect(screen.getByRole('link', { name: 'Acme Research' })).toHaveAttribute(
      'href',
      '/admin/organizations/7/overview'
    )
  })

  test('links a dissolved organization too, which has nothing left but its record', () => {
    render(
      <OrganizationNameCell
        organization={platformOrganization({ status: 'dissolved' })}
      />
    )

    expect(screen.getByRole('link', { name: 'Acme Research' })).toHaveAttribute(
      'href',
      '/admin/organizations/7/overview'
    )
  })
})
