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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'
import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import { channelSchema } from '../../../types'
import { ChannelsProvider } from '../../channels-provider'
import { ChannelMutateDrawer } from '../channel-mutate-drawer'

// The third-party icon bundle uses directory ESM imports unsupported by Node.
// Keep the actual drawer/form/API path; decorative icons are outside this test.
vi.mock('@/lib/lobe-icon', () => ({ getLobeIcon: () => null }))

afterEach(() => useAuthStore.getState().auth.reset())

test('editing a reviewer restores its CA certificate and saves the entered PEM as other', async () => {
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'root', role: ROLE.SUPER_ADMIN })
  const savedPEM =
    '-----BEGIN CERTIFICATE-----\nsaved-test-certificate\n-----END CERTIFICATE-----'
  const editedPEM =
    '-----BEGIN CERTIFICATE-----\nreplacement-test-certificate\n-----END CERTIFICATE-----'
  const channel = channelSchema.parse({
    id: 65,
    type: 65,
    name: 'reviewer',
    key: '',
    status: 1,
    base_url: 'https://reviewer.example/moderate',
    other: savedPEM,
    models: 'google/shieldgemma-2-4b-it',
    group: 'default',
    created_time: 1,
    test_time: 0,
    response_time: 0,
    balance_updated_time: 0,
  })
  vi.spyOn(api, 'get').mockImplementation(async (url) => {
    if (url === '/api/channel/65') {
      return { data: { success: true, data: channel } }
    }
    return { data: { success: true, data: [] } }
  })
  const put = vi
    .spyOn(api, 'put')
    .mockResolvedValue({ data: { success: true } })
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  render(
    <QueryClientProvider client={client}>
      <ChannelsProvider>
        <ChannelMutateDrawer open currentRow={channel} onOpenChange={vi.fn()} />
      </ChannelsProvider>
    </QueryClientProvider>
  )
  const field = await screen.findByRole('textbox', {
    name: 'Reviewer CA certificate (optional)',
  })
  expect(field).toHaveValue(savedPEM)
  fireEvent.change(field, { target: { value: editedPEM } })
  fireEvent.click(screen.getByRole('button', { name: 'Update Channel' }))
  await waitFor(() =>
    expect(put).toHaveBeenCalledWith(
      '/api/channel/',
      expect.objectContaining({ id: 65, other: editedPEM }),
      expect.anything()
    )
  )
  client.clear()
})
