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
import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'

import { UsagePage, type UsageSearch } from '@/features/usage'

const usageSearchSchema = z.object({
  start: z.string().optional().catch(''),
  end: z.string().optional().catch(''),
  model: z.string().optional().catch(''),
  token: z.string().optional().catch(''),
  billingAccountType: z.string().optional().catch(''),
  organizationId: z.string().optional().catch(''),
  overviewOrganizationId: z.string().optional().catch(''),
  page: z.number().optional().catch(1),
  pageSize: z.number().optional().catch(undefined),
})

export const Route = createFileRoute('/_authenticated/usage/')({
  component: UsageRoute,
  validateSearch: usageSearchSchema,
})

function UsageRoute() {
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <UsagePage
      search={search}
      onSearchChange={(next: Partial<UsageSearch>) => {
        navigate({ search: (previous) => ({ ...previous, ...next }) })
      }}
    />
  )
}
