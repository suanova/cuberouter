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
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { afterEach, expect, test, vi } from 'vitest'

import { parseQuotaFromDollars, quotaUnitsToEditableAmount } from '@/lib/format'

import { updateOrganization } from '../../api'
import type { Organization } from '../../types'
import { OrganizationPageProvider } from '../organization-page-provider'
import { OrganizationSettingsSection } from '../organization-settings-section'

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  updateOrganization: vi.fn(),
}))

const ORGANIZATION: Organization = {
  id: 7,
  name: 'Design',
  slug: 'design',
  description: '',
  group: 'default',
  status: 'active',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  owner_user_id: 1,
  created_by: 1,
  created_at: 0,
  updated_at: 0,
  default_member_token_quota: 0,
}

function renderSection(organization: Organization = ORGANIZATION) {
  return render(
    <OrganizationPageProvider
      surface='member'
      search={{}}
      navigate={() => undefined}
    >
      <OrganizationSettingsSection
        organization={organization}
        canUpdate
        readOnly={false}
        onUpdated={() => undefined}
      />
    </OrganizationPageProvider>
  )
}

afterEach(() => {
  vi.restoreAllMocks()
})

test('shows the stored default key quota in display currency', () => {
  renderSection({
    ...ORGANIZATION,
    default_member_token_quota: parseQuotaFromDollars(10),
  })
  expect(screen.getByLabelText('Default key quota')).toHaveValue(
    String(quotaUnitsToEditableAmount(parseQuotaFromDollars(10)))
  )
})

test('saves the default key quota with the organization profile', async () => {
  const user = userEvent.setup()
  const updateMock = vi.mocked(updateOrganization)
  updateMock.mockResolvedValue({ success: true } as never)

  renderSection()
  await user.type(screen.getByLabelText('Default key quota'), '10')
  await user.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() =>
    expect(updateMock).toHaveBeenCalledWith('member', 7, {
      name: 'Design',
      description: '',
      default_member_token_quota: parseQuotaFromDollars(10),
    })
  )
})

test('an empty default key quota saves zero (unlimited)', async () => {
  const user = userEvent.setup()
  const updateMock = vi.mocked(updateOrganization)
  updateMock.mockResolvedValue({ success: true } as never)

  renderSection()
  await user.click(screen.getByRole('button', { name: 'Save' }))

  await waitFor(() =>
    expect(updateMock).toHaveBeenCalledWith('member', 7, {
      name: 'Design',
      description: '',
      default_member_token_quota: 0,
    })
  )
})

test('rejects a negative default key quota', async () => {
  const user = userEvent.setup()
  const updateMock = vi.mocked(updateOrganization)

  renderSection()
  await user.type(screen.getByLabelText('Default key quota'), '-1')
  await user.click(screen.getByRole('button', { name: 'Save' }))

  expect(await screen.findByText('Enter a non-negative amount.')).toBeTruthy()
  expect(updateMock).not.toHaveBeenCalled()
})
