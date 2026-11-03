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
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { getOrganizations } from '@/features/organization/api'
import {
  ACCOUNT_CONTEXT_TYPE,
  useAccountContextStore,
} from '@/stores/account-context-store'

import { getSelfUsage } from './api'
import { UsageDateRangePicker } from './components/usage-date-range-picker'
import { UsageOverview } from './components/usage-overview'
import {
  resolveOverviewOrganizationId,
} from './lib/organization-selection'
import { usageDateParams, usageWindowFromSearch } from './lib/time-window'
import type { GetSelfUsageParams, UsageOrganizationOption } from './types'

export interface UsageSearch {
  start?: string
  end?: string
  model?: string
  token?: string
  billingAccountType?: string
  organizationId?: string
  overviewOrganizationId?: string
  page?: number
  pageSize?: number
}

interface UsagePageProps {
  search: UsageSearch
  onSearchChange: (next: Partial<UsageSearch>) => void
}

export function UsagePage(props: UsagePageProps) {
  const { t } = useTranslation()
  const window = usageWindowFromSearch(props.search)
  const dateParams = usageDateParams(window)

  const organizationsQuery = useQuery({
    queryKey: ['usage-organizations'],
    queryFn: async (): Promise<UsageOrganizationOption[]> => {
      const result = await getOrganizations()
      return (result.data ?? []).map((organization) => ({
        id: organization.id,
        name: organization.name,
      }))
    },
  })
  const organizations = organizationsQuery.data ?? []
  const currentContextOrganizationId = useAccountContextStore((state) =>
    state.accountContext.current?.type === ACCOUNT_CONTEXT_TYPE.ORGANIZATION
      ? state.accountContext.current.id
      : null
  )
  const selectedOrganizationId = props.search.overviewOrganizationId
    ? Number(props.search.overviewOrganizationId)
    : resolveOverviewOrganizationId(
        organizations,
        currentContextOrganizationId
      )

  const params: GetSelfUsageParams = {
    start: dateParams.start,
    end: dateParams.end,
    p: props.search.page ?? 1,
    page_size: props.search.pageSize ?? 20,
    ...(props.search.model ? { model_name: props.search.model } : {}),
    ...(props.search.token ? { token_name: props.search.token } : {}),
    ...(props.search.billingAccountType
      ? { billing_account_type: props.search.billingAccountType }
      : {}),
    ...(props.search.organizationId
      ? { organization_id: Number(props.search.organizationId) }
      : {}),
    ...(selectedOrganizationId
      ? { overview_organization_id: selectedOrganizationId }
      : {}),
  }

  const usageQuery = useQuery({
    queryKey: ['self-usage', params],
    queryFn: () => getSelfUsage(params),
  })
  const data = usageQuery.data?.data

  const updateSearch = (next: Partial<UsageSearch>) => {
    props.onSearchChange(next)
  }

  return (
    <SectionPageLayout>
      <SectionPageLayout.Title>{t('Usage')}</SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='mx-auto flex w-full max-w-7xl flex-col gap-4 sm:gap-5'>
          <div className='flex flex-wrap items-center gap-3'>
            <UsageDateRangePicker
              start={window.start}
              end={window.end}
              onChange={(range) => {
                const next = usageDateParams(range)
                updateSearch({ start: next.start, end: next.end, page: 1 })
              }}
            />
          </div>
          {data && !data.backfill_complete ? (
            <div
              role='status'
              className='rounded-lg border border-amber-500/40 bg-amber-500/5 px-4 py-2 text-sm text-amber-700 dark:text-amber-400'
            >
              {t('History is still being prepared.')}
            </div>
          ) : null}
          <UsageOverview
            data={data}
            loading={usageQuery.isLoading}
            organizations={organizations}
            selectedOrganizationId={selectedOrganizationId}
            onSelectOrganization={(id) =>
              updateSearch({ overviewOrganizationId: String(id), page: 1 })
            }
          />
          <div className='rounded-lg border' aria-hidden='true' />
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
