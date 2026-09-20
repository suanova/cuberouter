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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  normalizeOrganizationBillingPanel,
  ORGANIZATION_BILLING_PANELS,
  type OrganizationBillingPanel,
} from '../lib'
import { OrganizationBillingDetails } from './organization-billing-details'
import { OrganizationBillingMonthlyOverview } from './organization-billing-monthly-overview'
import { OrganizationBillingUserOverview } from './organization-billing-user-overview'
import { useOrganizationSectionRoute } from './organization-page-provider'
import { OrganizationSection, OrganizationSectionEmpty } from './organization-section'

/**
 * The three ways of reading the same ledger.
 *
 * A member, a month and a request are the three questions an administrator
 * asks about cost, and each one is a different aggregation over the same
 * records, so they are panels rather than columns of one table.
 */
const PANEL_LABELS: Record<OrganizationBillingPanel, string> = {
  user: 'By member',
  monthly: 'By month',
  details: 'Cost details',
}

type OrganizationBillingSectionProps = {
  organizationId: number
  /** The caller may read the organization's aggregated usage. */
  canView: boolean
  /**
   * The caller reads every member's rows, not just their own.
   *
   * Without it both the endpoint and this section narrow to the caller, which
   * is what makes the member column and the holder filter worth hiding.
   */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * What the organization spent: per member, per month, and entry by entry.
 *
 * The open panel lives in the URL rather than in component state. It carries no
 * more weight than a state hook, and it keeps a link coherent — the panels hold
 * their own filters in the URL too, and a link that names a filter without
 * naming the panel it belongs to would open a view where nothing appears to be
 * filtered.
 */
export function OrganizationBillingSection(props: OrganizationBillingSectionProps) {
  const { t } = useTranslation()
  const { search, navigate } = useOrganizationSectionRoute()
  const panel = normalizeOrganizationBillingPanel(search.billingTab)

  if (!props.canView) {
    return (
      <OrganizationSectionEmpty
        icon='usage'
        title={t('Usage')}
        message={t(
          'Only an organization owner or administrator can see organization-wide usage.'
        )}
      />
    )
  }

  const panels = {
    user: (
      <OrganizationBillingUserOverview
        organizationId={props.organizationId}
        canViewWideData={props.canViewWideData}
        onForbidden={props.onForbidden}
      />
    ),
    monthly: (
      <OrganizationBillingMonthlyOverview
        organizationId={props.organizationId}
        canViewWideData={props.canViewWideData}
        onForbidden={props.onForbidden}
      />
    ),
    details: (
      <OrganizationBillingDetails
        organizationId={props.organizationId}
        canViewWideData={props.canViewWideData}
        onForbidden={props.onForbidden}
      />
    ),
  } satisfies Record<OrganizationBillingPanel, ReactNode>

  return (
    <OrganizationSection
      icon='usage'
      title={t('Usage')}
      description={t(
        'What the organization has been charged, by member, by month, and request by request.'
      )}
      actions={
        <Tabs
          value={panel}
          onValueChange={(value) =>
            void navigate({
              search: (previous) => ({
                ...previous,
                billingTab: normalizeOrganizationBillingPanel(value),
              }),
            })
          }
        >
          <TabsList>
            {ORGANIZATION_BILLING_PANELS.map((key) => (
              <TabsTrigger key={key} value={key}>
                {t(PANEL_LABELS[key])}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      }
      scroll
    >
      {panels[panel]}
    </OrganizationSection>
  )
}
