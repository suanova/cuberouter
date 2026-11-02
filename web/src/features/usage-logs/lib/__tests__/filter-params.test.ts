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

import { buildApiParams } from '../utils'

describe('admin billing filters in log API params', () => {
  test('passes the billing type and organization id for an admin view', () => {
    const params = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: {
        billingAccountType: 'organization',
        organizationId: '7',
      },
      isAdmin: true,
    })

    expect(params.billing_account_type).toBe('organization')
    expect(params.organization_id).toBe(7)
  })

  test('ignores billing filters for a self view', () => {
    const params = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: {
        billingAccountType: 'organization',
        organizationId: '7',
      },
      isAdmin: false,
    })

    expect(params).not.toHaveProperty('billing_account_type')
    expect(params).not.toHaveProperty('organization_id')
  })

  test('drops an unrecognized billing type from a stale URL', () => {
    const params = buildApiParams({
      page: 1,
      pageSize: 20,
      searchParams: { billingAccountType: 'everything' },
      isAdmin: true,
    })

    expect(params).not.toHaveProperty('billing_account_type')
  })
})
