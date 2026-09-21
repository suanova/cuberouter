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

import { OrganizationsDissolveDialog } from './components/organizations-dissolve-dialog'
import { OrganizationsEditDrawer } from './components/organizations-edit-drawer'
import {
  OrganizationsProvider,
  useOrganizations,
} from './components/organizations-provider'
import { OrganizationsStatusDialog } from './components/organizations-status-dialog'
import { OrganizationsTable } from './components/organizations-table'

function OrganizationsContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow } = useOrganizations()

  return (
    <>
      {/* No actions slot: creating an organization is a platform action and
          lives on the platform page, not here. */}
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Organizations')}</SectionPageLayout.Title>
        <SectionPageLayout.Content>
          <OrganizationsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <OrganizationsEditDrawer
        open={open === 'update'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        organization={open === 'update' ? currentRow || undefined : undefined}
      />
      <OrganizationsStatusDialog />
      <OrganizationsDissolveDialog />
    </>
  )
}

export function Organizations() {
  return (
    <OrganizationsProvider>
      <OrganizationsContent />
    </OrganizationsProvider>
  )
}

export { OrganizationDetail } from './components/organization-detail'
export { OrganizationInviteLanding } from './components/organization-invite-landing'
