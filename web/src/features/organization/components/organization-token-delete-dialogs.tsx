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
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'

import {
  batchDeleteOrganizationTokens,
  deleteOrganizationToken,
} from '../api'
import { organizationTokenErrorText } from '../lib'
import type { OrganizationTokenRow } from '../types'

type OrganizationTokenDeleteDialogProps = {
  organizationId: number
  /** The key to remove. `null` closes the dialog. */
  token: OrganizationTokenRow | null
  onOpenChange: (open: boolean) => void
  onDeleted: () => Promise<unknown> | unknown
}

/**
 * Removing one key.
 *
 * A key is a live credential, so the confirmation names it: the list can hold
 * two keys with the same name, and "delete this one" is not a safe instruction
 * without knowing which.
 */
export function OrganizationTokenDeleteDialog(
  props: OrganizationTokenDeleteDialogProps
) {
  const { t } = useTranslation()
  const [isDeleting, setIsDeleting] = useState(false)
  const token = props.token

  const confirm = async () => {
    if (!token) return
    setIsDeleting(true)
    try {
      const result = await deleteOrganizationToken(
        props.organizationId,
        token.id
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to delete organization key'))
        return
      }
      toast.success(t('Organization key deleted'))
      props.onOpenChange(false)
      await props.onDeleted()
    } catch (error) {
      toast.error(
        organizationTokenErrorText(
          error,
          t('Failed to delete organization key')
        )
      )
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <ConfirmDialog
      open={token !== null}
      onOpenChange={props.onOpenChange}
      title={t('Delete organization key')}
      desc={
        <div className='space-y-2'>
          <p>
            {t(
              'The key stops working immediately. Anything already using it will start failing.'
            )}
          </p>
          {token ? (
            <p className='text-muted-foreground font-mono text-xs'>
              {token.name} · #{token.id}
            </p>
          ) : null}
        </div>
      }
      destructive
      isLoading={isDeleting}
      handleConfirm={() => void confirm()}
      confirmText={t('Delete')}
    />
  )
}

type OrganizationTokensBatchDeleteDialogProps = {
  organizationId: number
  /** The keys the caller has chosen to remove; empty closes the dialog. */
  tokens: OrganizationTokenRow[]
  onOpenChange: (open: boolean) => void
  onDeleted: () => Promise<unknown> | unknown
}

/** Removing several keys in one request. */
export function OrganizationTokensBatchDeleteDialog(
  props: OrganizationTokensBatchDeleteDialogProps
) {
  const { t } = useTranslation()
  const [isDeleting, setIsDeleting] = useState(false)
  const isOpen = props.tokens.length > 0

  const confirm = async () => {
    if (!isOpen) return
    setIsDeleting(true)
    try {
      const result = await batchDeleteOrganizationTokens(
        props.organizationId,
        props.tokens.map((token) => token.id)
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to delete organization keys'))
        return
      }
      // The backend reports how many it actually removed, which can be fewer
      // than were asked for if something changed underneath the request.
      const deleted = result.data ?? props.tokens.length
      toast.success(
        t('Deleted {{count}} organization key(s)', { count: deleted })
      )
      props.onOpenChange(false)
      await props.onDeleted()
    } catch (error) {
      toast.error(
        organizationTokenErrorText(
          error,
          t('Failed to delete organization keys')
        )
      )
    } finally {
      setIsDeleting(false)
    }
  }

  return (
    <ConfirmDialog
      open={isOpen}
      onOpenChange={props.onOpenChange}
      title={t('Delete organization keys')}
      desc={t(
        'The {{count}} selected keys stop working immediately. Anything already using them will start failing.',
        { count: props.tokens.length }
      )}
      destructive
      isLoading={isDeleting}
      handleConfirm={() => void confirm()}
      confirmText={t('Delete')}
    />
  )
}
