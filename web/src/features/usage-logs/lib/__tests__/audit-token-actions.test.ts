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

import { renderAuditContent } from '../format'

const t = (key: string, opts?: Record<string, unknown>) =>
  key.replaceAll(/\{\{(\w+)\}\}/g, (_, name) => String(opts?.[name] ?? ''))

describe('admin token audit actions', () => {
  test.each([
    ['token.key_view', 'Viewed API key k1 (ID: 7) of user alice'],
    ['token.status_enable', 'Enabled API key k1 (ID: 7) of user alice'],
    ['token.status_disable', 'Disabled API key k1 (ID: 7) of user alice'],
    ['token.delete', 'Deleted API key k1 (ID: 7) of user alice'],
  ])('%s renders with target details', (action, expected) => {
    expect(
      renderAuditContent(
        {
          op: {
            action,
            params: { token_id: 7, token_name: 'k1', target_username: 'alice' },
          },
        },
        t
      )
    ).toBe(expected)
  })
})
