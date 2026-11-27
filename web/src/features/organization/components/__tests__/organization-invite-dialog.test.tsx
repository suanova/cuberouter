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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import {
  createOrganizationInvite,
  getOrganizationMemberRestrictionOptions,
} from '../../api'
import type { MemberRestrictionOptions } from '../../types'
import { OrganizationInviteDialog } from '../organization-invite-dialog'

const { toastSuccess, toastError } = vi.hoisted(() => ({
  toastSuccess: vi.fn(),
  toastError: vi.fn(),
}))

vi.mock('sonner', () => ({
  toast: { error: toastError, success: toastSuccess },
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  createOrganizationInvite: vi.fn(),
  getOrganizationMemberRestrictionOptions: vi.fn(),
}))

const createInviteMock = vi.mocked(createOrganizationInvite)
const getOptionsMock = vi.mocked(getOrganizationMemberRestrictionOptions)

const ORGANIZATION_ID = 7
const MODEL_PLACEHOLDER = 'Select allowed models; leave empty for no limit'
const OVER_ALLOCATABLE_ERROR =
  'The quota limit exceeds the remaining allocatable quota.'

function restrictionOptions(): MemberRestrictionOptions {
  return {
    models: ['m1', 'm2', 'm3'],
    organization_quota: 50_000_000,
    allocated: 0,
    over_allocated: false,
  }
}

type CapabilityOverrides = {
  models?: boolean
  quotas?: boolean
}

function renderDialog(capabilities: CapabilityOverrides = {}): {
  onOpenChange: ReturnType<typeof vi.fn>
  onCreated: ReturnType<typeof vi.fn>
} {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onOpenChange = vi.fn()
  const onCreated = vi.fn()
  render(
    <QueryClientProvider client={queryClient}>
      <OrganizationInviteDialog
        organizationId={ORGANIZATION_ID}
        open
        onOpenChange={onOpenChange}
        onCreated={onCreated}
        canManageMemberModels={capabilities.models ?? true}
        canManageMemberQuotas={capabilities.quotas ?? true}
      />
    </QueryClientProvider>
  )
  return { onOpenChange, onCreated }
}

function fillEmail(): void {
  fireEvent.input(screen.getByRole('textbox', { name: 'Email' }), {
    target: { value: 'a@example.com' },
  })
}

function submitInvite(): Promise<void> {
  return userEvent.click(
    screen.getByRole('button', { name: 'Send invitation' })
  )
}

function restrictionsTrigger(): HTMLElement {
  return screen.getByRole('button', {
    name: /Model and quota restrictions/,
  })
}

function quotaInput(): HTMLElement {
  return screen.getByRole('textbox', { name: 'Quota limit' })
}

function modelAccessInput(): HTMLElement {
  return screen.getByRole('combobox', { name: MODEL_PLACEHOLDER })
}

function selectedModels(): string[] {
  const chips = document.querySelectorAll(
    '[data-slot="combobox-chips"] [data-slot="combobox-chip"]'
  )
  return [...chips].map((chip) => chip.textContent?.trim() ?? '')
}

async function closeModelList(): Promise<void> {
  if (modelAccessInput().getAttribute('aria-expanded') === 'true') {
    await userEvent.keyboard('{Escape}')
  }
}

async function selectModel(model: string): Promise<void> {
  if (!screen.queryByRole('option', { name: model })) {
    await userEvent.click(modelAccessInput())
  }
  await userEvent.click(await screen.findByRole('option', { name: model }))
  await closeModelList()
}

async function openRestrictions(): Promise<void> {
  await userEvent.click(restrictionsTrigger())
  await screen.findByRole('combobox', { name: MODEL_PLACEHOLDER })
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  createInviteMock.mockReset().mockResolvedValue({ success: true })
  getOptionsMock.mockReset().mockResolvedValue(restrictionOptions())
  toastSuccess.mockReset()
  toastError.mockReset()
})

describe('organization invite dialog member restrictions', () => {
  test('sends explicit empty restrictions by default', async () => {
    renderDialog()

    fillEmail()
    await submitInvite()

    await waitFor(() =>
      expect(createInviteMock).toHaveBeenCalledWith(ORGANIZATION_ID, {
        email: 'a@example.com',
        role: 'member',
        force_rotate: false,
        models: [],
        quota_limit: 0,
      })
    )
  })

  test('sends selected models and the converted quota', async () => {
    renderDialog()
    fillEmail()
    await openRestrictions()

    await selectModel('m2')
    await waitFor(() => expect(selectedModels()).toEqual(['m2']))
    fireEvent.input(quotaInput(), { target: { value: '10' } })

    await submitInvite()

    await waitFor(() =>
      expect(createInviteMock).toHaveBeenCalledWith(ORGANIZATION_ID, {
        email: 'a@example.com',
        role: 'member',
        force_rotate: false,
        models: ['m2'],
        quota_limit: 5_000_000,
      })
    )
  })

  test('blocks a quota above the remaining allocatable amount', async () => {
    renderDialog()
    fillEmail()
    await openRestrictions()
    await screen.findByText(/Remaining allocatable/)

    fireEvent.input(quotaInput(), { target: { value: '200' } })
    await submitInvite()

    expect(await screen.findByText(OVER_ALLOCATABLE_ERROR)).toBeInTheDocument()
    expect(createInviteMock).not.toHaveBeenCalled()
  })

  test('clears the quota error as soon as the amount changes', async () => {
    renderDialog()
    fillEmail()
    await openRestrictions()
    await screen.findByText(/Remaining allocatable/)

    fireEvent.input(quotaInput(), { target: { value: '200' } })
    await submitInvite()
    expect(await screen.findByText(OVER_ALLOCATABLE_ERROR)).toBeInTheDocument()

    fireEvent.input(quotaInput(), { target: { value: '10' } })

    expect(screen.queryByText(OVER_ALLOCATABLE_ERROR)).toBeNull()
    expect(quotaInput()).toHaveAttribute('aria-invalid', 'false')
  })

  test('resets the form when the parent reopens the dialog', async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    const onOpenChange = vi.fn()
    const renderTree = (open: boolean) => (
      <QueryClientProvider client={queryClient}>
        <OrganizationInviteDialog
          organizationId={ORGANIZATION_ID}
          open={open}
          onOpenChange={onOpenChange}
          onCreated={() => undefined}
          canManageMemberModels
          canManageMemberQuotas
        />
      </QueryClientProvider>
    )
    const { rerender } = render(renderTree(true))
    fillEmail()
    await openRestrictions()
    await selectModel('m2')
    await waitFor(() => expect(selectedModels()).toEqual(['m2']))
    fireEvent.input(quotaInput(), { target: { value: '10' } })

    rerender(renderTree(false))
    rerender(renderTree(true))

    const email = await screen.findByRole('textbox', { name: 'Email' })
    expect(email).toHaveValue('')
    if (!screen.queryByRole('textbox', { name: 'Quota limit' })) {
      await openRestrictions()
    }
    expect(quotaInput()).toHaveValue('')
    expect(selectedModels()).toEqual([])
  })

  test('omits restriction fields for a caller without either capability', async () => {
    renderDialog({ models: false, quotas: false })

    expect(
      screen.queryByRole('button', { name: /Model and quota restrictions/ })
    ).toBeNull()

    fillEmail()
    await submitInvite()

    await waitFor(() =>
      expect(createInviteMock).toHaveBeenCalledWith(ORGANIZATION_ID, {
        email: 'a@example.com',
        role: 'member',
        force_rotate: false,
      })
    )
    expect(getOptionsMock).not.toHaveBeenCalled()
  })

  test('resends with the same restrictions after confirming', async () => {
    createInviteMock.mockRejectedValueOnce({
      response: { data: { code: 'organization_invite_already_sent' } },
    })
    renderDialog()
    fillEmail()
    await openRestrictions()
    await selectModel('m2')
    await waitFor(() => expect(selectedModels()).toEqual(['m2']))
    fireEvent.input(quotaInput(), { target: { value: '10' } })

    await submitInvite()

    const confirm = await screen.findByRole('alertdialog')
    await userEvent.click(
      within(confirm).getByRole('button', { name: 'Send invitation' })
    )

    await waitFor(() =>
      expect(createInviteMock).toHaveBeenLastCalledWith(ORGANIZATION_ID, {
        email: 'a@example.com',
        role: 'member',
        force_rotate: true,
        models: ['m2'],
        quota_limit: 5_000_000,
      })
    )
  })
})
