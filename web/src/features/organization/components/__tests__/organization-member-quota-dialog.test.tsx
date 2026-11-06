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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import {
  getOrganizationMemberQuota,
  updateOrganizationMemberQuota,
} from '../../api'
import type { MemberQuota, OrganizationMemberRow } from '../../types'
import { OrganizationMemberQuotaDialog } from '../organization-member-quota-dialog'

const { toastSuccess } = vi.hoisted(() => ({ toastSuccess: vi.fn() }))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: toastSuccess },
}))

vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  getOrganizationMemberQuota: vi.fn(),
  updateOrganizationMemberQuota: vi.fn(),
}))

const getQuotaMock = vi.mocked(getOrganizationMemberQuota)
const updateQuotaMock = vi.mocked(updateOrganizationMemberQuota)

const ORGANIZATION_ID = 7
const MEMBER_USER_ID = 9
const QUOTA_INPUT_NAME = 'Amount to add'
const QUOTA_INVALID_AMOUNT_ERROR = 'Enter an amount greater than zero.'
const QUOTA_TOO_SMALL_ERROR =
  'The amount is too small to add. Enter a larger amount.'
const OVER_ALLOCATION_WARNING =
  "Member quota limits exceed the organization quota. Lower a member's limit or raise the organization quota before setting another."
const STORED_QUOTA: MemberQuota = {
  limit: 250_000,
  used: 100_000,
  organization_quota: 10_000_000,
  allocated: 1_000_000,
  over_allocated: false,
}

function memberRow(): OrganizationMemberRow {
  return {
    id: 3,
    organization_id: ORGANIZATION_ID,
    user_id: MEMBER_USER_ID,
    role: 'member',
    status: 'active',
    username: 'bob',
    display_name: 'Bob Member',
    email: 'bob@example.com',
  }
}

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((promiseResolve, promiseReject) => {
    resolve = promiseResolve
    reject = promiseReject
  })
  return { promise, reject, resolve }
}

async function renderDialog(): Promise<{ onOpenChange: ReturnType<typeof vi.fn> }> {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const onOpenChange = vi.fn()
  render(
    <QueryClientProvider client={queryClient}>
      <OrganizationMemberQuotaDialog
        organizationId={ORGANIZATION_ID}
        member={memberRow()}
        onOpenChange={onOpenChange}
      />
    </QueryClientProvider>
  )
  await screen.findByRole('heading', { name: 'Add quota' })
  await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME })
  return { onOpenChange }
}

function quotaInput(): HTMLElement {
  return screen.getByRole('textbox', { name: QUOTA_INPUT_NAME })
}

function addButton(): HTMLElement {
  return screen.getByRole('button', { name: 'Add' })
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  getQuotaMock.mockReset().mockResolvedValue(STORED_QUOTA)
  updateQuotaMock.mockReset().mockResolvedValue(undefined)
  toastSuccess.mockReset()
})

describe('organization member quota dialog', () => {
  test('opens on the stored limit with its usage and allocatable remainder', async () => {
    await renderDialog()

    expect(getQuotaMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      'member'
    )
    expect(quotaInput()).toHaveValue('')
    expect(
      screen.getAllByText(formatQuota(STORED_QUOTA.limit)).length
    ).toBeGreaterThan(0)
    expect(screen.getByText(formatQuota(STORED_QUOTA.used))).toBeInTheDocument()
    expect(
      screen.getByText(
        formatQuota(STORED_QUOTA.organization_quota - STORED_QUOTA.allocated)
      )
    ).toBeInTheDocument()
    expect(screen.getByText('Limit after adding')).toBeInTheDocument()
    expect(
      screen.getByText(
        'Member-created keys use the remaining member quota, or 1% of the organization quota when no limit is set. A set member quota caps cumulative consumption across keys; the 1% default is a per-key cap.'
      )
    ).toBeInTheDocument()
  })

  test('keeps Add disabled until an amount is entered', async () => {
    await renderDialog()

    expect(addButton()).toBeDisabled()
    fireEvent.input(quotaInput(), { target: { value: '1' } })
    expect(addButton()).toBeEnabled()
  })

  test('adds to the stored limit and previews the resulting ceiling', async () => {
    const { onOpenChange } = await renderDialog()

    fireEvent.input(quotaInput(), { target: { value: '3' } })
    expect(
      screen.getByText(formatQuota(STORED_QUOTA.limit + parseQuotaFromDollars(3)))
    ).toBeInTheDocument()
    await userEvent.click(addButton())

    await waitFor(() =>
      expect(updateQuotaMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        STORED_QUOTA.limit + parseQuotaFromDollars(3),
        'member'
      )
    )
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('starts an unlimited member at the amount added', async () => {
    getQuotaMock.mockResolvedValue({ ...STORED_QUOTA, limit: 0 })
    await renderDialog()

    fireEvent.input(quotaInput(), { target: { value: '100' } })
    expect(
      screen.getByText(formatQuota(parseQuotaFromDollars(100)))
    ).toBeInTheDocument()
    await userEvent.click(addButton())

    await waitFor(() =>
      expect(updateQuotaMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        parseQuotaFromDollars(100),
        'member'
      )
    )
  })

  test('rejects a non-positive or non-finite amount without saving', async () => {
    const { onOpenChange } = await renderDialog()

    for (const value of ['0', '-1', 'not-a-number']) {
      fireEvent.input(quotaInput(), { target: { value } })
      await userEvent.click(addButton())
      expect(await screen.findByRole('alert')).toHaveTextContent(
        QUOTA_INVALID_AMOUNT_ERROR
      )
    }
    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalled()
  })

  test('rejects a positive amount that rounds to zero quota units', async () => {
    await renderDialog()

    fireEvent.input(quotaInput(), { target: { value: '0.0000001' } })
    await userEvent.click(addButton())

    expect(await screen.findByRole('alert')).toHaveTextContent(
      QUOTA_TOO_SMALL_ERROR
    )
    expect(updateQuotaMock).not.toHaveBeenCalled()
  })

  test('clears the amount error when the input changes', async () => {
    await renderDialog()

    fireEvent.input(quotaInput(), { target: { value: '-1' } })
    await userEvent.click(addButton())
    expect(await screen.findByRole('alert')).toBeInTheDocument()

    fireEvent.input(quotaInput(), { target: { value: '5' } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
  })

  test('keeps the dialog open when the write is refused', async () => {
    const { onOpenChange } = await renderDialog()
    updateQuotaMock.mockRejectedValue(new Error('limit exceeds quota'))

    fireEvent.input(quotaInput(), { target: { value: '3' } })
    await userEvent.click(addButton())

    await waitFor(() => expect(updateQuotaMock).toHaveBeenCalled())
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalled()
    expect(screen.getByRole('heading', { name: 'Add quota' })).toBeInTheDocument()
  })

  test('shows the over-allocation warning when member limits exceed the organization quota', async () => {
    getQuotaMock.mockResolvedValue({
      ...STORED_QUOTA,
      over_allocated: true,
    })
    await renderDialog()

    expect(screen.getByText(OVER_ALLOCATION_WARNING)).toBeInTheDocument()
  })

  test('keeps Add disabled until the quota read lands', async () => {
    const read = deferred<MemberQuota>()
    getQuotaMock.mockReturnValue(read.promise)

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <OrganizationMemberQuotaDialog
          organizationId={ORGANIZATION_ID}
          member={memberRow()}
          onOpenChange={() => undefined}
        />
      </QueryClientProvider>
    )

    expect(addButton()).toBeDisabled()

    read.resolve(STORED_QUOTA)
    await waitFor(() =>
      expect(
        screen.getByRole('textbox', { name: QUOTA_INPUT_NAME })
      ).toBeInTheDocument()
    )
    expect(addButton()).toBeDisabled()
    fireEvent.input(quotaInput(), { target: { value: '1' } })
    expect(addButton()).toBeEnabled()
  })

  test('keeps Add disabled when the quota read fails', async () => {
    getQuotaMock.mockRejectedValue(new Error('member not active'))
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    })
    render(
      <QueryClientProvider client={queryClient}>
        <OrganizationMemberQuotaDialog
          organizationId={ORGANIZATION_ID}
          member={memberRow()}
          onOpenChange={() => undefined}
        />
      </QueryClientProvider>
    )

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(addButton()).toBeDisabled()
  })

  test('Cancel closes without writing', async () => {
    const { onOpenChange } = await renderDialog()

    fireEvent.input(quotaInput(), { target: { value: '3' } })
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })
})
