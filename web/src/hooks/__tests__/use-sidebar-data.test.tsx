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
import { describe, expect, test, vi } from 'vitest'

const { getOrganizations } = vi.hoisted(() => ({
  getOrganizations: vi.fn(),
}))

vi.mock('@/features/organization/api', () => ({ getOrganizations }))

const { useSidebarData } = await import('../use-sidebar-data')

const SIDEBAR_ORGANIZATIONS_QUERY_KEY = [
  'organizations',
  'sidebar',
  'personal',
]

async function renderSidebar(organizations: Array<{ role: string }>) {
  getOrganizations.mockResolvedValue({ success: true, data: organizations })
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const wrapper = (props: { children: ReactNode }) => (
    <QueryClientProvider client={queryClient}>
      {props.children}
    </QueryClientProvider>
  )
  const { result } = renderHook(() => useSidebarData(), { wrapper })
  await waitFor(() =>
    expect(
      queryClient.getQueryState(SIDEBAR_ORGANIZATIONS_QUERY_KEY)?.status
    ).toBe('success')
  )
  return result
}

function hasOrganizationsEntry(
  result: ReturnType<typeof useSidebarData>
): boolean {
  return result.navGroups
    .flatMap((group) => group.items)
    .some((item) => item.title === 'Organizations')
}

describe('organization center sidebar entry', () => {
  test('is shown to an owner or administrator', async () => {
    const result = await renderSidebar([{ role: 'owner' }])
    expect(hasOrganizationsEntry(result.current)).toBe(true)
  })

  test('is hidden from a plain member', async () => {
    const result = await renderSidebar([{ role: 'member' }])
    expect(hasOrganizationsEntry(result.current)).toBe(false)
  })
})
