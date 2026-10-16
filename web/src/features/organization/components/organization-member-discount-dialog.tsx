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
import { useQuery } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { ModelDiscountEditor } from '@/features/users/components/model-discount-editor'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

import {
  getOrganizationMemberModelDiscounts,
  updateOrganizationMemberModelDiscounts,
} from '../api'
import type { OrganizationMemberRow, UserModelDiscounts } from '../types'
import { useOrganizationSurface } from './organization-page-provider'

/**
 * A member's per-model model discounts, edited by the organization's own
 * administrators.
 *
 * The discounts are a property of the person, not of the membership: the same
 * record is what prices every call the member makes, whichever wallet pays for
 * it, and an organization key is priced by the member responsible for it. That
 * is why this dialog edits the member rather than the organization, and why it
 * is only offered for a row the caller may act on.
 *
 * The write replaces the record in full, so the editor is opened on what the
 * backend holds and the save button stays unavailable until the read lands —
 * saving an unloaded record would clear every discount the member has.
 */
export function OrganizationMemberDiscountDialog(props: {
  organizationId: number
  /** The row whose discounts are being edited; null while the dialog is closed. */
  member: OrganizationMemberRow | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const member = props.member
  const [draft, setDraft] = useState<UserModelDiscounts | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  // The editor's model picker offers the models the site prices.
  const { models: pricingModels } = usePricingData(member !== null)
  const modelOptions = useMemo(
    () => pricingModels.map((pricing) => pricing.model_name),
    [pricingModels]
  )

  const {
    data: storedDiscounts,
    isLoading,
    isError,
  } = useQuery({
    queryKey: [
      'organization-member-model-discounts',
      surface,
      props.organizationId,
      member?.user_id,
      getAccountContextCacheKey(),
    ],
    queryFn: () =>
      getOrganizationMemberModelDiscounts(
        props.organizationId,
        member?.user_id ?? 0,
        surface
      ),
    enabled: member !== null,
  })

  // The dialog outlives a close (the section keeps it mounted and only drops
  // the row it holds), so edits left over from the previous visit must not be
  // shown for the next member. Every visit passes through the closed state, so
  // this resets the draft once per opening.
  useEffect(() => {
    setDraft(null)
  }, [member])

  if (!member) return null

  const discounts = draft ?? storedDiscounts ?? {}
  // A failure here has already been reported by the API client's interceptor;
  // the placeholder is what keeps the dialog from presenting an empty editor
  // that reads as "this member has no discounts".
  const canSave = !isSaving && !isLoading && !isError

  const renderEditor = () => {
    if (isLoading) {
      return <p className='text-muted-foreground text-sm'>{t('Loading')}</p>
    }
    if (isError) {
      return <p className='text-destructive text-sm'>{t('Failed to load')}</p>
    }
    return (
      <ModelDiscountEditor
        value={discounts}
        modelOptions={modelOptions}
        disabled={isSaving}
        onChange={setDraft}
      />
    )
  }

  const save = async () => {
    setIsSaving(true)
    try {
      await updateOrganizationMemberModelDiscounts(
        props.organizationId,
        member.user_id,
        discounts,
        surface
      )
      toast.success(t('Operation successful'))
      props.onOpenChange(false)
    } catch {
      // The API client's interceptor already surfaces the failure as a toast,
      // and the dialog stays open with the edit for the operator to retry.
    } finally {
      setIsSaving(false)
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onOpenChange(false)
      }}
      title={t('Member Model Discounts')}
      description={t(
        'Applies to all calls made by this member (including personal wallet); organization keys are attributed to their responsible member.'
      )}
      bodyClassName='space-y-4'
      footer={
        <>
          <Button
            type='button'
            variant='outline'
            onClick={() => props.onOpenChange(false)}
            disabled={isSaving}
          >
            {t('Cancel')}
          </Button>
          <Button type='button' onClick={() => void save()} disabled={!canSave}>
            {isSaving ? <Loader2 className='animate-spin' /> : null}
            {isSaving ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm'>
        <dt className='text-muted-foreground'>{t('Username')}</dt>
        <dd className='truncate'>{member.username || '-'}</dd>
        <dt className='text-muted-foreground'>{t('Email')}</dt>
        <dd className='truncate'>{member.email || '-'}</dd>
      </dl>

      {renderEditor()}
    </Dialog>
  )
}
