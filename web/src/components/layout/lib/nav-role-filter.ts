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
import { isAdmin, isOps } from '@/lib/role-guards'

import type { NavGroup } from '../types'

/**
 * Narrow root navigation groups by the user's role:
 * - the `admin` group requires an admin-or-higher role, the `ops` group
 *   requires an ops-or-higher role;
 * - items with a `requiredRole` are hidden below that role;
 * - items with a `maxRole` are hidden above that role.
 */
export function filterNavGroupsByRole(
  navGroups: NavGroup[],
  role: number | undefined
): NavGroup[] {
  const current = role ?? 0
  return navGroups
    .filter((group) => {
      if (group.id === 'admin') {
        return isAdmin(current)
      }
      if (group.id === 'ops') {
        return isOps(current)
      }
      return true
    })
    .map((group) => {
      const items = group.items.filter(
        (item) =>
          (item.requiredRole === undefined || current >= item.requiredRole) &&
          (item.maxRole === undefined || current <= item.maxRole)
      )
      return items.length === group.items.length ? group : { ...group, items }
    })
}
