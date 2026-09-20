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

import { dissolveOrganization } from '@/features/organization/api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '@/features/organization/constants'
import {
  createIdempotencyKey,
  isOrganizationSlugConfirmed,
} from '@/features/organization/lib'

import { usePlatformOrganizations } from './platform-organizations-provider'

/**
 * Dissolves an organization from the platform side.
 *
 * Irreversible: the record survives as a tombstone so its logs and billing stay
 * readable, but the organization can never be enabled again. The operator
 * retypes the slug and gives a reason, and the request carries an idempotency
 * key — a dissolve that landed twice would be indistinguishable in the ledger
 * from two dissolutions, and the retry a timed-out first attempt invites is the
 * one thing a key exists to absorb.
 *
 * Restricted on the backend to the platform root role (`dissolve_organization`
 * is granted only to `platform_root`); the row menu only offers it to a caller
 * who holds that role.
 */
export function PlatformOrganizationDissolveDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } =
    usePlatformOrganizations()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmSlug, setConfirmSlug] = useState('')
  const [reason, setReason] = useState('')
  const [idempotencyKey, setIdempotencyKey] = useState(createIdempotencyKey)

  const isOpen = open === 'dissolve'

  useEffect(() => {
    if (!isOpen) return
    setConfirmSlug('')
    setReason('')
    // One key per intent: retrying from this dialog replays the same request,
    // and reopening the dialog starts a new intent.
    setIdempotencyKey(createIdempotencyKey())
  }, [isOpen])

  if (!currentRow) return null

  const isReasonMissing = reason.trim().length === 0
  const handleConfirm = async () => {
    if (isReasonMissing) return
    setIsSubmitting(true)
    try {
      const result = await dissolveOrganization(
        'admin',
        currentRow.id,
        { confirm_name: confirmSlug.trim(), reason: reason.trim() },
        idempotencyKey
      )
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.DISSOLVE_FAILED))
        return
      }
      toast.success(t(SUCCESS_MESSAGES.DISSOLVED))
      setOpen(null)
      triggerRefresh()
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
      title={t('Dissolve Organization')}
      desc={
        <>
          {t(
            'Dissolving {{name}} permanently closes the organization. Members lose access, its API keys stop working, and the action cannot be undone.',
            { name: currentRow.name }
          )}{' '}
          {t('Its logs and billing records are retained for auditing.')}
        </>
      }
      confirmText={isSubmitting ? t('Dissolving...') : t('Dissolve')}
      destructive
      disabled={
        isReasonMissing ||
        !isOrganizationSlugConfirmed(currentRow.slug, confirmSlug)
      }
      isLoading={isSubmitting}
      handleConfirm={handleConfirm}
    >
      <div className='flex flex-col gap-3'>
        <div className='flex flex-col gap-2'>
          <Label htmlFor='platform-organization-dissolve-reason'>
            {t('Reason')}
          </Label>
          <Textarea
            id='platform-organization-dissolve-reason'
            value={reason}
            onChange={(event) => setReason(event.target.value)}
            rows={3}
            placeholder={t('Why is this organization being dissolved?')}
          />
        </div>

        <div className='flex flex-col gap-2'>
          <Label htmlFor='platform-organization-dissolve-confirm-slug'>
            {t('Type the organization slug to confirm:')}{' '}
            <span className='font-semibold'>{currentRow.slug}</span>
          </Label>
          <Input
            id='platform-organization-dissolve-confirm-slug'
            value={confirmSlug}
            onChange={(event) => setConfirmSlug(event.target.value)}
            autoComplete='off'
          />
        </div>
      </div>
    </ConfirmDialog>
  )
}
