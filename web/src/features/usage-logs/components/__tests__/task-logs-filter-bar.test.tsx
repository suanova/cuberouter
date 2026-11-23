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

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { TaskLogsFilterBar } from '../task-logs-filter-bar'
import { UsageLogsProvider } from '../usage-logs-provider'

const routeState = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
}))

vi.mock('@tanstack/react-router', () => ({
  getRouteApi: () => ({
    useSearch: () => routeState.search,
    useNavigate: () => vi.fn(),
  }),
  useNavigate: () => vi.fn(),
}))
vi.mock('@/components/data-table', () => ({
  DataTableViewOptions: () => null,
}))
vi.mock('@/features/organization/api', () => ({
  getOrganizations: vi.fn().mockResolvedValue({ data: [] }),
}))

function renderBar() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <TaskLogsFilterBar table={{} as never} logCategory='task' />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

function setRole(role: number) {
  useAuthStore.getState().auth.setUser({ id: 1, username: 'u', role })
}

describe('TaskLogsFilterBar filters', () => {
  beforeEach(() => {
    routeState.search = {}
    useAuthStore.getState().auth.setUser(null)
  })

  test('admin view hides billing and expands to the parity filters', async () => {
    setRole(ROLE.ADMIN)
    renderBar()

    expect(screen.queryByText('All Billing Types')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Expand/ }))

    for (const placeholder of [
      'Model Name',
      'Token Name',
      'Request ID',
      'Platform',
      'Username',
      'Group',
      'Channel ID',
      'Organization ID',
    ]) {
      expect(screen.getByPlaceholderText(placeholder)).toBeInTheDocument()
    }
  })

  test('a stale admin billing type is neither shown nor counted', () => {
    routeState.search = { billingAccountType: 'organization' }
    setRole(ROLE.ADMIN)
    renderBar()

    expect(screen.queryByText('All Billing Types')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reset' })).toBeDisabled()
  })

  test('personal view keeps billing and hides admin-only filters', async () => {
    setRole(ROLE.USER)
    renderBar()

    expect(screen.getByText('All Billing Types')).toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Expand/ }))

    for (const placeholder of [
      'Model Name',
      'Token Name',
      'Request ID',
      'Platform',
    ]) {
      expect(screen.getByPlaceholderText(placeholder)).toBeInTheDocument()
    }
    for (const placeholder of [
      'Username',
      'Group',
      'Channel ID',
      'Organization ID',
    ]) {
      expect(screen.queryByPlaceholderText(placeholder)).not.toBeInTheDocument()
    }
  })
})
