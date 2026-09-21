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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { describe, expect, test, vi } from 'vitest'

import { PlatformOrganizationCreateDrawer } from '../platform-organization-create-drawer'
import { PlatformOrganizationsPrimaryButtons } from '../platform-organizations-primary-buttons'
import {
  PlatformOrganizationsProvider,
  usePlatformOrganizations,
} from '../platform-organizations-provider'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('@/features/organization/api', () => ({
  createOrganization: vi.fn(),
}))

/**
 * The page's own wiring, reduced to the part under test: the button and the
 * drawer, sharing one provider. The page also mounts the table, which would drag
 * in the router and the platform list query without telling us anything about
 * whether the create entry opens the create drawer.
 */
function CreateEntry() {
  const { open, setOpen } = usePlatformOrganizations()

  return (
    <>
      <PlatformOrganizationsPrimaryButtons />
      <PlatformOrganizationCreateDrawer
        open={open === 'create'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        onCreated={() => undefined}
      />
    </>
  )
}

describe('platform organization create entry', () => {
  test('opens the create form', async () => {
    render(
      <PlatformOrganizationsProvider>
        <CreateEntry />
      </PlatformOrganizationsProvider>
    )

    const button = screen.getByRole('button', { name: 'Create Organization' })
    // The button is never disabled. Creating is bounded by a server-side count
    // the platform list cannot see, so the refusal arrives as a typed response
    // from the endpoint rather than as a pre-emptive greyed-out button.
    expect(button).toBeEnabled()
    expect(
      screen.queryByPlaceholderText('Enter organization name')
    ).not.toBeInTheDocument()

    await userEvent.click(button)

    expect(
      await screen.findByPlaceholderText('Enter organization name')
    ).toBeInTheDocument()
  })
})
