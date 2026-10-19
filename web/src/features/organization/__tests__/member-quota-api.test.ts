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
  getOrganizationMemberQuota,
  updateOrganizationMemberQuota,
} from '../api'

vi.mock('@/lib/api', () => ({
  api: { get: vi.fn(), put: vi.fn() },
}))

const ORGANIZATION_ID = 7
const MEMBER_USER_ID = 9

describe('organization member quota api', () => {
  beforeEach(() => {
    vi.mocked(api.get).mockReset()
    vi.mocked(api.put).mockReset()
  })

  it('reads the quota through the member-surface endpoint', async () => {
    vi.mocked(api.get).mockResolvedValue({
      data: {
        success: true,
        data: {
          limit: 5000,
          used: 1200,
          organization_quota: 9000,
          allocated: 6500,
          over_allocated: true,
        },
      },
    })

    await expect(
      getOrganizationMemberQuota(ORGANIZATION_ID, MEMBER_USER_ID, 'member')
    ).resolves.toEqual({
      limit: 5000,
      used: 1200,
      organization_quota: 9000,
      allocated: 6500,
      over_allocated: true,
    })
    expect(api.get).toHaveBeenCalledWith(
      `/api/organizations/${ORGANIZATION_ID}/members/${MEMBER_USER_ID}/quota`
    )
  })

  it('reads through the admin surface when the page is the platform console', async () => {
    vi.mocked(api.get).mockResolvedValue({ data: { success: true } })

    await getOrganizationMemberQuota(ORGANIZATION_ID, MEMBER_USER_ID, 'admin')

    expect(api.get).toHaveBeenCalledWith(
      `/api/admin/organizations/${ORGANIZATION_ID}/members/${MEMBER_USER_ID}/quota`
    )
  })

  it('defaults missing fields instead of presenting undefined numbers', async () => {
    // A malformed or empty payload must not turn into NaN in the dialog's
    // dollar conversion; an unrestricted, unused member is the safe reading.
    vi.mocked(api.get).mockResolvedValue({ data: { success: true } })

    await expect(
      getOrganizationMemberQuota(ORGANIZATION_ID, MEMBER_USER_ID, 'member')
    ).resolves.toEqual({
      limit: 0,
      used: 0,
      organization_quota: 0,
      allocated: 0,
      over_allocated: false,
    })
  })

  it('throws when the read is refused as HTTP 200', async () => {
    // The backend answers a refused read (member no longer active) with HTTP
    // 200 and `success: false`; defaulting it would show a member as
    // unrestricted.
    vi.mocked(api.get).mockResolvedValue({
      data: { success: false, message: 'member not active' },
    })

    await expect(
      getOrganizationMemberQuota(ORGANIZATION_ID, MEMBER_USER_ID, 'member')
    ).rejects.toThrow('member not active')
  })

  it('sends the limit in quota units to the member-surface endpoint', async () => {
    vi.mocked(api.put).mockResolvedValue({ data: { success: true } })

    await updateOrganizationMemberQuota(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      5000,
      'member'
    )

    expect(api.put).toHaveBeenCalledWith(
      `/api/organizations/${ORGANIZATION_ID}/members/${MEMBER_USER_ID}/quota`,
      { limit: 5000 }
    )
  })

  it('writes through the admin surface when the page is the platform console', async () => {
    vi.mocked(api.put).mockResolvedValue({ data: { success: true } })

    await updateOrganizationMemberQuota(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      0,
      'admin'
    )

    expect(api.put).toHaveBeenCalledWith(
      `/api/admin/organizations/${ORGANIZATION_ID}/members/${MEMBER_USER_ID}/quota`,
      { limit: 0 }
    )
  })

  it('throws when the write is refused', async () => {
    vi.mocked(api.put).mockResolvedValue({
      data: { success: false, message: 'limit must not be negative' },
    })

    await expect(
      updateOrganizationMemberQuota(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        -1,
        'member'
      )
    ).rejects.toThrow('limit must not be negative')
  })
})
