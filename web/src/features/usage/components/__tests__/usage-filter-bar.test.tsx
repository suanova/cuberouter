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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi, type Mock } from 'vitest'

import {
  UsageFilterBar,
  type UsageFilterValues,
} from '../usage-filter-bar'

vi.mock('@/components/data-table', () => ({
  DataTableViewOptions: () => null,
}))

const table = {} as never

function renderFilterBar(
  onApply: Mock<(values: UsageFilterValues) => void>
): ReturnType<typeof render> {
  return render(
    <UsageFilterBar
      table={table}
      initial={{}}
      organizations={[]}
      modelOptions={['gpt-4o', 'wan3.0-video']}
      tokenOptions={[
        { name: 'personal-key', organization_id: 0, organization_name: '' },
        { name: 'org-key', organization_id: 9, organization_name: 'Acme' },
      ]}
      loading={false}
      onApply={onApply}
      onReset={() => undefined}
    />
  )
}

describe('UsageFilterBar dropdowns', () => {
  test('offers the models used in the range and applies the selection', () => {
    const onApply = vi.fn()
    renderFilterBar(onApply)

    const modelInput = screen.getByPlaceholderText('Model')
    fireEvent.focus(modelInput)

    expect(screen.getByRole('option', { name: 'All Models' })).toBeInTheDocument()
    const option = screen.getByRole('option', { name: 'gpt-4o' })
    expect(screen.getByRole('option', { name: 'wan3.0-video' })).toBeInTheDocument()

    fireEvent.mouseDown(option)
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ model: 'gpt-4o' })
    )
  })

  test('labels organization keys with the organization and applies the key name', () => {
    const onApply = vi.fn()
    renderFilterBar(onApply)

    fireEvent.focus(screen.getByPlaceholderText('API Key'))

    expect(screen.getByRole('option', { name: 'personal-key' })).toBeInTheDocument()
    const orgOption = screen.getByRole('option', { name: 'org-key (Acme)' })
    expect(orgOption).toBeInTheDocument()

    fireEvent.mouseDown(orgOption)
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ token: 'org-key' })
    )
  })
})
