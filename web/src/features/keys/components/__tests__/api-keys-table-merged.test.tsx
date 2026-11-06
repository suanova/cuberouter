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
import { render, screen, waitFor, within } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { copyToClipboard } from '@/lib/copy-to-clipboard'
import { useAuthStore } from '@/stores/auth-store'

import { getSelfApiKeys } from '../../api'
import type { ApiKey } from '../../types'
import { ApiKeysDialogs } from '../api-keys-dialogs'
import { ApiKeysProvider } from '../api-keys-provider'
import { ApiKeysTable } from '../api-keys-table'

const search = vi.hoisted(() => ({ value: {} as Record<string, unknown> }))
const updateOrganizationTokenMock = vi.hoisted(() => vi.fn())
const navigate = vi.hoisted(() => vi.fn())
const routeApi = vi.hoisted(() => ({
  useSearch: () => search.value,
  useNavigate: () => navigate,
}))

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  getRouteApi: () => routeApi,
}))

vi.mock('@/features/organization/api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/features/organization/api')>()),
  updateOrganizationToken: updateOrganizationTokenMock,
}))

vi.mock('@/lib/copy-to-clipboard', () => ({
  copyToClipboard: vi.fn().mockResolvedValue(true),
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  getSelfApiKeys: vi.fn(),
}))

const getSelfApiKeysMock = vi.mocked(getSelfApiKeys)
const copyToClipboardMock = vi.mocked(copyToClipboard)

let mobileViewport = false
Object.defineProperty(window, 'matchMedia', {
  configurable: true,
  value: (query: string) => ({
    matches: mobileViewport && query.includes('(max-width: 640px)'),
    media: query,
    onchange: null,
    addListener: () => undefined,
    removeListener: () => undefined,
    addEventListener: () => undefined,
    removeEventListener: () => undefined,
    dispatchEvent: () => false,
  }),
})

function apiKey(overrides: Partial<ApiKey> = {}): ApiKey {
  return {
    id: 1,
    name: 'Personal key',
    key: 'abcd**********wxyz',
    status: 1,
    remain_quota: 1_000_000,
    used_quota: 20_000,
    unlimited_quota: false,
    expired_time: -1,
    created_time: 1_700_000_000,
    accessed_time: 1_700_000_100,
    group: 'default',
    auto_groups: null,
    cross_group_retry: false,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    scope_type: 'personal',
    ...overrides,
  }
}

const personalKey = apiKey({
  model_limits_enabled: true,
  model_limits: '',
  allow_ips: '1.2.3.4',
})

const memberOrganizationKey = apiKey({
  id: 5,
  name: 'My org key',
  scope_type: 'organization',
  organization_id: 9,
  organization_name: 'Acme',
  visibility: 'private',
  creator_user_id: 7,
  responsible_user_id: 7,
  unlimited_quota: false,
  remain_quota: 500_000,
})

const organizationKey = apiKey({
  id: 2,
  name: 'Team key',
  key: 'orgsecret1234567890',
  key_preview: 'sk-orgs********7890',
  scope_type: 'organization',
  organization_id: 9,
  organization_name: 'Acme',
  visibility: 'public',
  unlimited_quota: true,
  model_limits_enabled: true,
  model_limits: 'gpt-5',
})

beforeEach(() => {
  mobileViewport = false
  updateOrganizationTokenMock.mockReset()
  search.value = {
    page: 1,
    pageSize: 20,
    filter: '',
    token: '',
    status: [],
  }
  navigate.mockReset()
  copyToClipboardMock.mockClear().mockResolvedValue(true)
  getSelfApiKeysMock.mockReset().mockResolvedValue({
    success: true,
    data: {
      items: [personalKey, organizationKey],
      total: 2,
      page: 1,
      page_size: 20,
    },
  })
})

function renderTable(): void {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <ApiKeysProvider>
        <ApiKeysTable />
        <ApiKeysDialogs />
      </ApiKeysProvider>
    </QueryClientProvider>
  )
}

async function findRow(name: string): Promise<HTMLElement> {
  const cell = await screen.findByText(name)
  const row = cell.closest('tr')
  if (!row) throw new Error(`row for ${name} not found`)
  return row
}

describe('ApiKeysTable', () => {
  test('renders personal and organization keys in one table', async () => {
    renderTable()

    await screen.findByText('Personal key')
    await screen.findByText('Team key')

    for (const header of [
      'Name',
      'Status',
      'API Key',
      'Billing Account',
      'Created',
      'Last Used',
      'Actions',
    ]) {
      expect(
        screen.getByRole('columnheader', { name: header })
      ).toBeInTheDocument()
    }
    for (const header of [
      'Group',
      'IP Restriction',
      'Expires',
      'Quota',
      'Models',
    ]) {
      expect(
        screen.queryByRole('columnheader', { name: header })
      ).not.toBeInTheDocument()
    }
    expect(screen.queryByRole('checkbox')).not.toBeInTheDocument()

    expect(screen.getByText('sk-abcd**********wxyz')).toBeInTheDocument()
    expect(screen.getByText('sk-orgs********7890')).toBeInTheDocument()
    expect(screen.queryByText('sk-orgsecret1234567890')).not.toBeInTheDocument()
    expect(screen.getByText('Personal')).toBeInTheDocument()
    expect(screen.getByText('Acme')).toBeInTheDocument()

    const organizationRow = await findRow('Team key')
    expect(
      within(organizationRow).queryByRole('button', { name: 'Edit' })
    ).not.toBeInTheDocument()
    expect(
      within(organizationRow).queryByRole('button', { name: 'Delete' })
    ).not.toBeInTheDocument()
    expect(
      within(organizationRow).queryByRole('button', { name: 'Disable' })
    ).not.toBeInTheDocument()
    expect(
      within(organizationRow).getByRole('button', { name: 'View' })
    ).toBeInTheDocument()
    expect(
      within(organizationRow).getByRole('button', { name: 'Copy API key' })
    ).toBeInTheDocument()

    const personalRow = await findRow('Personal key')
    for (const name of ['Disable', 'Edit', 'View', 'Delete']) {
      expect(
        within(personalRow).getByRole('button', { name })
      ).toBeInTheDocument()
    }
    expect(
      within(personalRow).queryByRole('button', { name: 'Open menu' })
    ).not.toBeInTheDocument()
  })

  test('shows read-only details for an organization key', async () => {
    renderTable()

    const organizationRow = await findRow('Team key')
    await userEvent.click(
      within(organizationRow).getByRole('button', { name: 'View' })
    )

    const sheet = await screen.findByRole('dialog')
    expect(within(sheet).getByText('API Key Details')).toBeInTheDocument()
    expect(within(sheet).getByText('Acme')).toBeInTheDocument()
    expect(within(sheet).getByText('Public')).toBeInTheDocument()
    expect(within(sheet).getByText('gpt-5')).toBeInTheDocument()
    expect(within(sheet).getByText('Unlimited')).toBeInTheDocument()
    expect(
      within(sheet).getByText(
        'Depends on the quota actually allocated to you by the organization.'
      )
    ).toBeInTheDocument()
  })

  test('shows personal key details without the organization quota note', async () => {
    renderTable()

    const personalRow = await findRow('Personal key')
    await userEvent.click(
      within(personalRow).getByRole('button', { name: 'View' })
    )

    const sheet = await screen.findByRole('dialog')
    expect(within(sheet).getByText('API Key Details')).toBeInTheDocument()
    expect(within(sheet).getByText('No models allowed')).toBeInTheDocument()
    expect(within(sheet).getByText('IP Restriction')).toBeInTheDocument()
    expect(within(sheet).getByText('1.2.3.4')).toBeInTheDocument()
    expect(within(sheet).getByText('Remaining quota')).toBeInTheDocument()
    expect(within(sheet).getByText('Used Quota')).toBeInTheDocument()
    expect(
      within(sheet).queryByText(
        'Depends on the quota actually allocated to you by the organization.'
      )
    ).not.toBeInTheDocument()
  })

  test('lets a member disable their own private organization key, without edit or delete', async () => {
    useAuthStore.getState().auth.setUser({
      id: 7,
      username: 'member',
      role: 1,
    })
    getSelfApiKeysMock.mockResolvedValue({
      success: true,
      data: {
        items: [memberOrganizationKey],
        total: 1,
        page: 1,
        page_size: 20,
      },
    })
    updateOrganizationTokenMock.mockResolvedValue({ success: true })
    renderTable()

    const row = await findRow('My org key')
    expect(within(row).queryByRole('button', { name: 'Edit' })).toBeNull()
    expect(within(row).queryByRole('button', { name: 'Delete' })).toBeNull()

    await userEvent.click(within(row).getByRole('button', { name: 'Disable' }))
    await waitFor(() =>
      expect(updateOrganizationTokenMock).toHaveBeenCalledTimes(1)
    )
    expect(updateOrganizationTokenMock).toHaveBeenCalledWith(
      'member',
      9,
      5,
      expect.objectContaining({
        name: 'My org key',
        status: 2,
        visibility: 'private',
        responsible_user_id: 7,
      }),
      { accountContextId: 9 }
    )
  })

  test('copies an organization key directly from the list', async () => {
    renderTable()

    const organizationRow = await findRow('Team key')
    await userEvent.click(
      within(organizationRow).getByRole('button', { name: 'Copy API key' })
    )

    await waitFor(() => {
      expect(copyToClipboardMock).toHaveBeenCalledWith('sk-orgsecret1234567890')
    })
  })

  test('passes status, keyword and token filters to the combined endpoint', async () => {
    search.value = {
      page: 2,
      pageSize: 10,
      filter: 'team',
      token: 'orgs',
      status: ['1'],
    }
    renderTable()

    await waitFor(() => {
      expect(getSelfApiKeysMock).toHaveBeenCalledWith({
        p: 2,
        size: 10,
        keyword: 'team',
        token: 'orgs',
        status: 1,
      })
    })
  })

  test('renders mobile cards with billing attribution and last used time', async () => {
    mobileViewport = true
    renderTable()

    await screen.findByText('Personal key')
    await screen.findByText('Team key')

    expect(screen.getByText('Acme')).toBeInTheDocument()
    expect(screen.getByText('Personal')).toBeInTheDocument()
    expect(screen.getAllByText('Last Used')).toHaveLength(2)
  })
})

afterEach(() => {
  useAuthStore.getState().auth.setUser(null)
})
