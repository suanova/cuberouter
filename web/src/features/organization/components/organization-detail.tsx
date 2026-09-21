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
import { useQueryClient } from '@tanstack/react-query'
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { SectionPageLayout } from '@/components/layout'
import { StatusBadge } from '@/components/status-badge'
import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Skeleton } from '@/components/ui/skeleton'
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'
import { refreshAccountContexts } from '@/lib/account-context'

import {
  ORGANIZATION_STATUSES,
  organizationRoleMeta,
  READ_ONLY_MESSAGE_KEYS,
  type OrganizationDetailTabKey,
} from '../constants'
import { useOrganizationDetail } from '../hooks/use-organization-detail'
import {
  getOrganizationReadOnlyState,
  getOrganizationTabs,
  normalizeOrganizationTabKey,
} from '../lib'
import { OrganizationDissolveConfirm } from './organization-dissolve-confirm'
import { OrganizationExitDialog } from './organization-member-dialogs'
import { OrganizationPageProvider } from './organization-page-provider'
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
  const search = route.useSearch()
  const queryClient = useQueryClient()
  const activeTab = normalizeOrganizationTabKey(section)
  const { detail, refetch } = useOrganizationDetail('member', organizationId)
  const [isDissolving, setIsDissolving] = useState(false)
  const [isLeaving, setIsLeaving] = useState(false)

  /**
   * Re-read the account contexts, drop everything cached under them and return
   * to the organization center.
   *
   * A section the backend refuses means this caller's access changed while the
   * page was open — they were removed, or the organization was dissolved. The
   * page can no longer show anything true, so it leaves.
   * `refreshAccountContexts` is what makes the next page correct: the server
   * resolves the current context itself and falls back to personal when the
   * stored organization is gone.
   */
  const leaveOrganization = useCallback(async () => {
    try {
      await refreshAccountContexts()
    } catch {
      // The store records the failure; leaving is still the right move.
    }
    // Everything cached was read under the lost access.
    await queryClient.invalidateQueries()
    void navigate({ to: '/organizations', replace: true })
  }, [navigate, queryClient])

  const leftRef = useRef(false)
  const handleForbidden = useCallback(() => {
    if (leftRef.current) return
    leftRef.current = true
    toast.info(
      t(
        'You no longer have access to this organization. We switched you to your personal dashboard.'
      )
    )
    void leaveOrganization()
  }, [leaveOrganization, t])

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
  // The role is the actor's, not the organization's: a platform administrator
  // browsing without membership has none, and the badge is simply absent.
  const roleMeta = organizationRoleMeta(actor.role)
  // Dissolving is offered where the settings are, and only to a caller who may
  // do it and whose access is not already read-only.
  const canDissolve =
    currentTab === 'settings' &&
    actor.capabilities.can_dissolve_organization &&
    !readOnly
  // Leaving is a member's own way out, and the button for it lives in the
  // members section — which a member does not have. Offered in the header for
  // exactly the callers that section cannot host it for; an administrator who
  // can see the roster keeps the button where it always was.
  const canLeaveFromHeader =
    actor.capabilities.can_exit_organization &&
    !readOnly &&
    !tabs.some((tab) => tab.key === 'members')
  return (
    <>
      {/* A sibling of the layout, not a child of it: SectionPageLayout renders
          only its four named slots and drops every other child, so a confirm
          mounted inside it would never mount and the Dissolve button would look
          dead. */}
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          <span className='flex items-center gap-2'>
            <span className='truncate'>{organization.name}</span>
            <StatusBadge
              label={t(statusMeta.labelKey)}
              variant={statusMeta.variant}
            />
            {roleMeta && (
              <StatusBadge
                label={t(roleMeta.labelKey)}
                variant={roleMeta.variant}
              />
            )}
          </span>
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          {canLeaveFromHeader && (
            <Button variant='outline' onClick={() => setIsLeaving(true)}>
              {t('Leave organization')}
            </Button>
          )}
          {canDissolve && (
            <Button variant='destructive' onClick={() => setIsDissolving(true)}>
              {t('Dissolve Organization')}
            </Button>
          )}
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <div className='flex h-full min-h-0 flex-col gap-4'>
            {readOnly && reason && (
              <Alert variant='destructive' className='shrink-0'>
                <AlertDescription>
                  {t(READ_ONLY_MESSAGE_KEYS[reason])}
                </AlertDescription>
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
              <OrganizationPageProvider
                surface='member'
                search={search}
                navigate={navigate}
              >
                <OrganizationSections
                  detail={detail}
                  tab={currentTab}
                  readOnly={readOnly}
                  onForbidden={handleForbidden}
                  onUpdated={refetch}
                  onLeftOrganization={leaveOrganization}
                />
              </OrganizationPageProvider>
            </div>
          </div>
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <OrganizationDissolveConfirm
        organization={isDissolving ? organization : null}
        onOpenChange={(value) => !value && setIsDissolving(false)}
        onDissolved={leaveOrganization}
      />

      <OrganizationExitDialog
        organizationId={organization.id}
        open={isLeaving}
        onOpenChange={setIsLeaving}
        onExited={leaveOrganization}
      />
    </>
  )
}
