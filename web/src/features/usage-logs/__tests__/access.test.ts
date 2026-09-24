/*
Copyright (C) 2023-2026 QuantumNous

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
import assert from 'node:assert/strict'
import { describe, test } from 'vitest'

import { ROLE } from '@/lib/roles'

import { resolveLogsViewAccess } from '../components/usage-logs-provider'

describe('usage log access tier', () => {
  test('keeps users and elevated self views on the self tier', () => {
    assert.equal(resolveLogsViewAccess(ROLE.USER, 'all'), 'self')
    assert.equal(resolveLogsViewAccess(ROLE.ADMIN, 'self'), 'self')
    assert.equal(resolveLogsViewAccess(ROLE.SUPER_ADMIN, 'self'), 'self')
  })

  test('distinguishes admin and root while viewing all logs', () => {
    assert.equal(resolveLogsViewAccess(ROLE.ADMIN, 'all'), 'admin')
    assert.equal(resolveLogsViewAccess(ROLE.SUPER_ADMIN, 'all'), 'root')
  })

  test('pins ops to the self tier on drawing/task categories (AdminAuth endpoints)', () => {
    // Common logs are OpsAuth: ops may view all.
    assert.equal(resolveLogsViewAccess(ROLE.OPS, 'all', 'common'), 'admin')
    // Drawing/task read endpoints are AdminAuth: ops must stay on self.
    assert.equal(resolveLogsViewAccess(ROLE.OPS, 'all', 'drawing'), 'self')
    assert.equal(resolveLogsViewAccess(ROLE.OPS, 'all', 'task'), 'self')
    // Admin and above are unaffected by the category restriction.
    assert.equal(resolveLogsViewAccess(ROLE.ADMIN, 'all', 'drawing'), 'admin')
    assert.equal(resolveLogsViewAccess(ROLE.ADMIN, 'all', 'task'), 'admin')
    assert.equal(
      resolveLogsViewAccess(ROLE.SUPER_ADMIN, 'all', 'task'),
      'root'
    )
    // Self tier stays self regardless of category.
    assert.equal(resolveLogsViewAccess(ROLE.OPS, 'self', 'drawing'), 'self')
  })
})
