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

import { UsageFilterBar, type UsageFilterValues } from '../usage-filter-bar'

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
        {
          id: 1,
          name: 'personal-key',
          organization_id: 0,
          organization_name: '',
        },
        {
          id: 2,
          name: 'org-key',
          organization_id: 9,
          organization_name: 'Acme',
        },
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

    const modelInput = screen.getByRole('combobox', { name: 'Model' })
    fireEvent.focus(modelInput)

    expect(
      screen.getByRole('option', { name: 'All Models' })
    ).toBeInTheDocument()
    const option = screen.getByRole('option', { name: 'gpt-4o' })
    expect(
      screen.getByRole('option', { name: 'wan3.0-video' })
    ).toBeInTheDocument()

    fireEvent.mouseDown(option)
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ model: 'gpt-4o' })
    )
  })

  test('labels organization keys with the organization and applies the key name', () => {
    const onApply = vi.fn()
    renderFilterBar(onApply)

    fireEvent.focus(screen.getByRole('combobox', { name: 'API Key' }))

    expect(
      screen.getByRole('option', { name: 'personal-key' })
    ).toBeInTheDocument()
    const orgOption = screen.getByRole('option', { name: 'org-key (Acme)' })
    expect(orgOption).toBeInTheDocument()

    fireEvent.mouseDown(orgOption)
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ token: 'org-key', tokenId: 2 })
    )
  })

  test('keeps an unapplied token draft when the options refresh', () => {
    const options = [
      { id: 2, name: 'org-key', organization_id: 9, organization_name: 'Acme' },
    ]
    const ui = (tokenOptions: typeof options) => (
      <UsageFilterBar
        table={table}
        initial={{}}
        organizations={[]}
        modelOptions={[]}
        tokenOptions={tokenOptions}
        loading={false}
        onApply={() => undefined}
        onReset={() => undefined}
      />
    )
    const rendered = render(ui(options))
    const combobox = screen.getByRole('combobox', { name: 'API Key' })
    fireEvent.focus(combobox)
    fireEvent.mouseDown(screen.getByRole('option', { name: 'org-key (Acme)' }))
    // Selecting reopens the search input; close it so the selected label shows.
    fireEvent.keyDown(combobox, { key: 'Escape' })

    rendered.rerender(ui([...options]))

    expect(screen.getByRole('combobox', { name: 'API Key' })).toHaveValue(
      'org-key (Acme)'
    )
  })

  test('organization usage offers the member filter and hides billing', () => {
    const onApply = vi.fn()
    render(
      <UsageFilterBar
        table={table}
        initial={{}}
        organizations={[]}
        modelOptions={['gpt-4o']}
        tokenOptions={[]}
        userOptions={[
          { id: 101, name: 'Alice Chen' },
          { id: 102, name: 'Bob' },
        ]}
        variant='organization'
        loading={false}
        onApply={onApply}
        onReset={() => undefined}
      />
    )

    expect(screen.queryByText('All Billing Types')).not.toBeInTheDocument()

    const userCombobox = screen.getByRole('combobox', { name: 'User' })
    fireEvent.focus(userCombobox)
    fireEvent.mouseDown(screen.getByRole('option', { name: 'Bob' }))
    fireEvent.click(screen.getByRole('button', { name: 'Search' }))

    expect(onApply).toHaveBeenCalledWith(
      expect.objectContaining({ userId: 102 })
    )
  })
})
