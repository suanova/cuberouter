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

import { getUsers } from '../../api'
import type { User } from '../../types'
import { UsersProvider } from '../users-provider'
import { UsersTable } from '../users-table'

const search = vi.hoisted(() => ({
  value: {} as Record<string, unknown>,
}))
const navigate = vi.hoisted(() => vi.fn())
const routeApi = vi.hoisted(() => ({
  useSearch: () => search.value,
  useNavigate: () => navigate,
}))

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  getRouteApi: () => routeApi,
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  getUsers: vi.fn(),
  searchUsers: vi.fn(),
}))

// @visactor/vchart ships ESM that vitest's externalized loader cannot resolve.
// The user dashboard dialog (never opened here) is the only chart consumer.
vi.mock('@visactor/react-vchart', () => ({
  VChart: () => null,
}))

const getUsersMock = vi.mocked(getUsers)

const INVITE_COUNT = 3

function buildUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    username: 'invite-user',
    display_name: 'Invite User',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    aff_count: INVITE_COUNT,
    aff_history_quota: 1000000,
    ...overrides,
  }
}

beforeEach(() => {
  window.localStorage.clear()
  search.value = {
    page: 1,
    pageSize: 20,
    filter: '',
    status: [],
    role: [],
    group: '',
  }
  navigate.mockReset()
  getUsersMock.mockReset().mockResolvedValue({
    success: true,
    data: {
      items: [buildUser()],
      total: 1,
      page: 1,
      page_size: 20,
    },
  })
})

function renderUsersTable(): void {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <UsersProvider>
        <UsersTable />
      </UsersProvider>
    </QueryClientProvider>
  )
}

describe('users table invite info column', () => {
  test('starts hidden and appears after enabling it in the View menu', async () => {
    renderUsersTable()

    await screen.findByText('invite-user')
    expect(
      screen.queryByRole('columnheader', { name: 'Invite Info' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByText(`Invited: ${INVITE_COUNT}`)
    ).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'View' }))
    const toggle = await screen.findByRole('menuitemcheckbox', {
      name: 'Invite Info',
    })
    expect(toggle).toHaveAttribute('aria-checked', 'false')

    await userEvent.click(toggle)
    expect(
      await screen.findByRole('columnheader', { name: 'Invite Info' })
    ).toBeInTheDocument()
    expect(screen.getByText(`Invited: ${INVITE_COUNT}`)).toBeInTheDocument()
  })
})
