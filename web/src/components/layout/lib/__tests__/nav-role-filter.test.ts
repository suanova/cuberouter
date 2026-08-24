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
import { describe, expect, it } from 'vitest'

import type { NavGroup } from '../../types'
import { filterNavGroupsByRole } from '../nav-role-filter'

const groups: NavGroup[] = [
  {
    id: 'console',
    title: 'Console',
    items: [{ title: 'Dashboard', url: '/dashboard' }],
  },
  {
    id: 'ops',
    title: 'Ops',
    items: [
      { title: 'Ops Campaigns', url: '/ops/campaign', requiredRole: 5 },
      { title: 'Invite History', url: '/ops/invite-history', requiredRole: 5 },
    ],
  },
  {
    id: 'admin',
    title: 'Admin',
    items: [
      { title: 'System Info', url: '/system-info', requiredRole: 100 },
      { title: 'Users', url: '/users' },
    ],
  },
]

describe('filterNavGroupsByRole', () => {
  it('common user sees only the console group', () => {
    const result = filterNavGroupsByRole(groups, 1)
    expect(result.map((g) => g.id)).toEqual(['console'])
  })

  it('ops user sees console and ops groups, but not admin', () => {
    const result = filterNavGroupsByRole(groups, 5)
    expect(result.map((g) => g.id)).toEqual(['console', 'ops'])
  })

  it('admin sees all groups, with requiredRole items filtered', () => {
    const result = filterNavGroupsByRole(groups, 10)
    expect(result.map((g) => g.id)).toEqual(['console', 'ops', 'admin'])
    const admin = result.find((g) => g.id === 'admin')
    expect(admin?.items.map((i) => i.title)).toEqual(['Users'])
  })

  it('root sees everything', () => {
    const result = filterNavGroupsByRole(groups, 100)
    const admin = result.find((g) => g.id === 'admin')
    expect(admin?.items.map((i) => i.title)).toEqual(['System Info', 'Users'])
  })

  it('missing role behaves like guest', () => {
    const result = filterNavGroupsByRole(groups, undefined)
    expect(result.map((g) => g.id)).toEqual(['console'])
  })
})
