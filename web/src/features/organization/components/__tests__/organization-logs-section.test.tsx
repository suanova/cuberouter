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

import {
  getOrganizationLogStats,
  listOrganizationLogs,
  listOrganizationMembers,
} from '../../api'
import type { OrganizationLogRow } from '../../types'
import { OrganizationLogsSection } from '../organization-logs-section'

// The URL is the page's state, not this section's; the harness supplies a fixed
// set of search params instead of standing up a route.
const pageRoute = vi.hoisted(() => ({
  search: {} as Record<string, unknown>,
  navigate: vi.fn(),
}))

vi.mock('../organization-page-provider', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../organization-page-provider')>()),
  useOrganizationSectionRoute: () => pageRoute,
  useOrganizationSurface: () => 'member',
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  listOrganizationLogs: vi.fn(),
  getOrganizationLogStats: vi.fn(),
  listOrganizationMembers: vi.fn(),
}))

// @lobehub/icons transitively imports @emoji-mart JSON assets that vitest's
// externalized ESM loader rejects. Icons are irrelevant to the filter contract
// under test, so the icon loader boundary is stubbed.
vi.mock('@/lib/lobe-icon', () => ({
  getLobeIcon: () => null,
}))

const listLogsMock = vi.mocked(listOrganizationLogs)
const logStatsMock = vi.mocked(getOrganizationLogStats)
const listMembersMock = vi.mocked(listOrganizationMembers)

const ORGANIZATION_ID = 7

function logRow(
  overrides: Partial<OrganizationLogRow> = {}
): OrganizationLogRow {
  return {
    id: 1,
    user_id: 42,
    created_at: 1_700_000_000,
    type: 2,
    token_name: 'key-1',
    model_name: 'gpt-4o',
    quota: 1_000,
    prompt_tokens: 10,
    completion_tokens: 5,
    use_time: 1,
    is_stream: false,
    group: 'vip-group',
    request_id: 'request-1',
    ...overrides,
  }
}

function renderSection(): void {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <OrganizationLogsSection
        organizationId={ORGANIZATION_ID}
        canView
        canViewWideData
        onForbidden={() => undefined}
      />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  pageRoute.search = {}
  listLogsMock.mockReset().mockResolvedValue({
    success: true,
    data: {
      page: 1,
      page_size: 20,
      total: 1,
      items: [logRow()],
    },
  })
  logStatsMock.mockReset().mockResolvedValue({
    success: true,
    data: { quota: 1_000, rpm: 1, tpm: 2 },
  })
  listMembersMock.mockReset().mockResolvedValue({
    success: true,
    data: { page: 1, page_size: 100, total: 0, items: [] },
  })
})

describe('organization logs section', () => {
  test('offers the type and responsible filters but not the group filter', () => {
    renderSection()

    expect(screen.getByText('All Types')).toBeInTheDocument()
    expect(screen.getByText('All members')).toBeInTheDocument()
    expect(
      screen.getByPlaceholderText('Upstream Request ID')
    ).toBeInTheDocument()
    expect(screen.queryByPlaceholderText('Group')).not.toBeInTheDocument()
  })

  test('renders the caller column but not a group column or value', async () => {
    renderSection()

    // Wait for the current DOM, not a node `findByText` happened to catch
    // during a transient render: the table can replace its rows once the URL
    // state settles, and asserting on the detached node fails the test.
    await waitFor(
      () => {
        expect(screen.getByText('request-1')).toBeInTheDocument()
      },
      { timeout: 5_000 }
    )
    expect(
      screen.getByRole('columnheader', { name: 'Type' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Caller' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Group' })
    ).not.toBeInTheDocument()
    expect(screen.queryByText('vip-group')).not.toBeInTheDocument()
  })

  test('sends type, upstream request and responsible filters but never group', async () => {
    pageRoute.search = {
      logType: '2',
      logUpstreamRequest: 'up-1',
      logResponsible: '42',
    }
    renderSection()

    await waitFor(
      () => {
        expect(listLogsMock).toHaveBeenCalled()
        expect(logStatsMock).toHaveBeenCalled()
      },
      { timeout: 5_000 }
    )

    const listParams = listLogsMock.mock.calls[0]?.[2]
    expect(listParams?.type).toBe(2)
    expect(listParams?.upstream_request_id).toBe('up-1')
    expect(listParams?.responsible_user_id).toBe(42)
    expect(listParams).not.toHaveProperty('group')

    const statsParams = logStatsMock.mock.calls[0]?.[2]
    expect(statsParams?.type).toBe(2)
    expect(statsParams?.upstream_request_id).toBe('up-1')
    expect(statsParams?.responsible_user_id).toBe(42)
    expect(statsParams).not.toHaveProperty('group')
  })
})
