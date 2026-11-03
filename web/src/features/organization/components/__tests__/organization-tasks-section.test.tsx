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

import { formatQuota } from '@/lib/format'

import { listOrganizationTasks } from '../../api'
import type { OrganizationTaskRow } from '../../types'
import { OrganizationTasksSection } from '../organization-tasks-section'

// The URL is the page's state, not this section's; the harness supplies a fixed
// empty set of search params instead of standing up a route.
const pageRoute = vi.hoisted(() => ({ search: {}, navigate: vi.fn() }))

vi.mock('../organization-page-provider', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../organization-page-provider')>()),
  useOrganizationSectionRoute: () => pageRoute,
  useOrganizationSurface: () => 'member',
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  listOrganizationTasks: vi.fn(),
}))

const listTasksMock = vi.mocked(listOrganizationTasks)

const ORGANIZATION_ID = 7

function taskRow(
  overrides: Partial<OrganizationTaskRow> = {}
): OrganizationTaskRow {
  return {
    id: 1,
    created_at: 1_700_000_000,
    updated_at: 1_700_000_000,
    task_id: 'task-1',
    platform: 'mock-platform',
    user_id: 42,
    group: 'default',
    channel_id: 2,
    quota: 800_000,
    original_quota: 1_000_000,
    token_id: 3,
    token_name: 'key-1',
    request_id: 'request-1',
    creator_user_id: 42,
    creator_name: 'Alice',
    responsible_user_id: 42,
    responsible_name: 'Alice',
    action: 'generate',
    status: 'SUCCESS',
    fail_reason: '',
    submit_time: 1_700_000_000,
    start_time: 1_700_000_001,
    finish_time: 1_700_000_002,
    progress: '50%',
    properties: { input: 'prompt' },
    username: 'alice',
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
      <OrganizationTasksSection
        organizationId={ORGANIZATION_ID}
        canView
        canViewWideData
        onForbidden={() => undefined}
      />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  listTasksMock.mockReset().mockResolvedValue({
    success: true,
    data: {
      page: 1,
      page_size: 20,
      total: 1,
      items: [taskRow()],
    },
  })
})

describe('organization tasks section', () => {
  test('has no Midjourney tab while the platform has no Midjourney upstream', () => {
    renderSection()

    expect(screen.queryByRole('tab')).not.toBeInTheDocument()
    expect(screen.queryByText('Midjourney')).not.toBeInTheDocument()
  })

  test('shows names, task id, type and split prices without progress or artifacts', async () => {
    renderSection()

    await waitFor(
      () => {
        expect(screen.getByText('Alice')).toBeInTheDocument()
        expect(screen.getByText('key-1')).toBeInTheDocument()
        expect(screen.getByText('task-1')).toBeInTheDocument()
        expect(screen.getByText('Image to Video')).toBeInTheDocument()
        expect(screen.getByText(formatQuota(1_000_000))).toBeInTheDocument()
        expect(screen.getByText(formatQuota(800_000))).toBeInTheDocument()
      },
      { timeout: 5_000 }
    )

    for (const header of [
      'Submit Time',
      'Task ID',
      'Type',
      'Member',
      'Key',
      'Original price',
      'Discounted price',
      'Duration',
      'Status',
      'Details',
    ]) {
      expect(
        screen.getByRole('columnheader', { name: header })
      ).toBeInTheDocument()
    }
    expect(
      screen.queryByRole('columnheader', { name: 'Progress' })
    ).not.toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Artifacts' })
    ).not.toBeInTheDocument()
    expect(screen.queryByText('50%')).not.toBeInTheDocument()
  })

  test('hides the Excel export action in the task toolbar', async () => {
    renderSection()

    await waitFor(
      () => {
        expect(screen.getByText('Alice')).toBeInTheDocument()
      },
      { timeout: 5_000 }
    )

    expect(
      screen.queryByRole('button', { name: /Export/ })
    ).not.toBeInTheDocument()
  })
})
