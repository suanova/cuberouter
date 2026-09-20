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
import {
  LogIn,
  Pencil,
  Power,
  PowerOff,
  Trash2,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'

import { ERROR_MESSAGES } from '../constants'
import { useEnterOrganization } from '../hooks/use-enter-organization'
import {
  getOrganizationListActionFlags,
  isOrganizationEnterable,
} from '../lib'
import type { UserOrganization } from '../types'
import { useOrganizations } from './organizations-provider'

interface OrganizationsRowActionsProps {
  organization: UserOrganization
}

export function OrganizationsRowActions({
  organization,
}: OrganizationsRowActionsProps) {
  const { t } = useTranslation()
  const { setOpen, setCurrentRow } = useOrganizations()
  const enterOrganization = useEnterOrganization()

  const { enter, edit, disable, enable, dissolve } =
    getOrganizationListActionFlags(organization)

  const isDisabled = organization.status === 'disabled'
  // An organization switched off by the platform cannot be switched back on by
  // its own administrators; only the platform can undo that.
  const canEnable = enable && organization.can_self_enable

  const openDialog = (type: 'update' | 'status' | 'dissolve') => {
    setCurrentRow(organization)
    setOpen(type)
  }

  const handleEnter = async () => {
    try {
      await enterOrganization(
        organization.id,
        isOrganizationEnterable(organization)
      )
    } catch (error) {
      // A failed context switch would leave the target page showing nothing but
      // a context mismatch, so the failure is reported here instead.
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t(ERROR_MESSAGES.UNEXPECTED)
      )
    }
  }

  return (
    <DataTableRowActionMenu
      ariaLabel={t('Open menu')}
      contentClassName='w-48'
    >
      <DropdownMenuItem onSelect={() => void handleEnter()} disabled={!enter}>
        {t('Open')}
        <DropdownMenuShortcut>
          <LogIn size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>

      {(edit || disable || enable || dissolve) && <DropdownMenuSeparator />}

      <DropdownMenuItem
        onSelect={() => openDialog('update')}
        disabled={!edit || isDisabled}
      >
        {t('Edit')}
        <DropdownMenuShortcut>
          <Pencil size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>

      {isDisabled ? (
        <DropdownMenuItem
          onSelect={() => openDialog('status')}
          disabled={!canEnable}
        >
          {t('Enable')}
          <DropdownMenuShortcut>
            <Power size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      ) : (
        <DropdownMenuItem
          onSelect={() => openDialog('status')}
          disabled={!disable}
        >
          {t('Disable')}
          <DropdownMenuShortcut>
            <PowerOff size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      )}

      <DropdownMenuSeparator />

      <DropdownMenuItem
        onSelect={() => openDialog('dissolve')}
        className='text-destructive focus:text-destructive'
        disabled={!dissolve}
      >
        {t('Dissolve')}
        <DropdownMenuShortcut>
          <Trash2 size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>
    </DataTableRowActionMenu>
  )
}
