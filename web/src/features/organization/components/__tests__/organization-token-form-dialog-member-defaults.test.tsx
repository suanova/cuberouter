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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import { createOrganizationToken, updateOrganizationToken } from '../../api'
import type { OrganizationTokenRow } from '../../types'
import { OrganizationPageProvider } from '../organization-page-provider'
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

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  createOrganizationToken: vi.fn(),
  updateOrganizationToken: vi.fn(),
}))

const ORGANIZATION_ID = 7
const CURRENT_USER_ID = 42

const EDITED_KEY: OrganizationTokenRow = {
  id: 11,
  user_id: CURRENT_USER_ID,
  key: 'abcdefghijklmnop',
  status: 1,
  name: 'member-key',
  created_time: 0,
  accessed_time: 0,
  expired_time: -1,
  remain_quota: 20,
  unlimited_quota: false,
  model_limits_enabled: false,
  model_limits: '',
  used_quota: 0,
  group: '',
  cross_group_retry: false,
  visibility: 'private',
  creator_user_id: CURRENT_USER_ID,
  creator_username: 'alice',
  responsible_user_id: CURRENT_USER_ID,
}

const CREATED_KEY: OrganizationTokenRow = { ...EDITED_KEY, id: 12 }

const EXPIRED_KEY: OrganizationTokenRow = {
  ...EDITED_KEY,
  id: 13,
  name: 'expired-legacy',
  expired_time: Math.floor(Date.now() / 1000) - 3600,
}

function dialogElement(
  queryClient: QueryClient,
  editing: OrganizationTokenRow | null,
  canManageAllTokens: boolean,
  isPlatformAdministrator = false
) {
  return (
    <QueryClientProvider client={queryClient}>
      <OrganizationPageProvider
        surface='member'
        search={{}}
        navigate={() => undefined}
      >
        <OrganizationTokenFormDialog
          organizationId={ORGANIZATION_ID}
          open
          onOpenChange={() => undefined}
          editing={editing}
          canManageAllTokens={canManageAllTokens}
          isPlatformAdministrator={isPlatformAdministrator}
          currentUserId={CURRENT_USER_ID}
          isOrganizationMember
          groupOptions={[]}
          onSaved={() => undefined}
        />
      </OrganizationPageProvider>
    </QueryClientProvider>
  )
}

function renderDialog(
  editing: OrganizationTokenRow | null = null,
  canManageAllTokens = false,
  isPlatformAdministrator = false
) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  return render(
    dialogElement(
      queryClient,
      editing,
      canManageAllTokens,
      isPlatformAdministrator
    )
  )
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: [] },
  } as never)
})

afterEach(() => {
  vi.restoreAllMocks()
})

test('a member creating a key sees only the name field and policy note', () => {
  renderDialog(null, false)

  expect(screen.getByLabelText('Name')).toBeTruthy()
  expect(screen.queryByText('Create count')).toBeNull()
  expect(screen.queryByText('Unlimited Quota')).toBeNull()
  expect(screen.queryByText('Expiration Time')).toBeNull()
  expect(screen.queryByText('Model Limits')).toBeNull()
  expect(screen.queryByText('IP limits')).toBeNull()
  expect(
    screen.getByText(
      "Quota, expiration and model access are set by the organization's policy."
    )
  ).toBeTruthy()
})

test('a member creating a key submits only the name', async () => {
  const user = userEvent.setup()
  const createMock = vi.mocked(createOrganizationToken)
  createMock.mockResolvedValue({
    success: true,
    data: { ...CREATED_KEY, remain_quota: 20, expired_time: -1 },
  })

  renderDialog(null, false)
  await user.type(screen.getByLabelText('Name'), 'ci key')
  await user.click(screen.getByRole('button', { name: 'Create' }))

  await waitFor(() =>
    expect(createMock).toHaveBeenCalledWith(ORGANIZATION_ID, { name: 'ci key' })
  )
  expect(await screen.findByText('Never expires')).toBeTruthy()
})

test('a member editing a key sees the stored limits read-only', () => {
  renderDialog(EDITED_KEY, false)

  expect(screen.getByLabelText('Name')).toBeTruthy()
  expect(screen.queryByText('Unlimited Quota')).toBeNull()
  expect(screen.getByText('Quota')).toBeTruthy()
  expect(screen.getByText('Never expires')).toBeTruthy()
})

test('a member can rename a legacy key that already expired', async () => {
  const user = userEvent.setup()
  const updateMock = vi.mocked(updateOrganizationToken)
  updateMock.mockResolvedValue({
    success: true,
    data: { ...EXPIRED_KEY, name: 'renamed' },
  })

  renderDialog(EXPIRED_KEY, false)
  const nameInput = screen.getByLabelText('Name')
  await user.clear(nameInput)
  await user.type(nameInput, 'renamed')
  await user.click(screen.getByRole('button', { name: 'Save changes' }))

  await waitFor(() => expect(updateMock).toHaveBeenCalled())
})

test('an admin still sees every configurable field', () => {
  renderDialog(null, true)

  expect(screen.getByText('Create count')).toBeTruthy()
  expect(screen.getByText('Unlimited Quota')).toBeTruthy()
  expect(screen.getByText('Expiration Time')).toBeTruthy()
  expect(screen.getByText('Model Limits')).toBeTruthy()
  expect(screen.getByText('IP limits')).toBeTruthy()
})

test('a platform administrator in workspace keeps the full form', () => {
  renderDialog(null, false, true)

  expect(screen.getByText('Create count')).toBeTruthy()
  expect(screen.getByText('Unlimited Quota')).toBeTruthy()
  expect(screen.getByText('Expiration Time')).toBeTruthy()
  expect(screen.getByText('Model Limits')).toBeTruthy()
  expect(screen.getByText('IP limits')).toBeTruthy()
})
