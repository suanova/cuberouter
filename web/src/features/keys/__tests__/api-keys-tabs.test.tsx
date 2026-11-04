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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { ApiKeys } from '../index'

const routeState = vi.hoisted(() => ({
  search: { tab: 'personal' } as Record<string, unknown>,
  navigate: vi.fn(),
}))

vi.mock('@tanstack/react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@tanstack/react-router')>()),
  getRouteApi: () => ({
    useSearch: () => routeState.search,
    useNavigate: () => routeState.navigate,
  }),
}))

vi.mock('../components/api-keys-table', () => ({
  ApiKeysTable: () => <div>personal table</div>,
}))

vi.mock('../components/self-organization-tokens-table', () => ({
  SelfOrganizationTokensTable: () => <div>organization table</div>,
}))

vi.mock('../components/api-keys-primary-buttons', () => ({
  ApiKeysPrimaryButtons: () => <button type='button'>Create API Key</button>,
}))

vi.mock('../components/api-keys-dialogs', () => ({
  ApiKeysDialogs: () => null,
}))

beforeEach(() => {
  routeState.search = { tab: 'personal' }
  routeState.navigate.mockReset()
})

describe('API keys page tabs', () => {
  test('switches to the organization keys table without the create action', async () => {
    const rendered = render(<ApiKeys />)

    expect(screen.getByText('personal table')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Create API Key' })
    ).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('tab', { name: 'Organization keys' })
    )

    expect(routeState.navigate).toHaveBeenCalled()
    const options = routeState.navigate.mock.calls[0]?.[0] as {
      search: (previous: Record<string, unknown>) => Record<string, unknown>
    }
    expect(options.search({ tab: 'personal' })).toMatchObject({
      tab: 'organization',
    })

    routeState.search = { tab: 'organization' }
    rendered.rerender(<ApiKeys />)

    expect(screen.getByText('organization table')).toBeInTheDocument()
    expect(
      screen.queryByRole('button', { name: 'Create API Key' })
    ).not.toBeInTheDocument()
  })
})
