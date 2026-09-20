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
import { refreshAccountContexts } from '@/lib/account-context'

import { OrganizationDissolveConfirm } from './organization-dissolve-confirm'
import { useOrganizations } from './organizations-provider'

/**
 * The organization center's dissolve row action.
 *
 * After the organization is gone the context list is stale — it still offers
 * the dissolved organization — and the table still shows its row, so both are
 * refreshed before the dialog reports success.
 */
export function OrganizationsDissolveDialog() {
  const { open, setOpen, currentRow, triggerRefresh } = useOrganizations()

  return (
    <OrganizationDissolveConfirm
      organization={open === 'dissolve' ? currentRow : null}
      onOpenChange={(value) => !value && setOpen(null)}
      onDissolved={() =>
        Promise.all([triggerRefresh(), refreshAccountContexts()])
      }
    />
  )
}
