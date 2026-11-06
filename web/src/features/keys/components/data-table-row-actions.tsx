/*
Copyright (C) 2023-2026 QuantumNous

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
import type { Row } from '@tanstack/react-table'
import { Edit, Eye, Loader2, Power, PowerOff, Trash2 } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { updateOrganizationToken } from '@/features/organization/api'
import { useAuthStore } from '@/stores/auth-store'

import { updateApiKeyStatus } from '../api'
import { API_KEY_STATUS, ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { apiKeySchema } from '../types'
import { useApiKeys } from './api-keys-provider'

type DataTableRowActionsProps<TData> = {
  row: Row<TData>
}

function ActionButton(props: {
  label: string
  onClick: () => void
  disabled?: boolean
  className?: string
  children: React.ReactNode
}) {
  return (
    <Tooltip>
      <TooltipTrigger
        render={
          <Button
            variant='ghost'
            size='icon-sm'
            onClick={props.onClick}
            disabled={props.disabled}
            aria-label={props.label}
            className={props.className}
          />
        }
      >
        {props.children}
      </TooltipTrigger>
      <TooltipContent>{props.label}</TooltipContent>
    </Tooltip>
  )
}

export function DataTableRowActions<TData>({
  row,
}: DataTableRowActionsProps<TData>) {
  const { t } = useTranslation()
  const apiKey = apiKeySchema.parse(row.original)
  const currentUserId = useAuthStore((state) => state.auth.user?.id)
  const { setOpen, setCurrentRow, triggerRefresh } = useApiKeys()
  const isOrganizationKey = apiKey.scope_type === 'organization'
  const isEnabled = apiKey.status === API_KEY_STATUS.ENABLED
  const [isTogglingStatus, setIsTogglingStatus] = useState(false)
  // A member may switch off a key they created, but a public key belongs to the
  // organization and stays with its administrators. Deletion is deliberately
  // absent here: the key is the organization's, not the member's.
  const canToggleOrganizationKey =
    isOrganizationKey &&
    apiKey.visibility === 'private' &&
    apiKey.creator_user_id !== undefined &&
    apiKey.creator_user_id === currentUserId

  const openView = () => {
    setCurrentRow(apiKey)
    setOpen('view')
  }

  const toggleLabel = isEnabled ? t('Disable') : t('Enable')

  const handleToggleStatus = async () => {
    const newStatus = isEnabled
      ? API_KEY_STATUS.DISABLED
      : API_KEY_STATUS.ENABLED

    setIsTogglingStatus(true)
    try {
      if (isOrganizationKey) {
        if (!apiKey.organization_id) {
          toast.error(t(ERROR_MESSAGES.UNEXPECTED))
          return
        }
        const result = await updateOrganizationToken(
          'member',
          apiKey.organization_id,
          apiKey.id,
          {
            name: apiKey.name,
            status: newStatus,
            expired_time: apiKey.expired_time,
            remain_quota: apiKey.remain_quota,
            unlimited_quota: apiKey.unlimited_quota,
            model_limits_enabled: apiKey.model_limits_enabled,
            model_limits: apiKey.model_limits ?? '',
            allow_ips: apiKey.allow_ips ?? '',
            group: apiKey.group ?? '',
            cross_group_retry: !!apiKey.cross_group_retry,
            visibility: apiKey.visibility,
            responsible_user_id: apiKey.responsible_user_id,
          },
          { accountContextId: apiKey.organization_id }
        )
        if (result.success) {
          toast.success(
            newStatus === API_KEY_STATUS.ENABLED
              ? t('Organization key enabled')
              : t('Organization key disabled')
          )
          triggerRefresh()
        } else {
          toast.error(
            result.message || t('Failed to update organization key')
          )
        }
        return
      }

      const result = await updateApiKeyStatus(apiKey.id, newStatus)
      if (result.success) {
        const message = isEnabled
          ? t(SUCCESS_MESSAGES.API_KEY_DISABLED)
          : t(SUCCESS_MESSAGES.API_KEY_ENABLED)
        toast.success(message)
        triggerRefresh()
      } else {
        toast.error(result.message || t(ERROR_MESSAGES.STATUS_UPDATE_FAILED))
      }
    } catch {
      toast.error(t(ERROR_MESSAGES.UNEXPECTED))
    } finally {
      setIsTogglingStatus(false)
    }
  }

  let statusIcon = <Power className='size-4' />
  if (isTogglingStatus) {
    statusIcon = <Loader2 className='size-4 animate-spin' />
  } else if (isEnabled) {
    statusIcon = <PowerOff className='size-4' />
  }

  if (isOrganizationKey) {
    return (
      <div className='-ml-1.5 flex items-center gap-1'>
        {canToggleOrganizationKey && (
          <ActionButton
            label={toggleLabel}
            onClick={() => void handleToggleStatus()}
            disabled={isTogglingStatus}
            className={
              isEnabled
                ? 'text-destructive hover:text-destructive'
                : 'text-emerald-600 hover:text-emerald-600 dark:text-emerald-400 dark:hover:text-emerald-400'
            }
          >
            {statusIcon}
          </ActionButton>
        )}
        <ActionButton label={t('View')} onClick={openView}>
          <Eye />
        </ActionButton>
      </div>
    )
  }

  return (
    <div className='-ml-1.5 flex items-center gap-1'>
      <ActionButton
        label={toggleLabel}
        onClick={() => void handleToggleStatus()}
        disabled={isTogglingStatus}
        className={
          isEnabled
            ? 'text-destructive hover:text-destructive'
            : 'text-emerald-600 hover:text-emerald-600 dark:text-emerald-400 dark:hover:text-emerald-400'
        }
      >
        {statusIcon}
      </ActionButton>
      <ActionButton
        label={t('Edit')}
        onClick={() => {
          setCurrentRow(apiKey)
          setOpen('update')
        }}
      >
        <Edit />
      </ActionButton>
      <ActionButton label={t('View')} onClick={openView}>
        <Eye />
      </ActionButton>
      <ActionButton
        label={t('Delete')}
        onClick={() => {
          setCurrentRow(apiKey)
          setOpen('delete')
        }}
        className='text-destructive hover:text-destructive'
      >
        <Trash2 />
      </ActionButton>
    </div>
  )
}
