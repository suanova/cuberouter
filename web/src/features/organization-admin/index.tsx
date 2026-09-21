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
import { useNavigate } from '@tanstack/react-router'
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'
import { refreshAccountContexts } from '@/lib/account-context'

import { PlatformOrganizationCreateDrawer } from './components/platform-organization-create-drawer'
import { PlatformOrganizationDissolveDialog } from './components/platform-organization-dissolve-dialog'
import { PlatformOrganizationEditDrawer } from './components/platform-organization-edit-drawer'
import { PlatformOrganizationStatusDialog } from './components/platform-organization-status-dialog'
import { PlatformOrganizationsPrimaryButtons } from './components/platform-organizations-primary-buttons'
import {
  PlatformOrganizationsProvider,
  usePlatformOrganizations,
} from './components/platform-organizations-provider'
import { PlatformOrganizationsTable } from './components/platform-organizations-table'
import { PLATFORM_ORGANIZATION_DEFAULT_TAB } from './lib'

export { PlatformOrganizationAuditLog } from './components/platform-organization-audit-log'
export { PlatformOrganizationDetail } from './components/platform-organization-detail'

/**
 * The platform's organization list.
 *
 * Separate from the organization center, which lists the caller's own
 * memberships and answers from their account context. This page lists every
 * organization on the platform with the administrator's own identity, and none
 * of its actions select an account context — an administrator acts *on* these
 * organizations, never as one of their members. Creating one is the single
 * exception to the "acts on" part, because the creator owns what they create;
 * even then the context is not switched, only the switcher's list refreshed.
 */
function PlatformOrganizationsContent() {
  const { t } = useTranslation()
  const navigate = useNavigate()
  const { open, setOpen, currentRow, triggerRefresh } =
    usePlatformOrganizations()

  // The administrator owns the new organization, so it has to appear in the
  // account-context switcher. They are not switched into it: the platform page
  // reads organizations with the administrator's own identity.
  const handleCreated = (organizationId: number) => {
    triggerRefresh()
    void refreshAccountContexts()
    void navigate({
      to: '/admin/organizations/$organizationId/$section',
      params: {
        organizationId: String(organizationId),
        section: PLATFORM_ORGANIZATION_DEFAULT_TAB,
      },
    })
  }

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Organization Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <PlatformOrganizationsPrimaryButtons />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <PlatformOrganizationsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <PlatformOrganizationCreateDrawer
        open={open === 'create'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        onCreated={handleCreated}
      />
      <PlatformOrganizationEditDrawer
        open={open === 'edit'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        organization={currentRow}
        onSaved={triggerRefresh}
      />
      <PlatformOrganizationStatusDialog
        open={open === 'status'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        organization={currentRow}
        onCompleted={triggerRefresh}
      />
      <PlatformOrganizationDissolveDialog
        open={open === 'dissolve'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        organization={currentRow}
        onCompleted={triggerRefresh}
      />
    </>
  )
}

export function PlatformOrganizations() {
  return (
    <PlatformOrganizationsProvider>
      <PlatformOrganizationsContent />
    </PlatformOrganizationsProvider>
  )
}
