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

import { dissolveOrganization } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { createIdempotencyKey, isOrganizationSlugConfirmed } from '../lib'
import { useOrganizations } from './organizations-provider'

/**
 * Dissolves the organization. Irreversible: the record survives as a tombstone
 * but the organization can never be re-enabled, so the operator has to retype
 * its slug.
 */
export function OrganizationsDissolveDialog() {
  const { t } = useTranslation()
  const { open, setOpen, currentRow, triggerRefresh } = useOrganizations()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmSlug, setConfirmSlug] = useState('')
  const [idempotencyKey, setIdempotencyKey] = useState(createIdempotencyKey)

  const isOpen = open === 'dissolve'

  useEffect(() => {
    if (!isOpen) return
    setConfirmSlug('')
    // One key per intent: retrying from this dialog replays the same request
    // rather than dissolving twice, and reopening the dialog starts over.
    setIdempotencyKey(createIdempotencyKey())
  }, [isOpen])

  if (!currentRow) return null

  const handleConfirm = async () => {
    setIsSubmitting(true)
    try {
      const result = await dissolveOrganization(
        currentRow.id,
        { confirm_name: confirmSlug.trim() },
        idempotencyKey
      )
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.DISSOLVE_FAILED))
        return
      }
      toast.success(t(SUCCESS_MESSAGES.DISSOLVED))
      setOpen(null)
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
      disabled={!isOrganizationSlugConfirmed(currentRow.slug, confirmSlug)}
      isLoading={isSubmitting}
      handleConfirm={handleConfirm}
    >
      <div className='flex flex-col gap-2'>
        <Label htmlFor='organization-dissolve-confirm-slug'>
          {t('Type the organization slug to confirm:')}{' '}
          <span className='font-semibold'>{currentRow.slug}</span>
        </Label>
        <Input
          id='organization-dissolve-confirm-slug'
          value={confirmSlug}
          onChange={(event) => setConfirmSlug(event.target.value)}
          autoComplete='off'
        />
      </div>
    </ConfirmDialog>
  )
}
