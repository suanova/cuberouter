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
import { getRouteApi } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  ORGANIZATION_STATUSES,
  READ_ONLY_MESSAGE_KEYS,
  type OrganizationDetailTabKey,
} from '../constants'
import { useOrganizationDetail } from '../hooks/use-organization-detail'
import {
  getOrganizationReadOnlyState,
  getOrganizationTabs,
  normalizeOrganizationTabKey,
} from '../lib'
import { OrganizationSections } from './organization-sections'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

/**
 * One organization's workspace.
 *
 * Which sections exist is decided by the capability set the backend resolved for
 * this caller, so the tab strip is derived from the response rather than from
 * the caller's role: a section the caller cannot read is simply absent, because
 * its data requests would be rejected anyway.
 */
export function OrganizationDetail() {
  const { t } = useTranslation()
  const { organizationId, section } = route.useParams()
  const navigate = route.useNavigate()
  const activeTab = normalizeOrganizationTabKey(section)
  const detail = useOrganizationDetail(organizationId)

  const handleTabChange = (value: string) => {
    void navigate({
      to: '/organizations/$organizationId/$section',
      params: { organizationId, section: value as OrganizationDetailTabKey },
    })
  }

  if (!detail) {
    return (
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {detail === null ? (
            t('Organization')
          ) : (
            <Skeleton className='h-5 w-40' />
          )}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          {detail === null ? (
            <Alert variant='destructive'>
              <AlertDescription>
                {t('This organization is not available.')}
              </AlertDescription>
            </Alert>
          ) : (
            <Skeleton className='h-64 w-full' />
          )}
        </SectionPageLayout.Content>
      </SectionPageLayout>
    )
  }

  const { organization, actor } = detail
  const access = {
    capabilities: actor.capabilities,
    status: organization.status,
    accessMode: actor.access_mode,
    readOnly: actor.read_only,
  }
  const { readOnly, reason } = getOrganizationReadOnlyState(access)
  const tabs = getOrganizationTabs(access)
  // A section can vanish when the caller's access changes — a disabled
  // organization loses its settings tab, for instance. Landing on a tab that is
  // no longer in the strip would show nothing at all, so fall back to the first
  // one that is.
  const currentTab = tabs.some((tab) => tab.key === activeTab)
    ? activeTab
    : (tabs[0]?.key ?? activeTab)
  // An unrecognized status would otherwise crash the page; `OrganizationStatus`
  // is only as narrow as the backend keeps it.
  const statusMeta = ORGANIZATION_STATUSES[organization.status] ?? {
    labelKey: 'Unknown',
    variant: 'neutral' as const,
  }
  return (
    <SectionPageLayout fixedContent>
      <SectionPageLayout.Title>
        <span className='flex items-center gap-2'>
          <span className='truncate'>{organization.name}</span>
          <StatusBadge
            label={t(statusMeta.labelKey)}
            variant={statusMeta.variant}
          />
        </span>
      </SectionPageLayout.Title>
      <SectionPageLayout.Content>
        <div className='flex h-full min-h-0 flex-col gap-4'>
          {readOnly && reason && (
            <Alert variant='destructive' className='shrink-0'>
              <AlertDescription>{t(READ_ONLY_MESSAGE_KEYS[reason])}</AlertDescription>
            </Alert>
          )}

          <Tabs
            value={currentTab}
            onValueChange={handleTabChange}
            className='shrink-0'
          >
            <TabsList className='group-data-horizontal/tabs:h-auto max-w-full flex-wrap justify-start'>
              {tabs.map((tab) => (
                <TabsTrigger key={tab.key} value={tab.key}>
                  {t(tab.labelKey)}
                </TabsTrigger>
              ))}
            </TabsList>
          </Tabs>

          <div className='min-h-0 flex-1'>
            <OrganizationSections detail={detail} tab={currentTab} />
          </div>
        </div>
      </SectionPageLayout.Content>
    </SectionPageLayout>
  )
}
