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
import { describe, expect, it } from 'vitest'

import type { NavGroup } from '../../types'
import { filterNavGroupsByDshAccess } from '../nav-role-filter'

const groups: NavGroup[] = [
  {
    id: 'console',
    title: 'Console',
    items: [
      { title: 'Playground', url: '/playground' },
      { title: 'DSH Agent', url: '/dsh', requiresDshAccess: true },
      { title: 'Usage', url: '/usage' },
    ],
  },
  {
    id: 'dsh-only',
    title: 'DSH',
    items: [{ title: 'DSH Agent', url: '/dsh', requiresDshAccess: true }],
  },
]

describe('filterNavGroupsByDshAccess', () => {
  it('hides DSH-gated items when access is not granted', () => {
    const result = filterNavGroupsByDshAccess(groups, false)
    expect(result[0].items.map((item) => item.title)).toEqual([
      'Playground',
      'Usage',
    ])
  })

  it('drops a group once only DSH-gated items were in it', () => {
    const result = filterNavGroupsByDshAccess(groups, false)
    expect(result.map((group) => group.id)).toEqual(['console'])
  })

  it('keeps DSH-gated items when access is granted', () => {
    const result = filterNavGroupsByDshAccess(groups, true)
    expect(result).toBe(groups)
  })
})
