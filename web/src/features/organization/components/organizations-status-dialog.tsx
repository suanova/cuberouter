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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { refreshAccountContexts } from '@/lib/account-context'

import { updateOrganizationStatus } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { isOrganizationSlugConfirmed } from '../lib'
import { useOrganizations } from './organizations-provider'

/**
 * Enables or disables the organization. Disabling takes the organization's
 * traffic offline, so the operator has to retype its slug; enabling is the same
 * shape with different copy.
 */
export function OrganizationsStatusDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useOrganizations()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmSlug, setConfirmSlug] = useState('')

  const isOpen = open === 'status'
  const isEnabling = currentRow?.status === 'disabled'
  const confirmText = isSubmitting
    ? t('Saving...')
    : t(isEnabling ? 'Enable' : 'Disable')

  useEffect(() => {
    if (!isOpen) setConfirmSlug('')
  }, [isOpen])

  if (!currentRow) return null

  const handleConfirm = async () => {
    const status = isEnabling ? 'active' : 'disabled'
    setIsSubmitting(true)
    try {
      const result = await updateOrganizationStatus(currentRow.id, {
        status,
        confirm_name: confirmSlug.trim(),
      })
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.STATUS_FAILED))
        return
      }
      toast.success(
        t(isEnabling ? SUCCESS_MESSAGES.ENABLED : SUCCESS_MESSAGES.DISABLED)
      )
      setOpen(null)
      // A disabled organization can no longer be used as an account context, so
      // the switcher has to follow along.
      await Promise.all([triggerRefresh(), refreshAccountContexts()])
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t(ERROR_MESSAGES.UNEXPECTED)
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ConfirmDialog
      open={isOpen}
      onOpenChange={(value) => !value && setOpen(null)}
      title={
        isEnabling ? t('Enable Organization') : t('Disable Organization')
      }
      desc={
        isEnabling
          ? t(
              'Enabling {{name}} restores API access for its members and its API keys.',
              { name: currentRow.name }
            )
          : t(
              'Disabling {{name}} stops all API traffic for the organization and its API keys. Its data is kept.',
              { name: currentRow.name }
            )
      }
      confirmText={confirmText}
      destructive={!isEnabling}
      disabled={!isOrganizationSlugConfirmed(currentRow.slug, confirmSlug)}
      isLoading={isSubmitting}
      handleConfirm={handleConfirm}
    >
      <div className='flex flex-col gap-2'>
        <Label htmlFor='organization-status-confirm-slug'>
          {t('Type the organization slug to confirm:')}{' '}
          <span className='font-semibold'>{currentRow.slug}</span>
        </Label>
        <Input
          id='organization-status-confirm-slug'
          value={confirmSlug}
          onChange={(event) => setConfirmSlug(event.target.value)}
          autoComplete='off'
        />
      </div>
    </ConfirmDialog>
  )
}
