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
import { describe, expect, test, vi } from 'vitest'

import { ApiKeys } from '../index'

vi.mock('../components/api-keys-table', () => ({
  ApiKeysTable: () => <div>personal table</div>,
}))

vi.mock('../components/api-keys-primary-buttons', () => ({
  ApiKeysPrimaryButtons: () => <button type='button'>Create API Key</button>,
}))

vi.mock('../components/api-keys-dialogs', () => ({
  ApiKeysDialogs: () => null,
}))

describe('API keys page', () => {
  test('renders one key table with the create action', () => {
    render(<ApiKeys />)

    expect(screen.getByText('personal table')).toBeInTheDocument()
    expect(
      screen.getByRole('button', { name: 'Create API Key' })
    ).toBeInTheDocument()
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument()
  })
})
