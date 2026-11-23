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
import { useState } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { CommonLogsFilterBar } from '../common-logs-filter-bar'
import { UsageLogsProvider, useUsageLogsContext } from '../usage-logs-provider'

const routeState = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
}))
const navigateMock = vi.hoisted(() => vi.fn())

vi.mock('@tanstack/react-router', () => ({
  getRouteApi: () => ({
    useSearch: () => routeState.search,
    useNavigate: () => navigateMock,
  }),
  useNavigate: () => navigateMock,
}))
vi.mock('@/components/data-table', () => ({
  DataTableViewOptions: () => null,
}))
vi.mock('../common-logs-stats', () => ({ CommonLogsStats: () => null }))
vi.mock('@/features/organization/api', () => ({ getOrganizations: vi.fn() }))

function renderBar() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <CommonLogsFilterBar table={{} as never} />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

// Mounts the bar only after switching to "Only Mine", then switches back to
// "All" — the draft lives across the scope change, like the real page.
function ScopeHarness() {
  const { setViewScope } = useUsageLogsContext()
  const [mounted, setMounted] = useState(false)
  return (
    <>
      <button
        type='button'
        onClick={() => {
          setViewScope('self')
          setMounted(true)
        }}
      >
        start-self
      </button>
      <button type='button' onClick={() => setViewScope('all')}>
        to-all
      </button>
      {mounted ? <CommonLogsFilterBar table={{} as never} /> : null}
    </>
  )
}

function renderScopeHarness() {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    <QueryClientProvider client={client}>
      <UsageLogsProvider>
        <ScopeHarness />
      </UsageLogsProvider>
    </QueryClientProvider>
  )
}

describe('CommonLogsFilterBar billing filter visibility', () => {
  beforeEach(() => {
    routeState.search = {}
    useAuthStore.getState().auth.setUser(null)
  })

  test('admin view hides the billing selector and keeps the organization filter', async () => {
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'admin',
      role: ROLE.ADMIN,
    })
    renderBar()

    expect(screen.queryByText('All Billing Types')).not.toBeInTheDocument()
    await userEvent.click(screen.getByRole('button', { name: /Expand/ }))
    expect(screen.getByPlaceholderText('Organization ID')).toBeInTheDocument()
  })

  test('a stale admin billing type is neither shown nor counted', () => {
    routeState.search = { billingAccountType: 'organization' }
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'admin',
      role: ROLE.ADMIN,
    })
    renderBar()

    expect(screen.queryByText('All Billing Types')).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Reset' })).toBeDisabled()
  })

  test('personal view keeps the billing selector', () => {
    useAuthStore.getState().auth.setUser({
      id: 2,
      username: 'member',
      role: ROLE.USER,
    })
    renderBar()

    expect(screen.getByText('All Billing Types')).toBeInTheDocument()
  })

  test('an admin scope switch does not re-emit a hidden billing type', async () => {
    routeState.search = { billingAccountType: 'organization' }
    useAuthStore.getState().auth.setUser({
      id: 1,
      username: 'admin',
      role: ROLE.ADMIN,
    })
    renderScopeHarness()

    await userEvent.click(screen.getByRole('button', { name: 'start-self' }))
    await userEvent.click(screen.getByRole('button', { name: 'to-all' }))
    expect(screen.queryByText('All Billing Types')).not.toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Search' }))
    const lastCall = navigateMock.mock.calls.at(-1)?.[0] as
      | { search?: Record<string, unknown> }
      | undefined
    expect(lastCall?.search).not.toHaveProperty('billingAccountType')
  })
})
