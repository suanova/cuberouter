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
import { describe, expect, test } from 'vitest'

import { buildUsageColumns } from '../components/usage-columns'

function columnKeys(variant: 'personal' | 'organization' | 'site'): string[] {
  return buildUsageColumns((key) => key, variant).map(
    (column) => (column as { accessorKey?: string }).accessorKey ?? ''
  )
}

describe('buildUsageColumns', () => {
  test('site variant exposes the user and billing account columns', () => {
    const keys = columnKeys('site')
    expect(keys).toEqual(
      expect.arrayContaining([
        'user_name',
        'billing_account_type',
        'model_name',
      ])
    )
  })

  test('personal variant keeps billing but omits the user column', () => {
    const keys = columnKeys('personal')
    expect(keys).toContain('billing_account_type')
    expect(keys).not.toContain('user_name')
    expect(keys).not.toContain('responsible_name')
  })

  test('organization variant keeps the responsible user but omits billing', () => {
    const keys = columnKeys('organization')
    expect(keys).toContain('responsible_name')
    expect(keys).not.toContain('user_name')
    expect(keys).not.toContain('billing_account_type')
  })
})
