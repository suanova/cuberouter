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
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Loader2 } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { ModelDiscountEditor } from '@/features/users/components/model-discount-editor'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

import {
  getOrganizationMemberModelAccess,
  getOrganizationMemberModelDiscounts,
  updateOrganizationMemberModelAccess,
  updateOrganizationMemberModelDiscounts,
} from '../api'
import type {
  OrganizationMemberRow,
  UserModelDiscounts,
} from '../types'
import { useOrganizationSurface } from './organization-page-provider'

/**
 * A member's model settings, as the organization's own administrators set
 * them: which models the member may use and what they are charged for each.
 *
 * The two belong on one screen because they answer one question — what this
 * member is allowed to call, and at what price. The allow-list is the honest
 * frame for the discounts: restricting a member to three models leaves nothing
 * to say about the sixtieth, so the discount rows follow the selection instead
 * of offering the whole catalogue.
 *
 * The access list is a full replacement and the discounts are one too, so the
 * dialog opens on what the backend holds and stays unsavable until every
 * record this caller may write has landed — saving an unloaded record would
 * clear everything the member had. The writes go out on one save, the access
 * list first; the discounts carry the whole record rather than only the rows
 * on screen, so narrowing the list never deletes a discount kept for a model
 * the member can reach again later.
 */
export function OrganizationMemberModelSettingsDialog(props: {
  organizationId: number
  /** The row whose settings are being edited; null while the dialog is closed. */
  member: OrganizationMemberRow | null
  /** The caller may set the member's model allow-list. */
  canManageMemberModels: boolean
  /** The caller may set the member's model discounts. */
  canManageMemberDiscounts: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const queryClient = useQueryClient()
  const member = props.member
  const [draftAccess, setDraftAccess] = useState<string[] | null>(null)
  const [draftDiscounts, setDraftDiscounts] =
    useState<UserModelDiscounts | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  const memberAccessQueryKey = [
    'organization-member-model-access',
    surface,
    props.organizationId,
    member?.user_id,
    getAccountContextCacheKey(),
  ]
  // The key token form reads the same allow-list under a shorter key — the
  // account context is not part of it — so a save invalidates the shared prefix
  // rather than only this dialog's exact entry.
  const memberAccessSharedQueryKey = [
    'organization-member-model-access',
    surface,
    props.organizationId,
    member?.user_id,
  ]
  const memberDiscountsQueryKey = [
    'organization-member-model-discounts',
    surface,
    props.organizationId,
    member?.user_id,
    getAccountContextCacheKey(),
  ]

  // The editor's model picker offers the models the site prices; it is only
  // needed when the discounts are edited freely.
  const { models: pricingModels } = usePricingData(
    member !== null && props.canManageMemberDiscounts
  )
  const modelOptions = useMemo(
    () => pricingModels.map((pricing) => pricing.model_name),
    [pricingModels]
  )

  const accessQuery = useQuery({
    queryKey: memberAccessQueryKey,
    queryFn: () =>
      getOrganizationMemberModelAccess(
        props.organizationId,
        member?.user_id ?? 0,
        surface
      ),
    enabled: member !== null && props.canManageMemberModels,
  })

  const discountsQuery = useQuery({
    queryKey: memberDiscountsQueryKey,
    queryFn: () =>
      getOrganizationMemberModelDiscounts(
        props.organizationId,
        member?.user_id ?? 0,
        surface
      ),
    enabled: member !== null && props.canManageMemberDiscounts,
  })

  // The dialog outlives a close (the section keeps it mounted and only drops
  // the row it holds), so edits left over from the previous visit must not be
  // shown for the next member. Every visit passes through the closed state, so
  // this resets the drafts once per opening.
  useEffect(() => {
    setDraftAccess(null)
    setDraftDiscounts(null)
  }, [member])

  if (!member) return null

  const storedAccess = accessQuery.data ?? { access: [], models: [] }
  const selection = draftAccess ?? storedAccess.access
  const discounts = draftDiscounts ?? discountsQuery.data ?? {}
  // The discounts are edited per accessible model while the member is
  // restricted, and freely while the member may use everything — the same
  // freedom a caller without the models grant has.
  const lockedModels =
    props.canManageMemberModels && selection.length > 0 ? selection : undefined

  // A failure in any read has already been reported by the API client's
  // interceptor; the placeholder is what keeps the dialog from presenting an
  // empty record that reads as "this member has none". Saving stays unavailable
  // until every record this caller may write has arrived.
  const accessReady =
    !props.canManageMemberModels ||
    (!accessQuery.isLoading && !accessQuery.isError && !accessQuery.isFetching)
  const discountsReady =
    !props.canManageMemberDiscounts ||
    (!discountsQuery.isLoading &&
      !discountsQuery.isError &&
      !discountsQuery.isFetching)
  const canSave = !isSaving && accessReady && discountsReady

  // The description states the rule of every block that renders: the
  // allow-list frames the discounts, and the quota block needs its own rule
  // whenever it is visible, even next to the allow-list sentence. The discount
  // rule — which travels with the member rather than the key — is the
  // fallback when neither block is there.
  const descriptionSentences: string[] = []
  if (props.canManageMemberModels) {
    descriptionSentences.push(
      t(
        'All models are available by default; select models to restrict this member. Clear the selection to remove the restriction.'
      )
    )
  }
  if (descriptionSentences.length === 0) {
    descriptionSentences.push(
      t(
        'Applies to all calls made by this member (including personal wallet); organization keys are attributed to their responsible member.'
      )
    )
  }
  const description = descriptionSentences.join(' ')

  const renderAccess = () => {
    if (accessQuery.isLoading) {
      return <p className='text-muted-foreground text-sm'>{t('Loading')}</p>
    }
    if (accessQuery.isError) {
      return <p className='text-destructive text-sm'>{t('Failed to load')}</p>
    }
    return (
      <MultiSelect
        options={storedAccess.models.map((model) => ({
          label: model,
          value: model,
        }))}
        selected={selection}
        onChange={setDraftAccess}
        placeholder={t('Select allowed models; leave empty for no limit')}
        disabled={isSaving}
      />
    )
  }

  const renderDiscounts = () => {
    if (discountsQuery.isLoading) {
      return <p className='text-muted-foreground text-sm'>{t('Loading')}</p>
    }
    if (discountsQuery.isError) {
      return <p className='text-destructive text-sm'>{t('Failed to load')}</p>
    }
    return (
      <>
        <ModelDiscountEditor
          value={discounts}
          modelOptions={modelOptions}
          lockedModels={lockedModels}
          disabled={isSaving}
          onChange={setDraftDiscounts}
        />
        {/* The allow-list frames the set, so where it is present the discount
            rule is stated here rather than in the dialog's own description. */}
        {props.canManageMemberModels ? (
          <p className='text-muted-foreground text-sm'>
            {t(
              'Applies to all calls made by this member (including personal wallet); organization keys are attributed to their responsible member.'
            )}
          </p>
        ) : null}
      </>
    )
  }

  const save = async () => {
    setIsSaving(true)
    try {
      // The two writes are one operator intent, but only the allow-list
      // decides what the discount record may look like, so it goes first.
      if (props.canManageMemberModels) {
        await updateOrganizationMemberModelAccess(
          props.organizationId,
          member.user_id,
          selection,
          surface
        )
        // The dialog stays mounted between members and the app keeps query data
        // fresh for 10s; without refreshing this record, reopening within that
        // window serves the pre-save list and the next save writes it back over
        // the change. Publish what was just saved, then re-read the server.
        queryClient.setQueryData(memberAccessQueryKey, {
          access: selection,
          models: storedAccess.models,
        })
        void queryClient.invalidateQueries({
          queryKey: memberAccessSharedQueryKey,
        })
      }
      if (props.canManageMemberDiscounts) {
        await updateOrganizationMemberModelDiscounts(
          props.organizationId,
          member.user_id,
          discounts,
          surface
        )
        queryClient.setQueryData(memberDiscountsQueryKey, discounts)
        void queryClient.invalidateQueries({
          queryKey: memberDiscountsQueryKey,
        })
      }
      toast.success(t('Operation successful'))
      props.onOpenChange(false)
    } catch {
      // The API client's interceptor already surfaces the failure as a toast,
      // and the dialog stays open with the edits for the operator to retry; a
      // write that already landed stays landed, so retrying is safe.
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
      title={t('Model Settings')}
      description={description}
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

      {props.canManageMemberModels ? (
        <div className='space-y-3'>
          <div className='text-sm font-medium'>{t('Model Access')}</div>
          {renderAccess()}
        </div>
      ) : null}

      {props.canManageMemberDiscounts ? (
        <div className='space-y-3'>{renderDiscounts()}</div>
      ) : null}

    </Dialog>
  )
}
