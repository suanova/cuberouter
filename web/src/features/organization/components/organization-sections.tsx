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
import { Empty, EmptyDescription, EmptyTitle } from '@/components/ui/empty'

import type { OrganizationDetailTabKey } from '../constants'
import type { OrganizationDetail } from '../types'

/**
 * The body of the selected section.
 *
 * The tab strip and the section bodies are driven by the same capability set, so
 * a body only ever renders for a caller the backend has already allowed to read
 * it.
 */
export function OrganizationSections({
  detail,
  tab,
}: {
  detail: OrganizationDetail
  tab: OrganizationDetailTabKey
}) {
  return (
    <Empty>
      <EmptyTitle>{tab}</EmptyTitle>
      <EmptyDescription>{detail.organization.name}</EmptyDescription>
    </Empty>
  )
}
