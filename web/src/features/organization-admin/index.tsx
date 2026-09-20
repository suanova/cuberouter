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
import { useTranslation } from 'react-i18next'

import { SectionPageLayout } from '@/components/layout'

import { PlatformOrganizationDissolveDialog } from './components/platform-organization-dissolve-dialog'
import { PlatformOrganizationEditDrawer } from './components/platform-organization-edit-drawer'
import { PlatformOrganizationStatusDialog } from './components/platform-organization-status-dialog'
import {
  PlatformOrganizationsProvider,
  usePlatformOrganizations,
} from './components/platform-organizations-provider'
import { PlatformOrganizationsTable } from './components/platform-organizations-table'

export { PlatformOrganizationDetail } from './components/platform-organization-detail'

/**
 * The platform's organization list.
 *
 * Separate from the organization center, which lists the caller's own
 * memberships and answers from their account context. This page lists every
 * organization on the platform with the administrator's own identity, and none
 * of its actions change the account context — an administrator acts *on* these
 * organizations, never inside them.
 */
function PlatformOrganizationsContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } =
    usePlatformOrganizations()

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>
          {t('Organization Management')}
        </SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <PlatformOrganizationsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

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
