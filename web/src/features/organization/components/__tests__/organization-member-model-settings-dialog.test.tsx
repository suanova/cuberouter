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
  getOrganizationMemberModelAccess,
  getOrganizationMemberModelDiscounts,
  listOrganizationMembers,
  updateOrganizationMemberModelAccess,
  updateOrganizationMemberModelDiscounts,
} from '../../api'
import type {
  MemberModelAccess,
  OrganizationMemberRow,
  UserModelDiscounts,
} from '../../types'
import { OrganizationMembersSection } from '../organization-members-section'

/**
 * The members table's way into a member's model settings: the one entry behind
 * which the model allow-list and the per-model discounts are edited and saved
 * together.
 *
 * What this file pins is the gate (either capability opens the entry, neither
 * hides it), the reads the dialog waits for, the link between the allow-list
 * and the discount rows it frames, and the payload rules that are easy to get
 * wrong: an empty allow-list is "no restriction" rather than "no models", and
 * the discount record travels out whole.
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
  updateOrganizationMemberModelAccess: vi.fn(),
  updateOrganizationMemberModelDiscounts: vi.fn(),
}))

const listMembersMock = vi.mocked(listOrganizationMembers)
const getModelAccessMock = vi.mocked(getOrganizationMemberModelAccess)
const getModelDiscountsMock = vi.mocked(getOrganizationMemberModelDiscounts)
const updateModelAccessMock = vi.mocked(updateOrganizationMemberModelAccess)
const updateModelDiscountsMock = vi.mocked(
  updateOrganizationMemberModelDiscounts
)

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
  await screen.findByRole('heading', { name: 'Model Settings' })
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
  listMembersMock.mockReset().mockResolvedValue({
    success: true,
    data: { page: 1, page_size: 10, total: 1, items: [memberRow()] },
  })
  getModelAccessMock.mockReset().mockResolvedValue({
    access: STORED_ACCESS,
    models: UNIVERSE,
  })
  getModelDiscountsMock.mockReset().mockResolvedValue(STORED_DISCOUNTS)
  updateModelAccessMock.mockReset().mockResolvedValue(undefined)
  updateModelDiscountsMock.mockReset().mockResolvedValue(undefined)
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
    expect(toastSuccess).toHaveBeenCalledWith('Operation successful')
    await waitFor(() =>
      expect(screen.queryByText('Model Settings')).not.toBeInTheDocument()
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
      expect(screen.queryByText('Model Settings')).not.toBeInTheDocument()
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
      expect(screen.getAllByText('Failed to load')).toHaveLength(2)
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
    expect(toastSuccess).not.toHaveBeenCalled()
    expect(screen.getByText('Model Settings')).toBeInTheDocument()
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
    expect(screen.getByText('Model Settings')).toBeInTheDocument()
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
    await userEvent.click(screen.getByRole('button', { name: 'Cancel' }))

    await openSettingsDialog(1)

    await waitFor(() => expect(selectedModels()).toEqual(['m2']))
    expect(lockedRowInput('m2')).toHaveValue(40)
    expect(screen.queryByDisplayValue('80')).not.toBeInTheDocument()
    expect(updateModelAccessMock).not.toHaveBeenCalled()
    expect(updateModelDiscountsMock).not.toHaveBeenCalled()
  })
})
