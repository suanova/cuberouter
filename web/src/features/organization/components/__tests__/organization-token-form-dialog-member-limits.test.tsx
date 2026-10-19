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
import { act, render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, expect, test, vi } from 'vitest'

import { api } from '@/lib/api'

import {
  getOrganizationMemberModelAccess,
  updateOrganizationToken,
} from '../../api'
import type { OrganizationSurface } from '../../lib'
import type { OrganizationTokenRow } from '../../types'
import { OrganizationPageProvider } from '../organization-page-provider'
import { OrganizationTokenFormDialog } from '../organization-token-form-dialog'

vi.mock(
  '@/features/organization/hooks/use-organization-member-options',
  () => ({
    useOrganizationTokenResponsibleOptions: () => ({
      options: [
        { value: 42, label: 'alice' },
        { value: 43, label: 'bob' },
        { value: 44, label: 'carol' },
      ],
      isLoading: false,
    }),
  })
)

// Only the write and the member-access read are stubbed; the request shapes
// belong to the api module, which runs for real. `getUserModels` still goes
// through the axios instance and is answered by the `api.get` spy below.
vi.mock('../../api', async (importOriginal) => ({
  ...(await importOriginal<typeof import('../../api')>()),
  createOrganizationToken: vi.fn(),
  getOrganizationMemberModelAccess: vi.fn(),
  updateOrganizationToken: vi.fn(),
}))

const getMemberAccessMock = vi.mocked(getOrganizationMemberModelAccess)
const updateTokenMock = vi.mocked(updateOrganizationToken)

const ORGANIZATION_ID = 7
/** alice: the operator, with no allow-list of her own. */
const UNRESTRICTED_USER_ID = 42
/** bob: held to m1 and m2. */
const RESTRICTED_USER_ID = 43
/** carol: a second unrestricted member, for switching back. */
const SECOND_UNRESTRICTED_USER_ID = 44
/** What the operator's own `GET /api/user/models` answers. */
const OPERATOR_MODELS = ['m1', 'm2', 'm3']
/** bob's allow-list; m3 is the model he may not use. */
const RESTRICTED_ACCESS = ['m1', 'm2']
const MODEL_PLACEHOLDER = 'Select allowed models; leave empty for no limit'

type DialogOptions = {
  canManageAllTokens?: boolean
  surface?: OrganizationSurface
}

function dialogElement(
  queryClient: QueryClient,
  editing: OrganizationTokenRow | null,
  options: DialogOptions
) {
  return (
    <QueryClientProvider client={queryClient}>
      <OrganizationPageProvider
        surface={options.surface ?? 'member'}
        search={{}}
        navigate={() => undefined}
      >
        <OrganizationTokenFormDialog
          organizationId={ORGANIZATION_ID}
          open
          onOpenChange={() => undefined}
          editing={editing}
          canManageAllTokens={options.canManageAllTokens ?? true}
          currentUserId={UNRESTRICTED_USER_ID}
          isOrganizationMember
          groupOptions={[]}
          onSaved={() => undefined}
        />
      </OrganizationPageProvider>
    </QueryClientProvider>
  )
}

/** Renders the dialog; `rerender` swaps the key it edits without remounting. */
function renderDialog(
  editing: OrganizationTokenRow | null = null,
  options: DialogOptions = {}
): { rerender: (next: OrganizationTokenRow | null) => void } {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  const view = render(dialogElement(queryClient, editing, options))
  return {
    rerender: (next: OrganizationTokenRow | null) =>
      view.rerender(dialogElement(queryClient, next, options)),
  }
}

/** The form of an existing key whose stored limits are m1 and m3. */
function editedKey(): OrganizationTokenRow {
  return {
    id: 11,
    user_id: RESTRICTED_USER_ID,
    key: 'abcdefghijklmnop',
    status: 1,
    name: 'narrowed-key',
    created_time: 0,
    accessed_time: 0,
    expired_time: -1,
    remain_quota: 0,
    unlimited_quota: true,
    model_limits_enabled: true,
    model_limits: 'm1,m3',
    used_quota: 0,
    group: '',
    cross_group_retry: false,
    visibility: 'private',
    creator_user_id: 7,
    creator_username: 'creator-alice',
    responsible_user_id: RESTRICTED_USER_ID,
  }
}

/** Opens the member picker and chooses the member with this label. */
async function selectResponsibleMember(label: string): Promise<void> {
  await userEvent.click(
    screen.getByRole('combobox', { name: 'Responsible user' })
  )
  await userEvent.click(await screen.findByRole('option', { name: label }))
}

/** The model-limits control's own input. */
function modelInput(): HTMLElement {
  return screen.getByRole('combobox', { name: MODEL_PLACEHOLDER })
}

/** The models the limits control holds, in the order it holds them. */
function selectedModels(): string[] {
  const chips = document.querySelectorAll(
    '[data-slot="combobox-chips"] [data-slot="combobox-chip"]'
  )
  return [...chips].map((chip) => chip.textContent?.trim() ?? '')
}

/** Closes the model list once open, so its modal popup stops hiding the form. */
async function closeModelList(): Promise<void> {
  if (modelInput().getAttribute('aria-expanded') === 'true') {
    await userEvent.keyboard('{Escape}')
  }
}

/** Opens the model list and keeps it open for the assertions that follow. */
async function openModelList(): Promise<void> {
  await userEvent.click(modelInput())
}

/** Picks a model in the limits control, opening its list if needed. */
async function selectModel(model: string): Promise<void> {
  if (!screen.queryByRole('option', { name: model })) {
    await openModelList()
  }
  await userEvent.click(await screen.findByRole('option', { name: model }))
  await closeModelList()
}

beforeEach(() => {
  vi.spyOn(api, 'get').mockResolvedValue({
    data: { success: true, data: OPERATOR_MODELS },
  } as never)
  getMemberAccessMock.mockReset().mockResolvedValue({
    access: [],
    models: OPERATOR_MODELS,
  })
  updateTokenMock.mockReset().mockResolvedValue({ success: true })
})

test('offers only the models the selected responsible member may use', async () => {
  getMemberAccessMock.mockImplementation(async (_organizationId, userId) =>
    userId === RESTRICTED_USER_ID
      ? { access: RESTRICTED_ACCESS, models: OPERATOR_MODELS }
      : { access: [], models: OPERATOR_MODELS }
  )
  renderDialog()

  await selectResponsibleMember('bob')
  await openModelList()

  await waitFor(() =>
    expect(screen.queryByRole('option', { name: 'm3' })).not.toBeInTheDocument()
  )
  expect(screen.getByRole('option', { name: 'm1' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm2' })).toBeInTheDocument()
})

test('keeps the operator model list for a member with no allow-list', async () => {
  getMemberAccessMock.mockResolvedValue({
    access: [],
    models: OPERATOR_MODELS,
  })
  renderDialog()

  await waitFor(() =>
    expect(getMemberAccessMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      UNRESTRICTED_USER_ID,
      'member'
    )
  )
  await openModelList()

  expect(await screen.findByRole('option', { name: 'm1' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm2' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm3' })).toBeInTheDocument()
})

test('falls back to the operator model list when the access read fails', async () => {
  getMemberAccessMock.mockRejectedValue(new Error('member not active'))
  renderDialog()

  await waitFor(() => expect(getMemberAccessMock).toHaveBeenCalled())
  await openModelList()

  expect(screen.getByRole('option', { name: 'm1' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm2' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm3' })).toBeInTheDocument()
  expect(screen.getByText('Create organization key')).toBeInTheDocument()
})

test('trims the picked models when the holder has a narrower allow-list', async () => {
  getMemberAccessMock.mockImplementation(async (_organizationId, userId) =>
    userId === RESTRICTED_USER_ID
      ? { access: RESTRICTED_ACCESS, models: OPERATOR_MODELS }
      : { access: [], models: OPERATOR_MODELS }
  )
  renderDialog()

  await selectModel('m1')
  await selectModel('m3')
  await waitFor(() => expect(selectedModels()).toEqual(['m1', 'm3']))

  await selectResponsibleMember('bob')

  await waitFor(() => expect(selectedModels()).toEqual(['m1']))
})

test('crops the stored limits of an edited key and saves the intersection', async () => {
  getMemberAccessMock.mockResolvedValue({
    access: RESTRICTED_ACCESS,
    models: OPERATOR_MODELS,
  })
  const key = editedKey()
  renderDialog(key)

  await waitFor(() => expect(selectedModels()).toEqual(['m1']))

  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  await waitFor(() =>
    expect(updateTokenMock).toHaveBeenCalledWith(
      'member',
      ORGANIZATION_ID,
      key.id,
      expect.objectContaining({ model_limits: 'm1' })
    )
  )
})

test('keeps the stored limits and reports them when none belong to the holder', async () => {
  // bob's allow-list admits neither of the key's stored limits. Trimming the
  // selection to nothing would save the key as unrestricted, silently widening
  // it, so the selection is kept and the operator is told why it cannot be
  // narrowed.
  getMemberAccessMock.mockImplementation(async (_organizationId, userId) =>
    userId === RESTRICTED_USER_ID
      ? { access: ['m2'], models: OPERATOR_MODELS }
      : { access: [], models: OPERATOR_MODELS }
  )
  const key = editedKey()
  renderDialog(key)

  expect(
    await screen.findByText(
      'None of the selected models are available to this member'
    )
  ).toBeInTheDocument()
  expect(selectedModels()).toEqual(['m1', 'm3'])

  const nameInput = screen.getByRole('textbox', { name: 'Name' })
  await userEvent.clear(nameInput)
  await userEvent.type(nameInput, 'renamed-key')
  await userEvent.click(screen.getByRole('button', { name: 'Save changes' }))

  // The stored selection survives the save, so the key stays restricted rather
  // than being written back as "no limit".
  await waitFor(() =>
    expect(updateTokenMock).toHaveBeenCalledWith(
      'member',
      ORGANIZATION_ID,
      key.id,
      expect.objectContaining({
        name: 'renamed-key',
        model_limits_enabled: true,
        model_limits: 'm1,m3',
      })
    )
  )
})

test('keeps the current selection when switching to an unrestricted member', async () => {
  getMemberAccessMock.mockImplementation(async (_organizationId, userId) =>
    userId === RESTRICTED_USER_ID
      ? { access: ['m1', 'm4'], models: [...OPERATOR_MODELS, 'm4'] }
      : { access: [], models: OPERATOR_MODELS }
  )
  // m4 is bob's to use but is not in the operator's own model list, so a trim
  // to the fallback candidates on the way to carol would drop it.
  renderDialog({ ...editedKey(), model_limits: 'm1,m4' })

  await waitFor(() => expect(selectedModels()).toEqual(['m1', 'm4']))

  await selectResponsibleMember('carol')
  await waitFor(() =>
    expect(getMemberAccessMock).toHaveBeenCalledWith(
      ORGANIZATION_ID,
      SECOND_UNRESTRICTED_USER_ID,
      'member'
    )
  )
  // Let carol's unrestricted answer land before reading the selection, so a
  // late trim would be caught rather than raced past.
  await act(async () => undefined)

  expect(selectedModels()).toEqual(['m1', 'm4'])
})

test('opens a second key on its own stored limits, not the previous edit', async () => {
  getMemberAccessMock.mockResolvedValue({
    access: RESTRICTED_ACCESS,
    models: OPERATOR_MODELS,
  })
  const view = renderDialog({ ...editedKey(), model_limits: 'm1' })
  await waitFor(() => expect(selectedModels()).toEqual(['m1']))

  view.rerender({ ...editedKey(), id: 12, model_limits: 'm2' })

  await waitFor(() => expect(selectedModels()).toEqual(['m2']))
})

test('does not read a member allow-list or expose model limits for a member editing their own key', async () => {
  getMemberAccessMock.mockResolvedValue({
    access: RESTRICTED_ACCESS,
    models: OPERATOR_MODELS,
  })
  renderDialog(editedKey(), { canManageAllTokens: false })

  // A member has no administrator's view of a member allow-list and cannot
  // change the key's model limits; the stored value is shown read-only.
  expect(
    screen.queryByRole('combobox', { name: MODEL_PLACEHOLDER })
  ).not.toBeInTheDocument()
  expect(screen.getByText('m1,m3')).toBeInTheDocument()

  await act(async () => undefined)
  expect(getMemberAccessMock).not.toHaveBeenCalled()
})

test('keeps the full model list on the platform administrator surface', async () => {
  getMemberAccessMock.mockResolvedValue({
    access: RESTRICTED_ACCESS,
    models: OPERATOR_MODELS,
  })
  renderDialog(editedKey(), { surface: 'admin' })

  await openModelList()

  // The member-access route exists on the member surface only, so an
  // administrator reads no allow-list and the picker keeps every model the
  // operator can see; the backend validates the save against the holder.
  expect(await screen.findByRole('option', { name: 'm3' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm1' })).toBeInTheDocument()
  expect(screen.getByRole('option', { name: 'm2' })).toBeInTheDocument()
  expect(selectedModels()).toEqual(['m1', 'm3'])

  await act(async () => undefined)
  expect(getMemberAccessMock).not.toHaveBeenCalled()
})
