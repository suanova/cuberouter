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
import { CanceledError, type AxiosAdapter } from 'axios'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/http-client'

import { workflowAPI } from '../workflow-api'

vi.mock('@/lib/auth-session', () => ({
  applyAuthRotation: vi.fn(),
  clearAuthentication: vi.fn(),
  refreshAuthentication: vi.fn(),
}))

const originalAdapter = api.defaults.adapter
afterEach(() => {
  api.defaults.adapter = originalAdapter
})

test('unmounting one private image does not cancel another mounted copy of the same asset', async () => {
  const completions: Array<() => void> = []
  const adapter: AxiosAdapter = (config) =>
    new Promise((resolve, reject) => {
      config.signal?.addEventListener?.('abort', () =>
        reject(new CanceledError('unmounted'))
      )
      completions.push(() =>
        resolve({
          data: new Blob(['png']),
          status: 200,
          statusText: 'OK',
          headers: {},
          config,
        })
      )
    })
  api.defaults.adapter = adapter
  const unmount = new AbortController()
  const first = workflowAPI
    .asset('same-asset', unmount.signal)
    .catch((error: unknown) => error)
  const surviving = workflowAPI.asset('same-asset')
  await vi.waitFor(() => expect(completions).toHaveLength(2))
  unmount.abort()
  completions[1]()
  expect(await first).toBeInstanceOf(CanceledError)
  await expect(surviving).resolves.toBeInstanceOf(Blob)
})
