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
import { describe, expect, test, vi } from 'vitest'

import { getAllTaskLogs, getUserTaskLogs } from '../../api'
import { fetchLogsByCategory } from '../utils'

vi.mock('../../api', () => ({
  getAllLogs: vi.fn(),
  getUserLogs: vi.fn(),
  getLogStats: vi.fn(),
  getUserLogStats: vi.fn(),
  getAllMidjourneyLogs: vi.fn(),
  getUserMidjourneyLogs: vi.fn(),
  getAllTaskLogs: vi.fn(),
  getUserTaskLogs: vi.fn(),
}))

const getUserTaskLogsMock = vi.mocked(getUserTaskLogs)
const getAllTaskLogsMock = vi.mocked(getAllTaskLogs)

const ok = {
  success: true,
  data: { items: [], total: 0, page: 1, page_size: 20 },
}

describe('task log API params', () => {
  test('the personal page asks for the organization union and task filters', async () => {
    getUserTaskLogsMock.mockResolvedValue(ok)
    await fetchLogsByCategory({
      logCategory: 'task',
      isAdmin: false,
      page: 1,
      pageSize: 20,
      columnFilters: [],
      searchParams: {
        filter: 'task-1',
        status: 'SUCCESS',
        action: 'generate',
        billingAccountType: 'organization',
        organizationId: '7',
        startTime: 1_700_000_000_000,
        endTime: 1_700_003_600_000,
      },
    })

    expect(getUserTaskLogsMock).toHaveBeenCalledWith(
      expect.objectContaining({
        task_id: 'task-1',
        status: 'SUCCESS',
        action: 'generate',
        include_organization: true,
        billing_account_type: 'organization',
        organization_id: 7,
      })
    )
  })

  test('the admin page uses the all-tasks endpoint without a union flag', async () => {
    getAllTaskLogsMock.mockResolvedValue(ok)
    await fetchLogsByCategory({
      logCategory: 'task',
      isAdmin: true,
      page: 1,
      pageSize: 20,
      columnFilters: [],
      searchParams: {
        channel: '3',
        billingAccountType: 'personal',
      },
    })

    const params = getAllTaskLogsMock.mock.calls[0]?.[0]
    expect(params).toEqual(
      expect.objectContaining({
        channel_id: '3',
        billing_account_type: 'personal',
      })
    )
    expect(params).not.toHaveProperty('include_organization')
  })
})
