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
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip'

import { MAX_ACTIVE_ORGANIZATIONS } from '../constants'
import { useOrganizations } from './organizations-provider'

export function OrganizationsPrimaryButtons({
  ownedCount,
}: {
  /**
   * How many non-dissolved organizations this user created. The backend caps
   * that, not membership: being invited to others' organizations is unlimited.
   */
  ownedCount: number
}) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useOrganizations()
  const atLimit = ownedCount >= MAX_ACTIVE_ORGANIZATIONS

  const handleCreate = () => {
    setCurrentRow(null)
    setOpen('create')
  }

  const button = (
    <Button size='sm' onClick={handleCreate} disabled={atLimit}>
      <Plus className='h-4 w-4' />
      {t('Create Organization')}
    </Button>
  )

  if (!atLimit) return button

  // The backend rejects the request past the limit; saying so up front beats a
  // failure after the form has been filled in.
  return (
    <Tooltip>
      <TooltipTrigger render={<span className='inline-flex' />}>
        {button}
      </TooltipTrigger>
      <TooltipContent>
        <p className='text-xs'>
          {t('You can create at most {{count}} organizations.', {
            count: MAX_ACTIVE_ORGANIZATIONS,
          })}
        </p>
      </TooltipContent>
    </Tooltip>
  )
}
