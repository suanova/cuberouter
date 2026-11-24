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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  deleteAdminApiKey,
  fetchAdminTokenKey,
  getAdminApiKeys,
  updateAdminApiKeyStatus,
} from '../api'

vi.mock('@/lib/api', () => ({
  api: {
    get: vi.fn(),
    post: vi.fn(),
    put: vi.fn(),
    delete: vi.fn(),
  },
}))

const apiGetMock = vi.mocked(api.get)
const apiPostMock = vi.mocked(api.post)
const apiPutMock = vi.mocked(api.put)
const apiDeleteMock = vi.mocked(api.delete)

beforeEach(() => {
  apiGetMock.mockReset().mockResolvedValue({ data: { success: true } })
  apiPostMock.mockReset().mockResolvedValue({ data: { success: true } })
  apiPutMock.mockReset().mockResolvedValue({ data: { success: true } })
  apiDeleteMock.mockReset().mockResolvedValue({ data: { success: true } })
})

describe('admin token API client', () => {
  test('lists all-users keys with filters', async () => {
    await getAdminApiKeys({
      p: 2,
      size: 10,
      keyword: 'k',
      token: 't',
      status: 1,
      username: 'alice',
    })

    expect(apiGetMock).toHaveBeenCalledWith('/api/admin/token/', {
      params: {
        p: 2,
        size: 10,
        keyword: 'k',
        token: 't',
        status: 1,
        username: 'alice',
      },
    })
  })

  test('reveals a single key through the admin endpoint', async () => {
    await fetchAdminTokenKey(7)
    expect(apiPostMock).toHaveBeenCalledWith('/api/admin/token/7/key')
  })

  test('updates status through the admin endpoint', async () => {
    await updateAdminApiKeyStatus(7, 2)
    expect(apiPutMock).toHaveBeenCalledWith('/api/admin/token/7/status', {
      status: 2,
    })
  })

  test('deletes through the admin endpoint', async () => {
    await deleteAdminApiKey(7)
    expect(apiDeleteMock).toHaveBeenCalledWith('/api/admin/token/7')
  })
})
