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
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  createOrganizationJoinRules,
  deleteOrganizationJoinRule,
  listOrganizationJoinRules,
} from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), post: vi.fn(), delete: vi.fn() },
}))

describe('organization join rules api', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.post).mockReset()
    vi.mocked(api.delete).mockReset()
  })

  it('lists rules through the platform admin endpoint', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { success: true, data: [] } })
    await listOrganizationJoinRules(7)
    expect(api.get).toHaveBeenCalledWith('/api/admin/organizations/7/join-rules')
  })

  it('creates a batch with a reason and leaves the error response to the caller', async () => {
    vi.mocked(api.post).mockResolvedValue({
      data: { success: true, data: { rules: [], notices: [] } },
    })
    await createOrganizationJoinRules(7, {
      patterns: ['*.enterprise.com'],
      reason: 'onboarding',
    })
    // `skipErrorHandler` is part of the contract: a batch with one bad line
    // answers 400 with the per-line reasons in `line_errors`, and the
    // interceptor's toast would replace them with one generic message.
    expect(api.post).toHaveBeenCalledWith(
      '/api/admin/organizations/7/join-rules',
      {
        patterns: ['*.enterprise.com'],
        reason: 'onboarding',
      },
      { skipErrorHandler: true }
    )
  })

  it('deletes with the reason in the body', async () => {
    vi.mocked(api.delete).mockResolvedValue({ data: { success: true } })
    await deleteOrganizationJoinRule(7, 3, 'offboarding')
    expect(api.delete).toHaveBeenCalledWith(
      '/api/admin/organizations/7/join-rules/3',
      {
        data: { reason: 'offboarding' },
      }
    )
  })
})
