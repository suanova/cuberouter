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
import { afterEach, beforeEach, describe, expect, test, vi } from 'vitest'

import { listPlatformOrganizations } from '@/features/organization/api'
import { searchUsers } from '@/features/users/api'

import {
  createCubyApp,
  createCubyAppGrant,
  deleteCubyAppGrant,
  listCubyAppGrants,
  type CubyApp,
  type CubyAppGrant,
} from '../../api'
import { CubyAppAccessDialog } from '../cuby-app-access-dialog'
import { CubyAppCreateDialog } from '../cuby-app-create-dialog'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('../../api', () => ({
  createCubyApp: vi.fn(),
  createCubyAppGrant: vi.fn(),
  deleteCubyAppGrant: vi.fn(),
  listCubyAppGrants: vi.fn(),
}))

vi.mock('@/features/organization/api', () => ({
  listPlatformOrganizations: vi.fn(),
}))

vi.mock('@/features/users/api', () => ({
  searchUsers: vi.fn(),
}))

const createAppMock = vi.mocked(createCubyApp)
const createGrantMock = vi.mocked(createCubyAppGrant)
const deleteGrantMock = vi.mocked(deleteCubyAppGrant)
const listGrantsMock = vi.mocked(listCubyAppGrants)
const organizationsMock = vi.mocked(listPlatformOrganizations)
const usersMock = vi.mocked(searchUsers)

const APP: CubyApp = {
  id: 3,
  app_id: 'ai-company',
  name: 'AI company',
  description: '',
  status: 'enabled',
  created_at: 1_760_000_000,
  updated_at: 1_760_000_000,
  user_grant_count: 1,
  organization_grant_count: 1,
}

const GRANTS: CubyAppGrant[] = [
  {
    id: 11,
    app_id: 'ai-company',
    subject_type: 'organization',
    subject_id: 7,
    created_at: 1,
    organization_name: 'Harbour Ltd',
    organization_status: 'active',
  },
  {
    id: 12,
    app_id: 'ai-company',
    subject_type: 'user',
    subject_id: 21,
    created_at: 1,
    username: 'mei',
    email: 'mei@example.com',
  },
  {
    id: 13,
    app_id: 'ai-company',
    subject_type: 'user',
    subject_id: 99,
    created_at: 1,
  },
]

const queryClients: QueryClient[] = []

function renderWithQueries(node: React.ReactNode): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClients.push(queryClient)
  render(<QueryClientProvider client={queryClient}>{node}</QueryClientProvider>)
}

beforeEach(() => {
  createAppMock.mockResolvedValue({ success: true })
  createGrantMock.mockResolvedValue({ success: true })
  deleteGrantMock.mockResolvedValue({ success: true })
  listGrantsMock.mockResolvedValue({
    success: true,
    data: { items: GRANTS, total: GRANTS.length },
  })
})

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
})

describe('add app dialog', () => {
  test('cannot be confirmed until the id is one Cuby accepts', () => {
    render(
      <CubyAppCreateDialog open onOpenChange={vi.fn()} onCreated={vi.fn()} />
    )
    const confirm = screen.getByRole('button', { name: 'Add app' })
    const idField = screen.getByLabelText('App ID')

    expect(confirm).toBeDisabled()

    fireEvent.change(idField, { target: { value: 'AI Company' } })
    expect(confirm).toBeDisabled()
    expect(idField).toHaveAttribute('aria-invalid', 'true')

    fireEvent.change(idField, { target: { value: 'ai-company' } })
    expect(confirm).toBeEnabled()
    expect(idField).toHaveAttribute('aria-invalid', 'false')
  })

  test('registers the trimmed id and name, then closes and tells the page', async () => {
    const onOpenChange = vi.fn()
    const onCreated = vi.fn()
    render(
      <CubyAppCreateDialog
        open
        onOpenChange={onOpenChange}
        onCreated={onCreated}
      />
    )

    fireEvent.change(screen.getByLabelText('App ID'), {
      target: { value: ' ai-company ' },
    })
    fireEvent.change(screen.getByLabelText('Name'), {
      target: { value: ' AI company ' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add app' }))

    await waitFor(() =>
      expect(createAppMock).toHaveBeenCalledWith({
        app_id: 'ai-company',
        name: 'AI company',
        description: '',
      })
    )
    await waitFor(() => expect(onCreated).toHaveBeenCalledTimes(1))
    expect(onOpenChange).toHaveBeenCalledWith(false)
  })

  test('stays open with what was typed when the server refuses', async () => {
    createAppMock.mockResolvedValue({
      success: false,
      message: 'cuby app id already exists',
    })
    const onOpenChange = vi.fn()
    const onCreated = vi.fn()
    render(
      <CubyAppCreateDialog
        open
        onOpenChange={onOpenChange}
        onCreated={onCreated}
      />
    )

    fireEvent.change(screen.getByLabelText('App ID'), {
      target: { value: 'ai-company' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Add app' }))

    await waitFor(() => expect(createAppMock).toHaveBeenCalledTimes(1))
    expect(onCreated).not.toHaveBeenCalled()
    expect(onOpenChange).not.toHaveBeenCalledWith(false)
    expect(screen.getByLabelText('App ID')).toHaveValue('ai-company')
  })
})

describe('who has it dialog', () => {
  test('lists organizations and users, and names a holder that no longer exists by its id', async () => {
    renderWithQueries(
      <CubyAppAccessDialog
        app={APP}
        onOpenChange={vi.fn()}
        onChanged={vi.fn()}
      />
    )

    const holders = await screen.findByRole('list', { name: 'Has it now' })
    const rows = within(holders).getAllByRole('listitem')
    expect(rows).toHaveLength(3)
    expect(within(rows[0]).getByText('Harbour Ltd')).toBeInTheDocument()
    expect(within(rows[1]).getByText('mei')).toBeInTheDocument()
    expect(within(rows[1]).getByText('mei@example.com')).toBeInTheDocument()
    expect(within(rows[2]).getByText('User #99 (gone)')).toBeInTheDocument()
    expect(listGrantsMock).toHaveBeenCalledWith(3, { p: 1, page_size: 100 })
  })

  test('takes the app away from one holder and tells the page', async () => {
    const onChanged = vi.fn()
    renderWithQueries(
      <CubyAppAccessDialog
        app={APP}
        onOpenChange={vi.fn()}
        onChanged={onChanged}
      />
    )
    const holders = await screen.findByRole('list', { name: 'Has it now' })

    fireEvent.click(
      within(within(holders).getAllByRole('listitem')[1]).getByRole('button', {
        name: 'Take away',
      })
    )

    await waitFor(() => expect(deleteGrantMock).toHaveBeenCalledWith(3, 12))
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1))
  })

  test('finds an organization and gives it the app', async () => {
    organizationsMock.mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 8,
        total: 1,
        items: [{ id: 9, name: 'Quay Partners', slug: 'quay' }],
      },
    } as Awaited<ReturnType<typeof listPlatformOrganizations>>)
    const onChanged = vi.fn()
    renderWithQueries(
      <CubyAppAccessDialog
        app={APP}
        onOpenChange={vi.fn()}
        onChanged={onChanged}
      />
    )
    await screen.findByRole('list', { name: 'Has it now' })

    fireEvent.change(screen.getByLabelText('Give it to'), {
      target: { value: ' quay ' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    const results = await screen.findByRole('list', { name: 'Search results' })
    expect(organizationsMock).toHaveBeenCalledWith({
      p: 1,
      page_size: 8,
      keyword: 'quay',
      status: 'active,disabled',
    })
    fireEvent.click(within(results).getByRole('button', { name: 'Give' }))

    await waitFor(() =>
      expect(createGrantMock).toHaveBeenCalledWith(3, {
        subject_type: 'organization',
        subject_id: 9,
      })
    )
    await waitFor(() => expect(onChanged).toHaveBeenCalledTimes(1))
  })

  test('searches users instead once the holder type is switched to user', async () => {
    usersMock.mockResolvedValue({
      success: true,
      data: {
        page: 1,
        page_size: 8,
        total: 1,
        items: [{ id: 21, username: 'mei', email: 'mei@example.com' }],
      },
    } as Awaited<ReturnType<typeof searchUsers>>)
    renderWithQueries(
      <CubyAppAccessDialog
        app={APP}
        onOpenChange={vi.fn()}
        onChanged={vi.fn()}
      />
    )
    await screen.findByRole('list', { name: 'Has it now' })

    fireEvent.change(screen.getByRole('combobox', { name: 'Holder type' }), {
      target: { value: 'user' },
    })
    fireEvent.change(screen.getByLabelText('Give it to'), {
      target: { value: 'mei' },
    })
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    const results = await screen.findByRole('list', { name: 'Search results' })
    expect(usersMock).toHaveBeenCalledWith({ keyword: 'mei', page_size: 8 })
    expect(organizationsMock).not.toHaveBeenCalled()
    fireEvent.click(within(results).getByRole('button', { name: 'Give' }))

    await waitFor(() =>
      expect(createGrantMock).toHaveBeenCalledWith(3, {
        subject_type: 'user',
        subject_id: 21,
      })
    )
  })

  test('says nobody can use the app when it was given to no one', async () => {
    listGrantsMock.mockResolvedValue({
      success: true,
      data: { items: [], total: 0 },
    })
    renderWithQueries(
      <CubyAppAccessDialog
        app={APP}
        onOpenChange={vi.fn()}
        onChanged={vi.fn()}
      />
    )

    expect(
      await screen.findByText('Nobody yet. No account can use this app.')
    ).toBeInTheDocument()
  })

  test('renders nothing while no app is selected', () => {
    renderWithQueries(
      <CubyAppAccessDialog
        app={null}
        onOpenChange={vi.fn()}
        onChanged={vi.fn()}
      />
    )

    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
    expect(listGrantsMock).not.toHaveBeenCalled()
  })
})
