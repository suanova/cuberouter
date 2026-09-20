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
import { Textarea } from '@/components/ui/textarea'

import { updateOrganizationStatus } from '@/features/organization/api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '@/features/organization/constants'
import { isOrganizationSlugConfirmed } from '@/features/organization/lib'

import type { PlatformOrganizationTarget } from '../lib'

type PlatformOrganizationStatusDialogProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  organization: PlatformOrganizationTarget | null
  /** The status changed; the page behind the dialog is stale. */
  onCompleted: () => void
}

/**
 * Takes an organization's traffic offline, or puts it back.
 *
 * The operator retypes the slug, as on the member surface, and additionally has
 * to write why. The reason is not decoration: the change lands in the
 * organization's audit trail, which its own members read, and "the platform
 * switched us off" without a recorded justification is not something they can
 * act on.
 *
 * The switch is only offered in the direction the organization is not already
 * in — the caller builds the button from the current status — so this dialog
 * never shows an enable for an organization that is already active.
 */
export function PlatformOrganizationStatusDialog(
  props: PlatformOrganizationStatusDialogProps
) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmSlug, setConfirmSlug] = useState('')
  const [reason, setReason] = useState('')

  const organization = props.organization
  const isEnabling = organization?.status === 'disabled'

  useEffect(() => {
    if (!props.open) return
    setConfirmSlug('')
    setReason('')
  }, [props.open])

  if (!organization) return null

  const isReasonMissing = reason.trim().length === 0
  const handleConfirm = async () => {
    if (isReasonMissing) return
    setIsSubmitting(true)
    try {
      const result = await updateOrganizationStatus('admin', organization.id, {
        status: isEnabling ? 'active' : 'disabled',
        confirm_name: confirmSlug.trim(),
        reason: reason.trim(),
      })
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.STATUS_FAILED))
        return
      }
      toast.success(
        t(isEnabling ? SUCCESS_MESSAGES.ENABLED : SUCCESS_MESSAGES.DISABLED)
      )
      props.onOpenChange(false)
      props.onCompleted()
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
      open={props.open}
      onOpenChange={(value) => !value && props.onOpenChange(false)}
      title={isEnabling ? t('Enable Organization') : t('Disable Organization')}
      desc={
        isEnabling
          ? t(
              'Enabling {{name}} restores API access for its members and its API keys.',
              { name: organization.name }
            )
          : t(
              'Disabling {{name}} stops all API traffic for the organization and its API keys. Its data is kept, and the organization can be enabled again.',
              { name: organization.name }
            )
      }
      confirmText={isSubmitting ? t('Saving...') : t(isEnabling ? 'Enable' : 'Disable')}
      destructive={!isEnabling}
      disabled={
        isReasonMissing ||
        !isOrganizationSlugConfirmed(organization.slug, confirmSlug)
      }
      isLoading={isSubmitting}
      handleConfirm={handleConfirm}
    >
      <div className='flex flex-col gap-3'>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='platform-organization-status-reason'>
            {t('Reason')}
          </Label>
          <Textarea
            id='platform-organization-status-reason'
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            placeholder={t('Why is this change being made?')}
          />
          <p className='text-muted-foreground text-xs'>
            {t('Recorded in the organization audit log.')}
          </p>
        </div>

        <div className='flex flex-col gap-2'>
          <Label htmlFor='platform-organization-status-confirm-slug'>
            {t('Type the organization slug to confirm:')}{' '}
            <span className='font-semibold'>{organization.slug}</span>
          </Label>
          <Input
            id='platform-organization-status-confirm-slug'
            value={confirmSlug}
            onChange={(event) => setConfirmSlug(event.target.value)}
            autoComplete='off'
          />
        </div>
      </div>
    </ConfirmDialog>
  )
}
