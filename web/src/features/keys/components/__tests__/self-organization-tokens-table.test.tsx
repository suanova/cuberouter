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
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getOrganizations } from '@/features/organization/api'
import type {
  OrganizationTokenRow,
  UserOrganization,
} from '@/features/organization/types'

import { getSelfOrganizationTokens } from '../../api'
import { SelfOrganizationTokensTable } from '../self-organization-tokens-table'

const search = vi.hoisted(() => ({ value: {} as Record<string, unknown> }))
const navigate = vi.hoisted(() => vi.fn())
const routeApi = vi.hoisted(() => ({
  useSearch: () => search.value,
  useNavigate: () => navigate,
}))

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  getRouteApi: () => routeApi,
  Link: (props: { children?: React.ReactNode; to?: string }) => (
    <a href={props.to}>{props.children}</a>
  ),
}))

vi.mock('@/features/organization/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/organization/api')>()),
  getOrganizations: vi.fn(),
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  getSelfOrganizationTokens: vi.fn(),
}))

const getTokensMock = vi.mocked(getSelfOrganizationTokens)
const getOrganizationsMock = vi.mocked(getOrganizations)

function token(
  overrides: Partial<OrganizationTokenRow> = {}
): OrganizationTokenRow {
  return {
    id: 2,
    user_id: 101,
    key: 'acme-public',
    key_preview: 'sk-acme********blic',
    status: 1,
    name: 'acme-public',
    created_time: 1_700_000_000,
    accessed_time: 1_700_000_100,
    expired_time: -1,
    remain_quota: 1_000_000,
    unlimited_quota: false,
    model_limits_enabled: false,
    model_limits: '',
    used_quota: 20_000,
    group: 'default',
    cross_group_retry: false,
    visibility: 'public',
    organization_id: 9,
    organization_name: 'Acme',
    responsible_user_id: 101,
    responsible_username: 'alice',
    responsible_display_name: 'Alice',
    ...overrides,
  }
}

beforeEach(() => {
  search.value = {
    orgPage: 1,
    orgPageSize: 20,
    orgFilter: '',
    orgToken: '',
    orgStatus: [],
    orgVisibility: [],
    orgOrganization: [],
  }
  navigate.mockReset()
  getTokensMock.mockReset().mockResolvedValue({
    success: true,
    data: { items: [token()], total: 1, page: 1, page_size: 20 },
  })
  getOrganizationsMock.mockReset().mockResolvedValue({
    success: true,
    data: [{ id: 9, name: 'Acme' } as unknown as UserOrganization],
  })
})

function renderTable(): void {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <SelfOrganizationTokensTable />
    </QueryClientProvider>
  )
}

describe('SelfOrganizationTokensTable', () => {
  test('lists the organization, visibility and responsible member', async () => {
    renderTable()

    await waitFor(() => {
      expect(screen.getByText('Acme')).toBeInTheDocument()
    })

    for (const header of [
      'Organization',
      'Name',
      'Key',
      'Status',
      'Visibility',
      'Responsible user',
      'Remaining quota',
      'Used Quota',
      'Group',
      'Expires at',
    ]) {
      expect(
        screen.getByRole('columnheader', { name: header })
      ).toBeInTheDocument()
    }
    expect(screen.getByText('sk-acme********blic')).toBeInTheDocument()
    expect(screen.getByText('Public')).toBeInTheDocument()
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(
      screen.getByRole('link', { name: 'Manage in organization' })
    ).toBeInTheDocument()
  })

  test('passes the URL filters to the organization key endpoint', async () => {
    search.value = {
      orgPage: 2,
      orgPageSize: 10,
      orgFilter: 'acme',
      orgToken: 'public',
      orgStatus: ['1'],
      orgVisibility: ['public'],
      orgOrganization: ['9'],
    }
    renderTable()

    await waitFor(() => {
      expect(getTokensMock).toHaveBeenCalled()
    })

    expect(getTokensMock).toHaveBeenCalledWith({
      p: 2,
      size: 10,
      keyword: 'acme',
      token: 'public',
      status: 1,
      visibility: 'public',
      organization_id: 9,
    })
  })
})
