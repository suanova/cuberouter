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
import { renderHook, waitFor } from '@testing-library/react'
import type { ReactNode } from 'react'
import { afterEach, describe, expect, test, vi } from 'vitest'

import { useAuthStore } from '@/stores/auth-store'

import { getUserGroups, getUserModels } from '../../api'
import type { GroupOption } from '../../types'
import { usePlaygroundOptions } from '../use-playground-options'

vi.mock('../../api', () => ({
  getUserGroups: vi.fn(),
  getUserModels: vi.fn(),
}))

const mockedGetUserGroups = vi.mocked(getUserGroups)
const mockedGetUserModels = vi.mocked(getUserModels)

function renderOptions(params: {
  currentGroup: string
  currentModel: string
  onGroupChange: (group: string) => void
  setGroups?: (groups: GroupOption[]) => void
}) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = (props: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      {props.children}
    </QueryClientProvider>
  )

  return renderHook(
    () =>
      usePlaygroundOptions({
        currentGroup: params.currentGroup,
        currentModel: params.currentModel,
        setGroups: params.setGroups ?? vi.fn(),
        setModels: vi.fn(),
        updateConfig: (key, value) => {
          if (key === 'group') params.onGroupChange(String(value))
        },
      }),
    { wrapper }
  )
}

afterEach(() => {
  vi.clearAllMocks()
  useAuthStore.setState((state) => ({
    auth: { ...state.auth, user: null },
  }))
})

describe('playground options', () => {
  test('loads models for the account group instead of the stored default group', async () => {
    useAuthStore.setState((state) => ({
      auth: {
        ...state.auth,
        user: { id: 1, username: 'yeebo-user', role: 1, group: 'yeebo' },
      },
    }))
    mockedGetUserModels.mockResolvedValue([
      { label: 'yeebo-model', value: 'yeebo-model' },
    ])
    const groups: GroupOption[] = [
      { label: 'default', value: 'default', ratio: 1 },
      { label: 'yeebo', value: 'yeebo', ratio: 1 },
    ]
    mockedGetUserGroups.mockResolvedValue(groups)
    const onGroupChange = vi.fn()
    const setGroups = vi.fn()

    renderOptions({
      currentGroup: 'default',
      currentModel: 'yeebo-model',
      onGroupChange,
      setGroups,
    })

    await waitFor(() => {
      expect(mockedGetUserModels).toHaveBeenCalledWith('yeebo')
    })
    // 选择器隐藏时也写入分组，否则模型选择器会因为 groups 为空而禁用。
    await waitFor(() => {
      expect(setGroups).toHaveBeenCalledWith(groups)
    })
    await waitFor(() => {
      expect(onGroupChange).toHaveBeenCalledWith('yeebo')
    })
  })

  test('keeps following the account group when it changes', async () => {
    useAuthStore.setState((state) => ({
      auth: {
        ...state.auth,
        user: { id: 1, username: 'switching-user', role: 1, group: 'yeebo' },
      },
    }))
    mockedGetUserModels.mockResolvedValue([])
    mockedGetUserGroups.mockResolvedValue([])

    const { rerender } = renderOptions({
      currentGroup: 'yeebo',
      currentModel: '',
      onGroupChange: vi.fn(),
    })
    await waitFor(() => {
      expect(mockedGetUserModels).toHaveBeenCalledWith('yeebo')
    })

    useAuthStore.setState((state) => ({
      auth: {
        ...state.auth,
        user: { id: 1, username: 'switching-user', role: 1, group: 'suanova' },
      },
    }))
    rerender()

    await waitFor(() => {
      expect(mockedGetUserModels).toHaveBeenCalledWith('suanova')
    })
  })
})
