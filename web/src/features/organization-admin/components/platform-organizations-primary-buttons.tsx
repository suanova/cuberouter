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
import { Plus } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'

import { usePlatformOrganizations } from './platform-organizations-provider'

/**
 * The platform list's create entry, and the application's only one.
 *
 * No role check here: the route this renders under is already guarded by
 * `beforeLoad` for `ROLE.ADMIN`, and the create endpoint is guarded again
 * server-side. Repeating the check in the button would add a third place to keep
 * in step without adding a third guarantee.
 *
 * `setCurrentRow(null)` because the drawer is create-only and must not inherit
 * whatever row the previous dialog left behind.
 */
export function PlatformOrganizationsPrimaryButtons() {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = usePlatformOrganizations()

  return (
    <div className='flex gap-2'>
      <Button
        size='sm'
        onClick={() => {
          setCurrentRow(null)
          setOpen('create')
        }}
      >
        <Plus className='h-4 w-4' />
        {t('Create Organization')}
      </Button>
    </div>
  )
}
