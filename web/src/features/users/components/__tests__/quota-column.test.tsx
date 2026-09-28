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
import {
  flexRender,
  getCoreRowModel,
  useReactTable,
} from '@tanstack/react-table'
import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { formatQuota } from '@/lib/format'

import type { User } from '../../types'
import { useUsersColumns } from '../users-columns'

// @visactor/vchart ships ESM that vitest's externalized loader cannot resolve.
// Charts live in the user dashboard dialog, which is never opened here, so the
// chart boundary is stubbed.
vi.mock('@visactor/react-vchart', () => ({
  VChart: () => null,
}))

const WALLET_QUOTA = 99962113
const WALLET_USED_QUOTA = 37887
const WALLET_TOTAL = WALLET_QUOTA + WALLET_USED_QUOTA

function buildUser(overrides: Partial<User> = {}): User {
  return {
    id: 1,
    username: 'quota-user',
    display_name: 'Quota User',
    quota: 0,
    used_quota: 0,
    request_count: 0,
    group: 'default',
    status: 1,
    role: 1,
    ...overrides,
  }
}

// Renders the users table quota column cell through the real column
// definition, so the wiring between the user row and the cell is covered.
function QuotaCellProbe({ user }: { user: User }) {
  const columns = useUsersColumns()
  const table = useReactTable({
    data: [user],
    columns,
    getCoreRowModel: getCoreRowModel(),
  })
  const cell = table
    .getRowModel()
    .rows[0].getVisibleCells()
    .find((visibleCell) => visibleCell.column.id === 'quota')
  if (!cell) {
    throw new Error('quota column not found')
  }
  return <>{flexRender(cell.column.columnDef.cell, cell.getContext())}</>
}

describe('users list quota column', () => {
  test('renders wallet remaining and total for a user without subscription', () => {
    render(
      <QuotaCellProbe
        user={buildUser({ quota: WALLET_QUOTA, used_quota: WALLET_USED_QUOTA })}
      />
    )

    expect(screen.getByText(formatQuota(WALLET_QUOTA))).toBeInTheDocument()
    expect(screen.getByText(formatQuota(WALLET_TOTAL))).toBeInTheDocument()
    expect(screen.queryByText('No Quota')).not.toBeInTheDocument()
  })

  test('keeps the wallet basis for a user that also holds an active subscription', () => {
    render(
      <QuotaCellProbe
        user={buildUser({
          quota: WALLET_QUOTA,
          used_quota: WALLET_USED_QUOTA,
          subscription_total_quota: 5000000,
          subscription_remain_quota: 1000000,
          subscription_used_quota: 4000000,
          subscription_unlimited: false,
          subscription_remain_value: 2,
        })}
      />
    )

    expect(screen.getByText(formatQuota(WALLET_QUOTA))).toBeInTheDocument()
    expect(screen.queryByText(formatQuota(1000000))).not.toBeInTheDocument()
  })

  test('shows the no-quota badge when both wallet quota and usage are zero', () => {
    render(<QuotaCellProbe user={buildUser()} />)

    expect(screen.getByText('No Quota')).toBeInTheDocument()
  })
})
