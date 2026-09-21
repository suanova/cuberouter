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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { createOrganization } from '@/features/organization/api'
import type { Organization } from '@/features/organization/types'

import { PlatformOrganizationCreateDrawer } from '../platform-organization-create-drawer'

const { toastError } = vi.hoisted(() => ({ toastError: vi.fn() }))

vi.mock('sonner', () => ({
  toast: { error: toastError, success: vi.fn() },
}))

// The drawer only reaches for `createOrganization`; the rest of the module talks
// to the API client, which has nothing to say here.
vi.mock('@/features/organization/api', () => ({
  createOrganization: vi.fn(),
}))

const createOrganizationMock = vi.mocked(createOrganization)

function renderDrawer(props: {
  onCreated?: (organizationId: number) => void
  onOpenChange?: (open: boolean) => void
}) {
  const onCreated = props.onCreated ?? vi.fn()
  const onOpenChange = props.onOpenChange ?? vi.fn()

  const result = render(
    <PlatformOrganizationCreateDrawer
      open
      onOpenChange={onOpenChange}
      onCreated={onCreated}
    />
  )

  return { onCreated, onOpenChange, result }
}

function nameField(): HTMLInputElement {
  return screen.getByPlaceholderText('Enter organization name')
}

function submit(): void {
  const form = document.querySelector<HTMLFormElement>(
    '#platform-organization-create-form'
  )
  if (!form) throw new Error('Expected the create form')
  fireEvent.submit(form)
}

function typeName(name: string): void {
  fireEvent.input(nameField(), { target: { value: name } })
}

beforeEach(() => {
  createOrganizationMock.mockReset()
  toastError.mockReset()
})

describe('platform organization create drawer', () => {
  test('creates with only the fields the endpoint reads', async () => {
    createOrganizationMock.mockResolvedValue({
      success: true,
      data: { id: 42 } as Organization,
    })

    const { onCreated, onOpenChange } = renderDrawer({})
    typeName('  Acme Research  ')
    submit()

    await waitFor(() => expect(onCreated).toHaveBeenCalledWith(42))

    // No `group` and no `reason`: a new organization starts on the default group
    // and there is nothing to give a reason for. The backend ignores both, so
    // sending them would only be a claim that they were honoured.
    expect(createOrganizationMock).toHaveBeenCalledWith(
      { name: 'Acme Research', description: '' },
      { skipErrorHandler: true }
    )
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('reports the organization limit in place and stays open', async () => {
    createOrganizationMock.mockRejectedValue({
      response: {
        data: { success: false, code: 'organization_limit_exceeded' },
      },
    })

    const { onCreated, onOpenChange } = renderDrawer({})
    typeName('One Too Many')
    submit()

    await waitFor(() => expect(toastError).toHaveBeenCalled())

    // The code, not the transport's "Request failed with status code 409".
    expect(toastError).toHaveBeenCalledWith(
      'You have reached the maximum number of organizations you can create or join.'
    )
    expect(onCreated).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(nameField()).toBeInTheDocument()
  })

  test('refuses an empty name without calling the endpoint', async () => {
    renderDrawer({})
    typeName('   ')
    submit()

    await waitFor(() =>
      expect(screen.getByText('Name is required')).toBeInTheDocument()
    )

    expect(createOrganizationMock).not.toHaveBeenCalled()
  })
})
