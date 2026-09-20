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

import { dissolveOrganization } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import { createIdempotencyKey, isOrganizationSlugConfirmed } from '../lib'
import { useOrganizationSurface } from './organization-page-provider'

/** The fields the confirmation needs; an `Organization` satisfies this. */
type DissolvableOrganization = {
  id: number
  name: string
  slug: string
}

type OrganizationDissolveConfirmProps = {
  /** The organization to dissolve, or `null` when the dialog is closed. */
  organization: DissolvableOrganization | null
  onOpenChange: (open: boolean) => void
  /** Runs once the organization is gone, to drop it from whatever list showed it. */
  onDissolved: () => Promise<unknown> | unknown
}

/**
 * Confirms dissolving an organization.
 *
 * Irreversible: the record survives as a tombstone but the organization can
 * never be re-enabled, so the operator has to retype its slug. It is opened by
 * passing an organization rather than by an `open` flag, which makes it
 * impossible to show the dialog without one.
 *
 * Shared by the organization center's row action and the organization's own
 * settings tab. What differs between them is only what happens afterwards, so
 * that is the one thing they pass in.
 */
export function OrganizationDissolveConfirm(
  props: OrganizationDissolveConfirmProps
) {
  const { t } = useTranslation()
  // The organization center's row action opens this from outside any detail
  // page, so this resolves to the member surface there and to the page's own
  // surface when the settings tab opens it.
  const surface = useOrganizationSurface()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmSlug, setConfirmSlug] = useState('')
  const [idempotencyKey, setIdempotencyKey] = useState(createIdempotencyKey)

  const isOpen = props.organization !== null

  useEffect(() => {
    if (!isOpen) return
    setConfirmSlug('')
    // One key per intent: retrying from this dialog replays the same request
    // rather than dissolving twice, and reopening the dialog starts over.
    setIdempotencyKey(createIdempotencyKey())
  }, [isOpen])

  const organization = props.organization
  if (!organization) return null

  const handleConfirm = async () => {
    setIsSubmitting(true)
    try {
      const result = await dissolveOrganization(
        surface,
        organization.id,
        { confirm_name: confirmSlug.trim() },
        idempotencyKey
      )
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.DISSOLVE_FAILED))
        return
      }
      toast.success(t(SUCCESS_MESSAGES.DISSOLVED))
      props.onOpenChange(false)
      await props.onDissolved()
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
      onOpenChange={props.onOpenChange}
      title={t('Dissolve Organization')}
      desc={
        <>
          {t(
            'Dissolving {{name}} permanently closes the organization. Members lose access, its API keys stop working, and the action cannot be undone.',
            { name: organization.name }
          )}{' '}
          {t('Its logs and billing records are retained for auditing.')}
        </>
      }
      confirmText={isSubmitting ? t('Dissolving...') : t('Dissolve')}
      destructive
      disabled={!isOrganizationSlugConfirmed(organization.slug, confirmSlug)}
      isLoading={isSubmitting}
      handleConfirm={handleConfirm}
    >
      <div className='flex flex-col gap-2'>
        <Label htmlFor='organization-dissolve-confirm-slug'>
          {t('Type the organization slug to confirm:')}{' '}
          <span className='font-semibold'>{organization.slug}</span>
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
