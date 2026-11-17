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

import {
  deleteCubyApp,
  listCubyApps,
  setCubyAppManagement,
  updateCubyApp,
  type CubyApp,
} from '../api'
import { CubyApps } from '../index'

vi.mock('sonner', () => ({
  toast: { error: vi.fn(), success: vi.fn() },
}))

vi.mock('../api', () => ({
  deleteCubyApp: vi.fn(),
  listCubyApps: vi.fn(),
  setCubyAppManagement: vi.fn(),
  updateCubyApp: vi.fn(),
}))

// The two dialogs have their own tests; here they only need to be observable.
vi.mock('../components/cuby-app-create-dialog', () => ({
  CubyAppCreateDialog: (props: { open: boolean }) =>
    props.open ? <div data-testid='create-dialog' /> : null,
}))
vi.mock('../components/cuby-app-access-dialog', () => ({
  CubyAppAccessDialog: (props: { app: CubyApp | null }) =>
    props.app ? (
      <div data-testid='access-dialog'>{props.app.app_id}</div>
    ) : null,
}))

const listMock = vi.mocked(listCubyApps)
const managementMock = vi.mocked(setCubyAppManagement)
const updateMock = vi.mocked(updateCubyApp)
const deleteMock = vi.mocked(deleteCubyApp)

function app(overrides: Partial<CubyApp> = {}): CubyApp {
  return {
    id: 3,
    app_id: 'ai-company',
    name: 'AI company',
    description: '',
    status: 'enabled',
    created_at: 1_760_000_000,
    updated_at: 1_760_000_000,
    user_grant_count: 1,
    organization_grant_count: 2,
    ...overrides,
  }
}

const queryClients: QueryClient[] = []

function renderPage(): void {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  })
  queryClients.push(queryClient)
  render(
    <QueryClientProvider client={queryClient}>
      <CubyApps />
    </QueryClientProvider>
  )
}

function managementSwitch(): HTMLElement {
  return screen.getByRole('switch', { name: 'Limit Cuby apps by account' })
}

beforeEach(() => {
  listMock.mockResolvedValue({
    success: true,
    data: {
      enabled: false,
      apps: [
        app(),
        app({
          id: 4,
          app_id: 'financebuddy',
          name: 'FinanceBuddy',
          status: 'disabled',
          user_grant_count: 0,
          organization_grant_count: 0,
        }),
      ],
    },
  })
  managementMock.mockResolvedValue({ success: true, data: { enabled: true } })
  updateMock.mockResolvedValue({ success: true })
  deleteMock.mockResolvedValue({ success: true })
})

afterEach(() => {
  for (const queryClient of queryClients) queryClient.clear()
  queryClients.length = 0
})

describe('cuby apps page', () => {
  test('says Cuby limits nothing while the switch is off, and lists each app with who has it', async () => {
    renderPage()

    const list = await screen.findByRole('list', { name: 'Cuby Apps' })
    expect(managementSwitch()).not.toBeChecked()
    expect(
      screen.getByText(/^Off\. Cuby is told this instance does not manage apps/)
    ).toBeInTheDocument()

    const rows = within(list).getAllByRole('listitem')
    expect(rows).toHaveLength(2)
    expect(within(rows[0]).getByText('ai-company')).toBeInTheDocument()
    expect(
      within(rows[0]).getByText('Organizations: 2 · Users: 1')
    ).toBeInTheDocument()
    expect(within(rows[1]).getByText('Off for everyone')).toBeInTheDocument()
  })

  test('asks before limiting apps by account and only then switches it on', async () => {
    renderPage()
    await screen.findByRole('list', { name: 'Cuby Apps' })

    fireEvent.click(managementSwitch())

    const dialog = await screen.findByRole('alertdialog')
    expect(
      within(dialog).getByText('Limit Cuby apps by account?')
    ).toBeInTheDocument()
    expect(managementMock).not.toHaveBeenCalled()

    fireEvent.click(within(dialog).getByRole('button', { name: 'Switch on' }))

    await waitFor(() => expect(managementMock).toHaveBeenCalledWith(true))
  })

  test('leaves the switch alone when the administrator cancels', async () => {
    renderPage()
    await screen.findByRole('list', { name: 'Cuby Apps' })

    fireEvent.click(managementSwitch())
    const dialog = await screen.findByRole('alertdialog')
    fireEvent.click(within(dialog).getByRole('button', { name: 'Cancel' }))

    await waitFor(() =>
      expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
    )
    expect(managementMock).not.toHaveBeenCalled()
  })

  test('switches one app off for everyone without deleting it', async () => {
    renderPage()
    await screen.findByRole('list', { name: 'Cuby Apps' })

    fireEvent.click(
      screen.getByRole('switch', { name: 'AI company available' })
    )

    await waitFor(() =>
      expect(updateMock).toHaveBeenCalledWith(3, { status: 'disabled' })
    )
    expect(deleteMock).not.toHaveBeenCalled()
  })

  test('asks before deleting an app, because its grants go with it', async () => {
    renderPage()
    const list = await screen.findByRole('list', { name: 'Cuby Apps' })

    fireEvent.click(
      within(within(list).getAllByRole('listitem')[0]).getByRole('button', {
        name: 'Delete',
      })
    )

    const dialog = await screen.findByRole('alertdialog')
    expect(deleteMock).not.toHaveBeenCalled()
    fireEvent.click(within(dialog).getByRole('button', { name: 'Delete' }))

    await waitFor(() => expect(deleteMock).toHaveBeenCalledWith(3))
  })

  test('opens who-has-it for the app whose button was pressed', async () => {
    renderPage()
    const list = await screen.findByRole('list', { name: 'Cuby Apps' })

    fireEvent.click(
      within(within(list).getAllByRole('listitem')[1]).getByRole('button', {
        name: 'Who has it',
      })
    )

    expect(await screen.findByTestId('access-dialog')).toHaveTextContent(
      'financebuddy'
    )
  })

  test('says there are no apps yet when none is registered', async () => {
    listMock.mockResolvedValue({
      success: true,
      data: { enabled: false, apps: [] },
    })
    renderPage()

    expect(
      await screen.findByText(
        'No apps yet. Add one to start giving it to users and organizations.'
      )
    ).toBeInTheDocument()
  })
})
