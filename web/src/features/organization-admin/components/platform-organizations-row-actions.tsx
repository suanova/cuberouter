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
import { Pencil, Power, PowerOff, Trash2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'

import type { OrganizationManagementView } from '@/features/organization/types'

import { getPlatformOrganizationActionFlags } from '../lib'
import { usePlatformOrganizations } from './platform-organizations-provider'

interface PlatformOrganizationsRowActionsProps {
  organization: OrganizationManagementView
  /** The caller holds the platform root role, so dissolve is available. */
  isRoot: boolean
}

/**
 * The platform list's row menu.
 *
 * Every entry is offered only when the backend would accept it — the menu is
 * built from the organization's status and the caller's platform role, so there
 * is no control here that answers with a refusal. A dissolved organization has
 * no menu at all: nothing about it can change, and the record itself is the only
 * thing left to read.
 */
export function PlatformOrganizationsRowActions({
  organization,
  isRoot,
}: PlatformOrganizationsRowActionsProps) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = usePlatformOrganizations()

  const flags = getPlatformOrganizationActionFlags(organization, { isRoot })

  if (flags.readOnly) {
    return <span className='text-muted-foreground text-sm'>-</span>
  }

  const openDialog = (type: 'edit' | 'status' | 'dissolve') => {
    setCurrentRow(organization)
    setOpen(type)
  }

  return (
    <DataTableRowActionMenu ariaLabel={t('Open menu')} contentClassName='w-48'>
      <DropdownMenuItem
        onSelect={() => openDialog('edit')}
        disabled={!flags.canEdit}
      >
        {t('Edit')}
        <DropdownMenuShortcut>
          <Pencil size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>

      {flags.canEnable ? (
        <DropdownMenuItem onSelect={() => openDialog('status')}>
          {t('Enable')}
          <DropdownMenuShortcut>
            <Power size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      ) : (
        <DropdownMenuItem
          onSelect={() => openDialog('status')}
          disabled={!flags.canDisable}
        >
          {t('Disable')}
          <DropdownMenuShortcut>
            <PowerOff size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      )}

      {flags.canDissolve && (
        <>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onSelect={() => openDialog('dissolve')}
            className='text-destructive focus:text-destructive'
          >
            {t('Dissolve')}
            <DropdownMenuShortcut>
              <Trash2 size={16} />
            </DropdownMenuShortcut>
          </DropdownMenuItem>
        </>
      )}
    </DataTableRowActionMenu>
  )
}
