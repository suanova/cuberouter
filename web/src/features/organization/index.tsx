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
import { useAuthStore } from '@/stores/auth-store'

import { OrganizationsDissolveDialog } from './components/organizations-dissolve-dialog'
import { OrganizationsMutateDrawer } from './components/organizations-mutate-drawer'
import { OrganizationsPrimaryButtons } from './components/organizations-primary-buttons'
import {
  OrganizationsProvider,
  useOrganizations,
} from './components/organizations-provider'
import { OrganizationsStatusDialog } from './components/organizations-status-dialog'
import { OrganizationsTable } from './components/organizations-table'
import { useOrganizationsQuery } from './hooks/use-organizations-query'

function OrganizationsContent() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, refreshTrigger } = useOrganizations()
  const currentUserId = useAuthStore((state) => state.auth.user?.id)
  const { data: organizations } = useOrganizationsQuery(refreshTrigger)

  // The backend counts organizations this user *created* that are not dissolved
  // (maxActiveOrganizationsPerUser), not the ones they merely belong to.
  const ownedCount = (organizations ?? []).filter(
    (organization) =>
      organization.created_by === currentUserId &&
      organization.status !== 'dissolved'
  ).length

  return (
    <>
      <SectionPageLayout fixedContent>
        <SectionPageLayout.Title>{t('Organizations')}</SectionPageLayout.Title>
        <SectionPageLayout.Actions>
          <OrganizationsPrimaryButtons ownedCount={ownedCount} />
        </SectionPageLayout.Actions>
        <SectionPageLayout.Content>
          <OrganizationsTable />
        </SectionPageLayout.Content>
      </SectionPageLayout>

      <OrganizationsMutateDrawer
        open={open === 'create' || open === 'update'}
        onOpenChange={(isOpen) => !isOpen && setOpen(null)}
        currentRow={open === 'update' ? currentRow || undefined : undefined}
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
