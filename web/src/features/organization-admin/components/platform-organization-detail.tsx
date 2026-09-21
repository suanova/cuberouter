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
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { GroupBadge } from '@/components/group-badge'
import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { OrganizationPageProvider } from '@/features/organization/components/organization-page-provider'
import { OrganizationSections } from '@/features/organization/components/organization-sections'
import {
  ORGANIZATION_STATUSES,
  READ_ONLY_MESSAGE_KEYS,
} from '@/features/organization/constants'
import { useOrganizationDetail } from '@/features/organization/hooks/use-organization-detail'
import { formatTimestamp } from '@/lib/format'

import {
  getPlatformOrganizationDetailActions,
  getPlatformOrganizationTabs,
  normalizePlatformOrganizationTabKey,
  type PlatformOrganizationTabKey,
} from '../lib'
import { PlatformOrganizationDissolveDialog } from './platform-organization-dissolve-dialog'
import { PlatformOrganizationEditDrawer } from './platform-organization-edit-drawer'
import { PlatformOrganizationOwnerRepair } from './platform-organization-owner-repair'
import { PlatformOrganizationStatusDialog } from './platform-organization-status-dialog'

const route = getRouteApi(
  '/_authenticated/admin/organizations/$organizationId/$section'
)

/**
 * One organization, as the platform sees it.
 *
 * This page is the organization center's sections read from the outside: the
 * same members, keys, logs, tasks, usage and audit trail, fetched through
 * `/api/admin/organizations` with the administrator's own identity. Nothing here
 * touches the account context, which is the whole difference — an administrator
 * acts *on* an organization, never inside it, so there is nothing to switch to
 * and no context to align.
 *
 * The capability set in the detail payload decides what appears, as on the
 * member page, but it is a different set: the backend resolves it from the
 * platform policy rather than from a membership, which is why the actions here
 * are read from it rather than inferred from the administrator's role.
 */
export function PlatformOrganizationDetail() {
  const { t } = useTranslation()
  const { organizationId, section } = route.useParams()
  const navigate = route.useNavigate()
  const search = route.useSearch()
  const activeTab = normalizePlatformOrganizationTabKey(section)
  const { detail, refetch } = useOrganizationDetail('admin', organizationId)

  const [isEditing, setIsEditing] = useState(false)
  const [isChangingStatus, setIsChangingStatus] = useState(false)
  const [isDissolving, setIsDissolving] = useState(false)

  /**
   * A section the backend refuses means this caller's capabilities changed while
   * the page was open — the administrator's role was reduced, or the
   * organization was dissolved underneath them. Unlike the member page there is
   * nowhere to fall back to: the administrator was never inside the
   * organization, so the page re-reads its own payload and lets the tab strip
   * and the actions come back smaller.
   *
   * The flag is what keeps that from becoming a loop when the refused section
   * re-mounts and is refused again.
   */
  const hasReportedRefusal = useRef(false)
  const handleForbidden = useCallback(() => {
    if (hasReportedRefusal.current) return
    hasReportedRefusal.current = true
    toast.info(
      t('You no longer have access to this organization as an administrator.')
    )
    void refetch()
  }, [refetch, t])

  const handleTabChange = (value: string) => {
    void navigate({
      to: '/admin/organizations/$organizationId/$section',
      params: {
        organizationId,
        section: value as PlatformOrganizationTabKey,
      },
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
  }
  const actions = getPlatformOrganizationDetailActions(access)
  const tabs = getPlatformOrganizationTabs(access)
  // The strip shrinks when a capability goes away, so a tab that is no longer in
  // it — a section the caller may no longer read, or everything but the overview
  // once the organization is dissolved — falls back to the first tab that is.
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
    <>
      {/* The three dialogs are siblings of the layout, not children of it.
          SectionPageLayout renders only the four slots it knows by name
          (Title / Actions / Content / Breadcrumb) and drops every other child,
          so anything mounted inside it never appears and its trigger looks dead:
          the state flips and nothing opens. Same reason the organization center
          renders its own dialogs alongside the layout. */}
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          <span className='flex items-center gap-2'>
            <span className='truncate'>{organization.name}</span>
            <StatusBadge
              label={t(statusMeta.labelKey)}
              variant={statusMeta.variant}
            />
            <GroupBadge group={organization.group} />
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {actions.canEdit && (
            <Button variant='outline' onClick={() => setIsEditing(true)}>
              {t('Edit')}
            </Button>
          )}
          {actions.canEnable && (
            <Button variant='outline' onClick={() => setIsChangingStatus(true)}>
              {t('Enable')}
            </Button>
          )}
          {actions.canDisable && (
            <Button
              variant='destructive'
              onClick={() => setIsChangingStatus(true)}
            >
              {t('Disable')}
            </Button>
          )}
          {actions.canDissolve && (
            <Button variant='destructive' onClick={() => setIsDissolving(true)}>
              {t('Dissolve Organization')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {actions.readOnly && (
              <Alert variant='destructive' className='shrink-0'>
                <AlertDescription>
                  {t(READ_ONLY_MESSAGE_KEYS.dissolved)}
                </AlertDescription>
              </Alert>
            )}

            <div className='text-muted-foreground flex shrink-0 flex-wrap items-center gap-x-3 gap-y-1 text-xs'>
              <span>
                {t('Slug')}:{' '}
                <span className='font-mono'>{organization.slug}</span>
              </span>
              <span>
                {t('Created At')}: {formatTimestamp(organization.created_at)}
              </span>
              <span>
                {t('Owner')}: #{organization.owner_user_id}
              </span>
            </div>

            <Tabs
              value={currentTab}
              onValueChange={handleTabChange}
              className='shrink-0'
            >
              <TabsList className='max-w-full flex-wrap justify-start group-data-horizontal/tabs:h-auto'>
                {tabs.map((tab) => (
                  <TabsTrigger key={tab.key} value={tab.key}>
                    {t(tab.labelKey)}
                  </TabsTrigger>
                ))}
              </TabsList>
            </Tabs>

            <div className='min-h-0 flex-1'>
              <OrganizationPageProvider
                surface='admin'
                search={search}
                navigate={navigate}
              >
                {currentTab === 'owner-repair' ? (
                  <PlatformOrganizationOwnerRepair
                    organizationId={organization.id}
                    slug={organization.slug}
                    ownerUserId={organization.owner_user_id}
                    onTransferred={refetch}
                  />
                ) : (
                  <OrganizationSections
                    detail={detail}
                    tab={currentTab}
                    readOnly={actions.readOnly}
                    onForbidden={handleForbidden}
                    onUpdated={refetch}
                    // Leaving is a member's own act and is not offered here: the
                    // platform surface grants no exit capability, so this only
                    // runs if the backend ever changes that, and re-reading the
                    // payload is the right response to it either way.
                    onLeftOrganization={refetch}
                  />
                )}
              </OrganizationPageProvider>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <PlatformOrganizationEditDrawer
        open={isEditing}
        onOpenChange={setIsEditing}
        organization={organization}
        onSaved={() => void refetch()}
      />

      <PlatformOrganizationStatusDialog
        open={isChangingStatus}
        onOpenChange={setIsChangingStatus}
        organization={organization}
        onCompleted={() => void refetch()}
      />

      <PlatformOrganizationDissolveDialog
        open={isDissolving}
        onOpenChange={setIsDissolving}
        organization={organization}
        // An organization is dissolved as soon as the request lands, so the
        // dialog closes onto the page that now describes it: read-only, with
        // everything but the overview gone. There is nothing to navigate away
        // from — the record stays readable, which is why its members' logs and
        // billing are still here.
        onCompleted={() => void refetch()}
      />
    </>
  )
}
