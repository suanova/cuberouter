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

import { getOrganizationMemberRestrictionOptions } from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), put: vi.fn() },
}))

const ORGANIZATION_ID = 7

describe('organization member restriction options api', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.put).mockReset()
  })

  it('reads the restriction options from the organization endpoint', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          models: ['m1'],
          organization_quota: 9000,
          allocated: 6500,
          over_allocated: true,
        },
      },
    })

    await expect(
      getOrganizationMemberRestrictionOptions(ORGANIZATION_ID)
    ).resolves.toEqual({
      models: ['m1'],
      organization_quota: 9000,
      allocated: 6500,
      over_allocated: true,
    })
    expect(api.get).toHaveBeenCalledWith(
      `/api/organizations/${ORGANIZATION_ID}/member-restriction-options`
    )
  })

  it('defaults a missing payload to an unrestricted empty budget', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { success: true } })

    await expect(
      getOrganizationMemberRestrictionOptions(ORGANIZATION_ID)
    ).resolves.toEqual({
      models: [],
      organization_quota: 0,
      allocated: 0,
      over_allocated: false,
    })
  })

  it('throws when the read is refused as HTTP 200', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: { success: false, message: 'permission denied' },
    })

    await expect(
      getOrganizationMemberRestrictionOptions(ORGANIZATION_ID)
    ).rejects.toThrow('permission denied')
  })
})
