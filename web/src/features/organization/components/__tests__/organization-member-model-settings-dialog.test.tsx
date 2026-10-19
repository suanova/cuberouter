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

import { api } from '@/lib/api'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToEditableAmount,
} from '@/lib/format'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import {
  getOrganizationMemberModelAccess,
  getOrganizationMemberModelDiscounts,
  getOrganizationMemberQuota,
  listOrganizationMembers,
  updateOrganizationMemberModelAccess,
  updateOrganizationMemberModelDiscounts,
  updateOrganizationMemberQuota,
} from '../../api'
import type {
  MemberModelAccess,
  MemberQuota,
  OrganizationMemberRow,
  UserModelDiscounts,
} from '../../types'
import { OrganizationMembersSection } from '../organization-members-section'

/**
 * The members table's way into a member's settings: the one entry behind which
 * the model allow-list, the per-model discounts and the cumulative quota limit
 * are edited and saved together.
 *
 * What this file pins is the gate (any one capability opens the entry, none
 * hides it), the reads the dialog waits for, the link between the allow-list
 * and the discount rows it frames, and the payload rules that are easy to get
 * wrong: an empty allow-list is "no restriction" rather than "no models", the
 * discount record travels out whole, and the quota block converts the dollar
 * amount the operator types back into quota units — with an emptied input
 * meaning "no limit" rather than a limit of nothing.
 */
const { toastSuccess } = vi.hoisted(() => ({ toastSuccess: vi.fn() }))

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: toastSuccess },
}))

// The URL is the page's state, not this section's; the harness supplies a fixed
// empty set of search params instead of standing up a route. Both values are
// one stable object — a fresh `search` per render would re-enter the pagination
// effect on every pass.
const pageRoute = vi.hoisted(() => ({ search: {}, navigate: vi.fn() }))

vi.mock(
  '@/features/organization/components/organization-page-provider',
  async (importOriginal) => ({
    ...(await importOriginal<
      typeof import('@/features/organization/components/organization-page-provider')
    >()),
    useOrganizationSectionRoute: () => pageRoute,
  })
)

// The discount editor's model options come from the pricing list, which has
// nothing to say about this section.
vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({ models: [] }),
}))

// Only the section's endpoints are stubbed; the request shapes belong to the
// api module, which runs for real.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  listOrganizationMembers: vi.fn(),
  getOrganizationMemberModelAccess: vi.fn(),
  getOrganizationMemberModelDiscounts: vi.fn(),
  getOrganizationMemberQuota: vi.fn(),
  updateOrganizationMemberModelAccess: vi.fn(),
  updateOrganizationMemberModelDiscounts: vi.fn(),
  updateOrganizationMemberQuota: vi.fn(),
}))

const listMembersMock = vi.mocked(listOrganizationMembers)
const getModelAccessMock = vi.mocked(getOrganizationMemberModelAccess)
const getModelDiscountsMock = vi.mocked(getOrganizationMemberModelDiscounts)
const getQuotaMock = vi.mocked(getOrganizationMemberQuota)
const updateModelAccessMock = vi.mocked(updateOrganizationMemberModelAccess)
const updateModelDiscountsMock = vi.mocked(
  updateOrganizationMemberModelDiscounts
)
const updateQuotaMock = vi.mocked(updateOrganizationMemberQuota)

const ORGANIZATION_ID = 7
const MEMBER_USER_ID = 9
const SECOND_MEMBER_USER_ID = 11
const ACTOR_USER_ID = 1
/** The site's models, which is what the access read answers with as candidates. */
const UNIVERSE = ['m1', 'm2', 'm3']
const STORED_ACCESS = ['m1']
/** m3 is not on the allow-list, so its entry is the hidden one. */
const STORED_DISCOUNTS = { m1: 0.5, m3: 0.9 }
const SECOND_MEMBER_ACCESS = ['m2']
const SECOND_MEMBER_DISCOUNTS = { m2: 0.4 }
const ACCESS_PLACEHOLDER = 'Select allowed models; leave empty for no limit'
/**
 * The stored quota record, in quota units. Against the default display
 * currency (500,000 units to $1) it reads as $0.20 used of a $0.50 limit,
 * with $18 of the $20 organization quota still allocatable.
 */
const STORED_QUOTA: MemberQuota = {
  limit: 250_000,
  used: 100_000,
  organization_quota: 10_000_000,
  allocated: 1_000_000,
  over_allocated: false,
}
const QUOTA_INPUT_NAME = 'Cumulative consumption limit'
const QUOTA_INVALID_AMOUNT_ERROR =
  'Enter a non-negative amount, or leave the field empty for no limit.'
const QUOTA_TOO_SMALL_ERROR =
  'The amount is too small to be a limit. Enter a larger amount or leave the field empty for no limit.'
const OVER_ALLOCATION_WARNING =
  "Member quota limits exceed the organization quota. Lower a member's limit or raise the organization quota before setting another."

function memberRow(
  overrides: Partial<OrganizationMemberRow> = {}
): OrganizationMemberRow {
  return {
    id: 3,
    organization_id: ORGANIZATION_ID,
    user_id: MEMBER_USER_ID,
    role: 'member',
    status: 'active',
    username: 'bob',
    display_name: 'Bob Member',
    email: 'bob@example.com',
    ...overrides,
  }
}

function secondMemberRow(): OrganizationMemberRow {
  return memberRow({
    id: 4,
    user_id: SECOND_MEMBER_USER_ID,
    username: 'carol',
    display_name: 'Carol Member',
    email: 'carol@example.com',
  })
}

function renderSection(
  canManageMemberModels: boolean,
  canManageMemberDiscounts: boolean,
  canManageMemberQuotas: boolean,
  queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
): void {
  render(
    <QueryClientProvider client={queryClient}>
      <OrganizationMembersSection
        organizationId={ORGANIZATION_ID}
        actorRole='owner'
        currentUserId={ACTOR_USER_ID}
        canView
        canManageMembers
        canAddMembersDirectly={false}
        canExitOrganization={false}
        canManageMemberDiscounts={canManageMemberDiscounts}
        canManageMemberModels={canManageMemberModels}
        canManageMemberQuotas={canManageMemberQuotas}
        isPlatformAdministrator={false}
        readOnly={false}
        onForbidden={() => undefined}
        onLeftOrganization={() => undefined}
      />
    </QueryClientProvider>
  )
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

/** Opens the menu of one member row the fixture returned. */
async function openRowMenu(rowIndex = 0): Promise<void> {
  const menus = await screen.findAllByRole('button', { name: 'Open menu' })
  await userEvent.click(menus[rowIndex])
}

/** Opens the merged member settings dialog of one member row in the fixture. */
async function openSettingsDialog(rowIndex = 0): Promise<void> {
  await openRowMenu(rowIndex)
  await userEvent.click(
    screen.getByRole('menuitem', { name: 'Model Settings' })
  )
  await screen.findByText('Member Settings')
}

/** The quota block's dollar limit input. */
function quotaLimitInput(): HTMLElement {
  return screen.getByRole('textbox', { name: QUOTA_INPUT_NAME })
}

/** The allow-list control's own input. */
function modelAccessInput(): HTMLElement {
  return screen.getByRole('combobox', { name: ACCESS_PLACEHOLDER })
}

/** The models the allow-list control holds, in the order it holds them. */
function selectedModels(): string[] {
  const chips = document.querySelectorAll(
    '[data-slot="combobox-chips"] [data-slot="combobox-chip"]'
  )
  return [...chips].map((chip) => chip.textContent?.trim() ?? '')
}

/**
 * Closes the allow-list popup once it is open: an open combobox hides the rest
 * of the page from assistive technology, so the dialog's own controls are not
 * reachable until it is dismissed.
 */
async function closeModelAccessList(): Promise<void> {
  if (modelAccessInput().getAttribute('aria-expanded') === 'true') {
    await userEvent.keyboard('{Escape}')
  }
}

/** Picks a model in the allow-list control, opening its list if needed. */
async function selectModel(model: string): Promise<void> {
  if (!screen.queryByRole('option', { name: model })) {
    await userEvent.click(modelAccessInput())
  }
  await userEvent.click(await screen.findByRole('option', { name: model }))
  await closeModelAccessList()
}

/** Drops the last picked model, the way a keyboard user does. */
async function removeLastModel(): Promise<void> {
  await userEvent.click(modelAccessInput())
  await userEvent.keyboard('{Backspace}')
  await closeModelAccessList()
}

/** The pinned discount row of one model, when the editor shows one. */
function queryLockedRow(model: string): HTMLElement | null {
  return document.querySelector<HTMLElement>(
    `[data-slot="model-discount-locked-row"][data-model="${model}"]`
  )
}

/** The percent input of one model's pinned discount row. */
function lockedRowInput(model: string): HTMLElement {
  const row = queryLockedRow(model)
  if (!row) throw new Error(`Expected a pinned discount row for ${model}`)
  return within(row).getByRole('spinbutton', { name: 'Discount (%)' })
}

beforeEach(() => {
  // The quota block is displayed in the configured currency; every case starts
  // from the site default so a display setting cannot leak between tests.
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  listMembersMock.mockReset().mockResolvedValue({
    success: true,
    data: { page: 1, page_size: 10, total: 1, items: [memberRow()] },
  })
  getModelAccessMock.mockReset().mockResolvedValue({
    access: STORED_ACCESS,
    models: UNIVERSE,
  })
  getModelDiscountsMock.mockReset().mockResolvedValue(STORED_DISCOUNTS)
  getQuotaMock.mockReset().mockResolvedValue(STORED_QUOTA)
  updateModelAccessMock.mockReset().mockResolvedValue(undefined)
  updateModelDiscountsMock.mockReset().mockResolvedValue(undefined)
  updateQuotaMock.mockReset().mockResolvedValue(undefined)
  toastSuccess.mockReset()
})

describe('organization member model settings entry', () => {
  test('offers the merged entry to a caller who may manage member models', async () => {
    renderSection(true, false, false)

    await openRowMenu()

    expect(
      await screen.findByRole('menuitem', { name: 'Model Settings' })
    ).toBeInTheDocument()
  })

  test('offers the merged entry to a caller who may manage member discounts', async () => {
    renderSection(false, true, false)

    await openRowMenu()

    expect(
      await screen.findByRole('menuitem', { name: 'Model Settings' })
    ).toBeInTheDocument()
  })

  test('leaves the entry out for a caller who may manage none of the member settings', async () => {
    renderSection(false, false, false)

    await openRowMenu()

    // The menu itself is still there — an owner may disable or remove this
    // member — so what is asserted is the missing item, not a missing menu.
    expect(
      screen.queryByRole('menuitem', { name: 'Model Settings' })
    ).not.toBeInTheDocument()
    expect(screen.getByRole('menuitem', { name: 'Remove' })).toBeInTheDocument()
  })

  test('opens on the stored allow-list and one pinned discount row per model', async () => {
    renderSection(true, true, true)

    await openSettingsDialog()

    expect(getModelAccessMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      'member'
    )
    expect(getModelDiscountsMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      'member'
    )
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    // The rows are exactly the accessible models: m1 with its stored 50%, and
    // no row at all for m3, whose entry the member cannot reach anyway.
    expect(lockedRowInput('m1')).toHaveValue(50)
    expect(queryLockedRow('m3')).toBeNull()
    // The set is the allow-list's answer, so there is nothing to add or remove.
    expect(
      screen.queryByRole('button', { name: 'Add model discount' })
    ).not.toBeInTheDocument()
  })

  test('follows the allow-list when a model is dropped and picked again', async () => {
    getModelAccessMock.mockResolvedValue({
      access: STORED_ACCESS,
      models: UNIVERSE,
    })
    getModelDiscountsMock.mockResolvedValue({})
    renderSection(true, true, true)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(['m1']))

    await selectModel('m2')
    await waitFor(() => expect(selectedModels()).toEqual(['m1', 'm2']))
    expect(lockedRowInput('m2')).toHaveValue(100)

    fireEvent.input(lockedRowInput('m2'), { target: { value: '60' } })

    // Dropping the model hides its row. The row coming back showing 60 does not
    // by itself prove the entry survived — the input prefers the text being
    // typed — so the entry is read off the save payload instead.
    await removeLastModel()
    await waitFor(() => expect(selectedModels()).toEqual(['m1']))
    expect(queryLockedRow('m2')).toBeNull()

    await selectModel('m2')
    await waitFor(() => expect(selectedModels()).toEqual(['m1', 'm2']))
    expect(lockedRowInput('m2')).toHaveValue(60)

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    // m1's row was never given a percent, so it has no entry to save.
    await waitFor(() =>
      expect(updateModelDiscountsMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        { m2: 0.6 },
        'member'
      )
    )
  })

  test('removes a discount when its percent is cleared', async () => {
    getModelAccessMock.mockResolvedValue({
      access: ['m1', 'm3'],
      models: UNIVERSE,
    })
    renderSection(true, true, true)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(['m1', 'm3']))
    expect(lockedRowInput('m1')).toHaveValue(50)

    // An emptied percent means "no discount": the entry is removed from the
    // record rather than stored as a percent of zero.
    fireEvent.input(lockedRowInput('m1'), { target: { value: '' } })
    expect(lockedRowInput('m1')).toHaveValue(null)

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelDiscountsMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        { m3: 0.9 },
        'member'
      )
    )
    expect(updateModelAccessMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      ['m1', 'm3'],
      'member'
    )
  })

  test('clears the restriction when the allow-list is emptied', async () => {
    getModelAccessMock.mockResolvedValue({
      access: SECOND_MEMBER_ACCESS,
      models: UNIVERSE,
    })
    getModelDiscountsMock.mockResolvedValue(SECOND_MEMBER_DISCOUNTS)
    renderSection(true, true, false)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(['m2']))

    // An empty selection is not "no models may be used" but "no restriction",
    // so the dialog has to be able to send one — and say so.
    expect(
      screen.getByText(
        'All models are available by default; select models to restrict this member. Clear the selection to remove the restriction.'
      )
    ).toBeInTheDocument()
    await removeLastModel()
    await waitFor(() => expect(selectedModels()).toEqual([]))

    // With the restriction gone the editor is free again — its stored rows stay
    // on screen, and so does the way to add another.
    expect(
      screen.getByRole('spinbutton', { name: 'Discount (%)' })
    ).toHaveValue(40)
    expect(
      screen.getByRole('button', { name: 'Add model discount' })
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelAccessMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        [],
        'member'
      )
    )
    expect(updateModelDiscountsMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      SECOND_MEMBER_DISCOUNTS,
      'member'
    )
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
  })

  test('saves the allow-list and the whole discount record on one click', async () => {
    renderSection(true, true, true)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    fireEvent.input(lockedRowInput('m1'), { target: { value: '80' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelAccessMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        STORED_ACCESS,
        'member'
      )
    )
    // m3 is not on screen — the member may not use it — yet its stored entry is
    // in the payload: the allow-list decides what is editable, not what is kept.
    expect(updateModelDiscountsMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      { m1: 0.8, m3: 0.9 },
      'member'
    )
    // The allow-list decides what the discount record may look like, so it goes
    // to the backend first.
    expect(updateModelAccessMock.mock.invocationCallOrder[0]).toBeLessThan(
      updateModelDiscountsMock.mock.invocationCallOrder[0]
    )
    // The quota field was never touched, so no quota write is sent: saving the
    // other records must not overwrite a concurrent admin's limit with the
    // value this dialog read when it opened.
    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
    await waitFor(() =>
      expect(screen.queryByText('Member Settings')).not.toBeInTheDocument()
    )
  })

  test('invalidates the token form allow-list entry after a save', async () => {
    // The key token form reads the same allow-list under a shorter key — the
    // account context is not part of it — and caches it for 10s. An exact-key
    // invalidation leaves that entry stale, so the shared prefix is refreshed.
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 10_000 } },
    })
    const tokenFormKey = [
      'organization-member-model-access',
      'member',
      ORGANIZATION_ID,
      MEMBER_USER_ID,
    ]
    queryClient.setQueryData(tokenFormKey, {
      access: STORED_ACCESS,
      models: UNIVERSE,
    })
    expect(queryClient.getQueryState(tokenFormKey)?.isInvalidated).toBe(false)

    renderSection(true, true, true, queryClient)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(updateModelAccessMock).toHaveBeenCalled())
    await waitFor(() =>
      expect(queryClient.getQueryState(tokenFormKey)?.isInvalidated).toBe(true)
    )
  })

  test('reopening after a save keeps the saved allow-list, not the pre-save one', async () => {
    // The app sets staleTime to 10s (main.tsx). Without refreshing the cache on
    // save, reopening the dialog within that window serves the pre-save record
    // and the next save writes it back over the change.
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 10_000 } },
    })
    renderSection(true, true, true, queryClient)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    // Narrow the allow-list from ['m1'] to ['m2'] and save. The server starts
    // answering with the saved list from now on, which is what a refetch after
    // the write returns.
    await removeLastModel()
    await selectModel('m2')
    getModelAccessMock.mockResolvedValue({
      access: ['m2'],
      models: UNIVERSE,
    })
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateModelAccessMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        ['m2'],
        'member'
      )
    )
    await waitFor(() =>
      expect(screen.queryByText('Member Settings')).not.toBeInTheDocument()
    )

    // Reopen within the stale window and edit only a discount.
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(['m2']))
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Save' })).toBeEnabled()
    )
    fireEvent.input(lockedRowInput('m2'), { target: { value: '80' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelAccessMock).toHaveBeenLastCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        ['m2'],
        'member'
      )
    )
  })

  test('keeps saving unavailable until both reads land', async () => {
    const access = deferred<MemberModelAccess>()
    const discounts = deferred<UserModelDiscounts>()
    getModelAccessMock.mockReturnValue(access.promise)
    getModelDiscountsMock.mockReturnValue(discounts.promise)
    renderSection(true, true, true)
    await openSettingsDialog()

    const saveButton = screen.getByRole('button', { name: 'Save' })
    expect(saveButton).toBeDisabled()

    access.resolve({ access: STORED_ACCESS, models: UNIVERSE })
    await waitFor(() => expect(modelAccessInput()).toBeInTheDocument())
    expect(saveButton).toBeDisabled()

    discounts.resolve(STORED_DISCOUNTS)
    await waitFor(() => expect(saveButton).toBeEnabled())
  })

  test('disables every control while the save is in flight', async () => {
    const discountsWrite = deferred<void>()
    updateModelDiscountsMock.mockReturnValue(discountsWrite.promise)
    renderSection(true, true, true)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    // A second click must not double-write, and the records on screen must not
    // drift from the ones being written.
    await waitFor(() =>
      expect(screen.getByRole('button', { name: 'Saving...' })).toBeDisabled()
    )
    expect(screen.getByRole('button', { name: 'Cancel' })).toBeDisabled()
    expect(modelAccessInput()).toBeDisabled()
    expect(lockedRowInput('m1')).toBeDisabled()
    expect(quotaLimitInput()).toBeDisabled()

    discountsWrite.resolve(undefined)
    await waitFor(() =>
      expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
    )
  })

  test('keeps saving unavailable when the allow-list read fails', async () => {
    getModelAccessMock.mockRejectedValue(new Error('member not active'))
    renderSection(true, true, true)
    await openSettingsDialog()

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  test('keeps saving unavailable when the discount read fails', async () => {
    getModelDiscountsMock.mockRejectedValue(new Error('member not active'))
    renderSection(true, true, true)
    await openSettingsDialog()

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  test('treats a read refused as HTTP 200 as a failure, not an empty record', async () => {
    const realOrganizationApi =
      await vi.importActual<typeof import('../../api')>('../../api')
    getModelAccessMock.mockImplementation(
      realOrganizationApi.getOrganizationMemberModelAccess
    )
    getModelDiscountsMock.mockImplementation(
      realOrganizationApi.getOrganizationMemberModelDiscounts
    )
    getQuotaMock.mockImplementation(
      realOrganizationApi.getOrganizationMemberQuota
    )
    // The server reports a refused read (member no longer active) as HTTP 200
    // with `success: false`; the api module is what turns that envelope into a
    // rejection. Defaulting it would present the member as unrestricted and
    // discount-free — and the full replacements this dialog saves would then
    // clear the records that were never read.
    vi.spyOn(api, 'get').mockResolvedValue({
      data: { success: false, message: 'member not active' },
    } as never)

    renderSection(true, true, true)
    await openSettingsDialog()

    await waitFor(() =>
      expect(screen.getAllByText('Failed to load')).toHaveLength(3)
    )
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })

  test('keeps a refused allow-list write from closing as successful', async () => {
    const realOrganizationApi =
      await vi.importActual<typeof import('../../api')>('../../api')
    updateModelAccessMock.mockImplementation(
      realOrganizationApi.updateOrganizationMemberModelAccess
    )
    // A business refusal is answered as HTTP 200 with `success: false`; the api
    // module is what turns that envelope into a rejection, so the dialog stays
    // open on the edit instead of closing as if it had been stored.
    vi.spyOn(api, 'put').mockResolvedValue({
      data: { success: false, message: 'model name is invalid' },
    } as never)

    renderSection(true, true, true)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    fireEvent.input(lockedRowInput('m1'), { target: { value: '80' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(api.put).toHaveBeenCalledWith(
        `/api/organizations/${ORGANIZATION_ID}/members/${MEMBER_USER_ID}/model-access`,
        { models: STORED_ACCESS }
      )
    )
    // The allow-list never landed, so the later writes were not sent either:
    // the writes share one operator intent.
    expect(updateModelDiscountsMock).not.toHaveBeenCalled()
    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Member Settings')).toBeInTheDocument()
    expect(lockedRowInput('m1')).toHaveValue(80)
  })

  test('keeps a refused discount write from closing as successful', async () => {
    updateModelDiscountsMock.mockRejectedValue(
      new Error('Discount factor out of range')
    )
    renderSection(true, true, true)
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelDiscountsMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        STORED_DISCOUNTS,
        'member'
      )
    )
    // The allow-list write was the first half and stays landed; what failed is
    // the dialog's job to report, and it stays open with the edit to retry.
    expect(updateModelAccessMock).toHaveBeenCalledTimes(1)
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Member Settings')).toBeInTheDocument()
  })

  test('renders only the allow-list for a caller who may not manage discounts', async () => {
    renderSection(true, false, false)
    await openSettingsDialog()

    expect(screen.getByText('Model Access')).toBeInTheDocument()
    expect(screen.queryByText('Model Discounts')).not.toBeInTheDocument()
    expect(getModelDiscountsMock).not.toHaveBeenCalled()

    await waitFor(() => expect(selectedModels()).toEqual(STORED_ACCESS))
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelAccessMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        STORED_ACCESS,
        'member'
      )
    )
    expect(updateModelDiscountsMock).not.toHaveBeenCalled()
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
  })

  test('renders only the free discount editor for a caller who may not manage models', async () => {
    renderSection(false, true, false)
    await openSettingsDialog()

    expect(screen.queryByText('Model Access')).not.toBeInTheDocument()
    expect(getModelAccessMock).not.toHaveBeenCalled()

    // Without the models grant nothing frames the rows, so the editor is the
    // one it always was: every stored entry, and the way to add another.
    expect(await screen.findByDisplayValue('50')).toBeInTheDocument()
    expect(screen.getByDisplayValue('90')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Add model discount' })
    ).toBeInTheDocument()

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateModelDiscountsMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        STORED_DISCOUNTS,
        'member'
      )
    )
    expect(updateModelAccessMock).not.toHaveBeenCalled()
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
  })

  test('opens the next member on their own settings, not the previous edit', async () => {
    listMembersMock.mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 10,
        total: 2,
        items: [memberRow(), secondMemberRow()],
      },
    })
    getModelAccessMock.mockImplementation(
      async (_organizationId: number, userId: number) =>
        userId === SECOND_MEMBER_USER_ID
          ? { access: SECOND_MEMBER_ACCESS, models: UNIVERSE }
          : { access: STORED_ACCESS, models: UNIVERSE }
    )
    getModelDiscountsMock.mockImplementation(
      async (_organizationId: number, userId: number) =>
        userId === SECOND_MEMBER_USER_ID
          ? SECOND_MEMBER_DISCOUNTS
          : STORED_DISCOUNTS
    )
    renderSection(true, true, true)

    // An edit is left behind in the first member's dialog, which is then closed
    // without saving — as an operator who changes their mind does.
    await openSettingsDialog()
    await waitFor(() => expect(selectedModels()).toEqual(['m1']))
    fireEvent.input(lockedRowInput('m1'), { target: { value: '80' } })
    fireEvent.input(quotaLimitInput(), { target: { value: '7' } })
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await openSettingsDialog(1)

    await waitFor(() => expect(selectedModels()).toEqual(['m2']))
    expect(lockedRowInput('m2')).toHaveValue(40)
    expect(quotaLimitInput()).toHaveValue(
      String(quotaUnitsToEditableAmount(STORED_QUOTA.limit))
    )
    expect(screen.queryByDisplayValue('80')).not.toBeInTheDocument()
    expect(updateModelAccessMock).not.toHaveBeenCalled()
    expect(updateModelDiscountsMock).not.toHaveBeenCalled()
    expect(updateQuotaMock).not.toHaveBeenCalled()
  })
})

describe('organization member quota block', () => {
  test('opens on the stored quota with its usage and allocatable remainder', async () => {
    renderSection(true, true, true)
    await openSettingsDialog()

    expect(getQuotaMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      MEMBER_USER_ID,
      'member'
    )
    expect(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME })
    ).toHaveValue(String(quotaUnitsToEditableAmount(STORED_QUOTA.limit)))
    expect(screen.getByText(formatQuota(STORED_QUOTA.used))).toBeInTheDocument()
    expect(
      screen.getByText(formatQuota(STORED_QUOTA.limit))
    ).toBeInTheDocument()
    expect(
      screen.getByText(
        formatQuota(STORED_QUOTA.organization_quota - STORED_QUOTA.allocated)
      )
    ).toBeInTheDocument()
    expect(screen.queryByText(OVER_ALLOCATION_WARNING)).not.toBeInTheDocument()
  })

  test('states the quota rule alongside the allow-list rule when both render', async () => {
    renderSection(true, true, true)
    await openSettingsDialog()

    // The allow-list sentence frames the discount rows and the quota sentence
    // frames the limit input; showing both blocks means stating both rules.
    expect(
      screen.getByText(
        'All models are available by default; select models to restrict this member. Clear the selection to remove the restriction. Set the total this member may consume inside the organization. Leave it empty for no limit.'
      )
    ).toBeInTheDocument()
  })

  test('explains the default quota for member-created keys', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    expect(
      screen.getByText(
        'Member-created keys use the remaining member quota, or 1% of the organization quota when no limit is set. A set member quota caps cumulative consumption across keys; the 1% default is a per-key cap.'
      )
    ).toBeInTheDocument()
  })

  test('saves a whole dollar amount as literal quota units and shows it unchanged', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    const input = await screen.findByRole('textbox', {
      name: QUOTA_INPUT_NAME,
    })
    fireEvent.change(input, { target: { value: '3' } })

    // The site default is 500,000 quota units to the dollar, so $3 is exactly
    // 1,500,000 units — a literal, not the production parser compared to itself.
    expect(input).toHaveValue('3')
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateQuotaMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        1_500_000,
        'member'
      )
    )
  })

  test('keeps a fractional amount while it is typed character by character', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    const input = await screen.findByRole('textbox', {
      name: QUOTA_INPUT_NAME,
    })

    // Typing "0.5" one change at a time: each event carries the field's text.
    // Converting to quota units between keystrokes would render "0" as an
    // empty field and turn the remaining "5" into a five-dollar limit.
    fireEvent.change(input, { target: { value: '0' } })
    expect(input).toHaveValue('0')
    fireEvent.change(input, { target: { value: '0.' } })
    fireEvent.change(input, { target: { value: '0.5' } })
    expect(input).toHaveValue('0.5')

    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateQuotaMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        parseQuotaFromDollars(0.5),
        'member'
      )
    )
  })

  test('initializes the input from the editable display amount', async () => {
    // A limit stored from a $3 entry is 205,479 units under CNY at 7.3. The
    // raw unit-to-amount conversion renders that as 2.9999934; the editable
    // amount is the 3 the operator typed.
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CNY',
          usdExchangeRate: 7.3,
        },
      },
    }))
    getQuotaMock.mockResolvedValue({
      ...STORED_QUOTA,
      limit: parseQuotaFromDollars(3),
    })
    renderSection(true, false, true)
    await openSettingsDialog()

    expect(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME })
    ).toHaveValue('3')
  })

  test('saves 0, meaning no limit, when the input is emptied', async () => {
    renderSection(true, true, true)
    await openSettingsDialog()

    fireEvent.input(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME }),
      { target: { value: '' } }
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() =>
      expect(updateQuotaMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        0,
        'member'
      )
    )
  })

  test('rejects a negative amount without saving or closing the dialog', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    // A negative amount must not be read as "no limit" by clamping it to zero:
    // the backend treats zero as an unlimited member, which is the opposite of
    // what the operator typed.
    fireEvent.change(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME }),
      { target: { value: '-3' } }
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      QUOTA_INVALID_AMOUNT_ERROR
    )
    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Member Settings')).toBeInTheDocument()
  })

  test('rejects a non-finite amount without saving or closing the dialog', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    // 1e999 parses to Infinity; converting it through the quota rounding would
    // overflow into a meaningless limit, so it is refused like a negative one.
    fireEvent.change(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME }),
      { target: { value: '1e999' } }
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      QUOTA_INVALID_AMOUNT_ERROR
    )
    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Member Settings')).toBeInTheDocument()
  })

  test('rejects a positive amount that rounds to zero quota units', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    // Under the default 500,000 units per dollar, a fraction of a unit rounds
    // to 0 — which again would silently clear the cap instead of setting it.
    fireEvent.change(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME }),
      { target: { value: '0.0000001' } }
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    expect(await screen.findByRole('alert')).toHaveTextContent(
      QUOTA_TOO_SMALL_ERROR
    )
    expect(updateQuotaMock).not.toHaveBeenCalled()
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Member Settings')).toBeInTheDocument()
  })

  test('clears the amount error when the input changes', async () => {
    renderSection(true, false, true)
    await openSettingsDialog()

    const input = await screen.findByRole('textbox', {
      name: QUOTA_INPUT_NAME,
    })
    fireEvent.change(input, { target: { value: '-3' } })
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    expect(await screen.findByRole('alert')).toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'true')

    // Correcting the field clears the complaint immediately; it belongs to the
    // value that was refused, not to the field forever.
    fireEvent.change(input, { target: { value: '3' } })
    expect(screen.queryByRole('alert')).not.toBeInTheDocument()
    expect(input).toHaveAttribute('aria-invalid', 'false')
  })

  test('keeps the dialog open when the quota write is refused', async () => {
    updateQuotaMock.mockRejectedValue(
      new Error('member quota limit would exceed the organization quota')
    )
    renderSection(true, true, true)
    await openSettingsDialog()

    fireEvent.input(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME }),
      { target: { value: '3' } }
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))

    await waitFor(() => expect(updateQuotaMock).toHaveBeenCalled())
    // The refusal is reported by the API client; the dialog's job is to stay
    // open on the edit so the operator can correct it and retry.
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Member Settings')).toBeInTheDocument()
    expect(quotaLimitInput()).toHaveValue('3')
  })

  test('reopening after a save keeps the saved quota', async () => {
    // The app sets staleTime to 10s (main.tsx); without publishing the saved
    // record to the cache, reopening within that window serves the pre-save
    // limit and the next save writes it back over the change.
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, staleTime: 10_000 } },
    })
    renderSection(true, true, true, queryClient)
    await openSettingsDialog()

    const savedLimit = parseQuotaFromDollars(3)
    getQuotaMock.mockResolvedValue({ ...STORED_QUOTA, limit: savedLimit })
    fireEvent.input(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME }),
      { target: { value: '3' } }
    )
    await userEvent.click(screen.getByRole('button', { name: 'Save' }))
    await waitFor(() =>
      expect(updateQuotaMock).toHaveBeenCalledWith(
        ORGANIZATION_ID,
        MEMBER_USER_ID,
        savedLimit,
        'member'
      )
    )
    await waitFor(() =>
      expect(screen.queryByText('Member Settings')).not.toBeInTheDocument()
    )

    await openSettingsDialog()
    expect(
      await screen.findByRole('textbox', { name: QUOTA_INPUT_NAME })
    ).toHaveValue('3')
  })

  test('leaves the quota block out when the caller may not manage quotas', async () => {
    renderSection(true, false, false)
    await openSettingsDialog()

    expect(screen.queryByText('Quota Limit')).not.toBeInTheDocument()
    expect(
      screen.queryByRole('textbox', { name: QUOTA_INPUT_NAME })
    ).not.toBeInTheDocument()
    expect(getQuotaMock).not.toHaveBeenCalled()

    // The entry itself still belongs to the allow-list grant the caller holds.
    expect(screen.getByText('Model Access')).toBeInTheDocument()
  })

  test('shows the over-allocation warning when member limits exceed the organization quota', async () => {
    getQuotaMock.mockResolvedValue({ ...STORED_QUOTA, over_allocated: true })
    renderSection(true, true, true)
    await openSettingsDialog()

    expect(await screen.findByText(OVER_ALLOCATION_WARNING)).toBeInTheDocument()
  })

  test('keeps saving unavailable until the quota read lands', async () => {
    const quota = deferred<MemberQuota>()
    getQuotaMock.mockReturnValue(quota.promise)
    renderSection(true, false, true)
    await openSettingsDialog()

    const saveButton = screen.getByRole('button', { name: 'Save' })
    expect(saveButton).toBeDisabled()

    quota.resolve(STORED_QUOTA)
    await waitFor(() => expect(saveButton).toBeEnabled())
  })

  test('keeps saving unavailable when the quota read fails', async () => {
    getQuotaMock.mockRejectedValue(new Error('member not active'))
    renderSection(true, false, true)
    await openSettingsDialog()

    expect(await screen.findByText('Failed to load')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Save' })).toBeDisabled()
  })
})
