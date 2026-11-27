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
import { describe, expect, test } from 'vitest'

import { formatQuota, parseQuotaFromDollars } from '@/lib/format'

import { OrganizationInviteRestrictionsCell } from '../organization-invite-restrictions-cell'

describe('organization invite restrictions cell', () => {
  test.each([
    [
      'both unset',
      { model_access: null, quota_limit: null },
      'Not configured',
    ],
    [
      'explicit empty and zero',
      { model_access: '', quota_limit: 0 },
      'All models · No limit',
    ],
    [
      'listed models and positive quota',
      { model_access: '["a","b"]', quota_limit: parseQuotaFromDollars(10) },
      `2 model(s) · ${formatQuota(parseQuotaFromDollars(10))}`,
    ],
    [
      'unset models and positive quota',
      { model_access: null, quota_limit: parseQuotaFromDollars(5) },
      `Models unchanged · ${formatQuota(parseQuotaFromDollars(5))}`,
    ],
    [
      'invalid models json is unset',
      { model_access: '{oops', quota_limit: 100 },
      `Models unchanged · ${formatQuota(100)}`,
    ],
    [
      'one model is singular-safe',
      { model_access: '["a"]', quota_limit: null },
      '1 model(s) · Quota unchanged',
    ],
  ])('%s', (_name, invite, expected) => {
    render(<OrganizationInviteRestrictionsCell invite={invite} />)

    expect(screen.getByText(expected)).toBeInTheDocument()
  })
})
