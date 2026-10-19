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

const QUOTA_LIMITED_KEY: OrganizationTokenRow = {
  id: 11,
  user_id: 42,
  key: 'abcdefghijklmnop',
  status: 1,
  name: 'limited-key',
  created_time: 0,
  accessed_time: 0,
  expired_time: -1,
  remain_quota: 5_000_000,
  unlimited_quota: false,
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

/**
 * The quota amount is money in the deployment's own display currency — a
 * deployment billing in CNY enters 元 — so the field must not offer chip
 * presets worded as dollar amounts.
 */
test('an edited key with a quota limit shows no dollar-style quota presets', () => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  } as never)

  render(
    <QueryClientProvider client={new QueryClient()}>
      <OrganizationTokenFormDialog
        organizationId={7}
        open
        onOpenChange={() => undefined}
        editing={QUOTA_LIMITED_KEY}
        canManageAllTokens
        currentUserId={42}
        isOrganizationMember
        groupOptions={[{ value: 'vip', label: 'VIP', desc: 'Priority access' }]}
        onSaved={() => undefined}
      />
    </QueryClientProvider>
  )

  expect(screen.getByLabelText('Key quota limit')).toBeTruthy()
  expect(screen.queryAllByRole('button', { name: /dollar/i })).toHaveLength(0)
})
