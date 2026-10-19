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
import { render, screen } from '@testing-library/react'
import { expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import type { OrganizationTokenRow } from '../../types'
import { OrganizationTokenFormDialog } from '../organization-token-form-dialog'

vi.mock(
  '@/features/organization/hooks/use-organization-member-options',
  () => ({
    useOrganizationTokenResponsibleOptions: () => ({
      options: [{ value: 42, label: 'alice' }],
      isLoading: false,
    }),
  })
)

const EDITED_KEY: OrganizationTokenRow = {
  id: 11,
  user_id: 42,
  key: 'abcdefghijklmnop',
  status: 1,
  name: 'handover-key',
  created_time: 0,
  accessed_time: 0,
  expired_time: -1,
  remain_quota: 0,
  unlimited_quota: true,
  model_limits_enabled: false,
  model_limits: '',
  used_quota: 0,
  group: '',
  cross_group_retry: false,
  visibility: 'private',
  creator_user_id: 7,
  creator_username: 'creator-alice',
  responsible_user_id: 42,
}

function renderDialog(editing: OrganizationTokenRow | null): void {
  render(
    <QueryClientProvider client={new QueryClient()}>
      <OrganizationTokenFormDialog
        organizationId={7}
        open
        onOpenChange={() => undefined}
        editing={editing}
        canManageAllTokens
        currentUserId={42}
        isOrganizationMember
        groupOptions={[{ value: 'vip', label: 'VIP', desc: 'Priority access' }]}
        onSaved={() => undefined}
      />
    </QueryClientProvider>
  )
}

test('names the creating member as the owner when editing a key', () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  } as never)

  renderDialog(EDITED_KEY)

  const owner = screen.getByText('Owner').parentElement
  expect(owner?.textContent).toContain('creator-alice')
})

test('shows no owner while the key is still being created', () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  } as never)

  renderDialog(null)

  expect(screen.queryByText('Owner')).toBeNull()
})
