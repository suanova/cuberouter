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
import { act, renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getStatus } from '@/lib/api'
import { DEFAULT_LOGO } from '@/lib/constants'
import { useSystemConfigStore } from '@/stores/system-config-store'

import { updateSystemOption } from '../../api'
import { useUpdateOption } from '../use-update-option'

vi.mock('../../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../api')>()
  return { ...actual, updateSystemOption: vi.fn() }
})

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return { ...actual, getStatus: vi.fn() }
})

const logoDataUrl =
  'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='

const queryClient = new QueryClient({
  defaultOptions: { queries: { retry: false } },
})

function wrapper({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
  )
}

beforeEach(() => {
  queryClient.clear()
  useSystemConfigStore.getState().setConfig({ logo: DEFAULT_LOGO })
  vi.mocked(updateSystemOption).mockResolvedValue({
    success: true,
    message: '',
  })
  vi.mocked(getStatus).mockResolvedValue({
    system_name: 'Demo',
    logo: logoDataUrl,
  })
})

describe('useUpdateOption', () => {
  test('refreshes the shared system config when the logo changes', async () => {
    const { result } = renderHook(() => useUpdateOption(), { wrapper })

    act(() => {
      result.current.mutate({ key: 'Logo', value: logoDataUrl })
    })

    await waitFor(() => {
      expect(useSystemConfigStore.getState().config.logo).toBe(logoDataUrl)
    })
    expect(getStatus).toHaveBeenCalledTimes(1)
  })

  test('does not refetch status for unrelated options', async () => {
    const { result } = renderHook(() => useUpdateOption(), { wrapper })

    act(() => {
      result.current.mutate({ key: 'SomeUnrelatedOption', value: 'value' })
    })

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true)
    })
    expect(getStatus).not.toHaveBeenCalled()
  })
})
