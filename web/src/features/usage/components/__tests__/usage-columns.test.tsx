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
import type { ColumnDef } from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import type { ReactNode } from 'react'
import { describe, expect, test } from 'vitest'

import type { SelfUsageRow } from '../../types'
import { buildUsageColumns } from '../usage-columns'

function headerText(column: ColumnDef<SelfUsageRow>): ReactNode {
  if (typeof column.header !== 'function') return column.header
  return column.header({} as never)
}

function ColumnHeaders(props: {
  variant: 'personal' | 'organization'
}): ReactNode {
  const columns = buildUsageColumns((key) => key, props.variant)
  return (
    <table>
      <thead>
        <tr>
          {columns.map((column) => {
            const key = 'accessorKey' in column ? column.accessorKey : column.id
            return <th key={String(key)}>{headerText(column)}</th>
          })}
        </tr>
      </thead>
    </table>
  )
}

describe('buildUsageColumns', () => {
  test('personal usage shows the billing account and no member column', () => {
    render(<ColumnHeaders variant='personal' />)

    expect(
      screen.getByRole('columnheader', { name: 'Date' })
    ).toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Billing Account' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'User' })
    ).not.toBeInTheDocument()
  })

  test('organization usage shows the responsible member instead of the billing account', () => {
    render(<ColumnHeaders variant='organization' />)

    expect(
      screen.getByRole('columnheader', { name: 'User' })
    ).toBeInTheDocument()
    expect(
      screen.queryByRole('columnheader', { name: 'Billing Account' })
    ).not.toBeInTheDocument()
    expect(
      screen.getByRole('columnheader', { name: 'Model' })
    ).toBeInTheDocument()
  })
})
