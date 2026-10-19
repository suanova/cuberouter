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
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { usePricingData } from '@/features/pricing/hooks/use-pricing-data'
import { ModelDiscountEditor } from '@/features/users/components/model-discount-editor'
import {
  formatQuota,
  parseQuotaFromDollars,
  quotaUnitsToEditableAmount,
} from '@/lib/format'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

import {
  getOrganizationMemberModelAccess,
  getOrganizationMemberModelDiscounts,
  getOrganizationMemberQuota,
  updateOrganizationMemberModelAccess,
  updateOrganizationMemberModelDiscounts,
  updateOrganizationMemberQuota,
} from '../api'
import type {
  MemberQuota,
  OrganizationMemberRow,
  UserModelDiscounts,
} from '../types'
import { useOrganizationSurface } from './organization-page-provider'

/**
 * A member's settings, as the organization's own administrators set them:
 * which models the member may use, what they are charged for each, and how
 * much they may consume in total.
 *
 * They belong on one screen because they answer one question — what this
 * member is allowed to do, at what price, and up to what spend. The allow-list
 * is the honest frame for the discounts: restricting a member to three models
 * leaves nothing to say about the sixtieth, so the discount rows follow the
 * selection instead of offering the whole catalogue.
 *
 * The access list is a full replacement and the discounts are one too, so the
 * dialog opens on what the backend holds and stays unsavable until every
 * record this caller may write has landed — saving an unloaded record would
 * clear everything the member had. The writes go out on one save, the access
 * list first; the discounts carry the whole record rather than only the rows
 * on screen, so narrowing the list never deletes a discount kept for a model
 * the member can reach again later. The quota ceiling is independent of both:
 * a refusal there leaves the others landed and the dialog open to retry.
 */
export function OrganizationMemberModelSettingsDialog(props: {
  organizationId: number
  /** The row whose settings are being edited; null while the dialog is closed. */
  member: OrganizationMemberRow | null
  /** The caller may set the member's model allow-list. */
  canManageMemberModels: boolean
  /** The caller may set the member's model discounts. */
  canManageMemberDiscounts: boolean
  /** The caller may set the member's cumulative consumption limit. */
  canManageMemberQuotas: boolean
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const queryClient = useQueryClient()
  const member = props.member
  const [draftAccess, setDraftAccess] = useState<string[] | null>(null)
  const [draftDiscounts, setDraftDiscounts] =
    useState<UserModelDiscounts | null>(null)
  /**
   * The limit being typed, as the display amount the operator entered; null
   * until they edit it, and an empty string meaning "no limit". Keeping the
   * text rather than quota units is what lets a fractional amount survive
   * keystroke by keystroke — converting on every change would reload the field
   * from a rounded unit count.
   */
  const [draftQuota, setDraftQuota] = useState<string | null>(null)
  /**
   * The complaint about the amount currently in the quota field, if any. It is
   * set when a save is attempted with a value that cannot become a limit and
   * cleared as soon as the operator edits the field.
   */
  const [quotaAmountError, setQuotaAmountError] = useState<string | null>(null)
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
  const memberQuotaQueryKey = [
    'organization-member-quota',
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

  const quotaQuery = useQuery({
    queryKey: memberQuotaQueryKey,
    queryFn: () =>
      getOrganizationMemberQuota(
        props.organizationId,
        member?.user_id ?? 0,
        surface
      ),
    enabled: member !== null && props.canManageMemberQuotas,
  })

  // The dialog outlives a close (the section keeps it mounted and only drops
  // the row it holds), so edits left over from the previous visit must not be
  // shown for the next member. Every visit passes through the closed state, so
  // this resets the drafts once per opening.
  useEffect(() => {
    setDraftAccess(null)
    setDraftDiscounts(null)
    setDraftQuota(null)
    setQuotaAmountError(null)
  }, [member])

  if (!member) return null

  const storedAccess = accessQuery.data ?? { access: [], models: [] }
  const selection = draftAccess ?? storedAccess.access
  const discounts = draftDiscounts ?? discountsQuery.data ?? {}
  const storedQuota: MemberQuota = quotaQuery.data ?? {
    limit: 0,
    used: 0,
    organization_quota: 0,
    allocated: 0,
    over_allocated: false,
  }
  // What the stored ceiling reads as in an editable field: rounded to the
  // display precision so a unit count converted back does not surface as a
  // repeating fraction (CNY: 205,479 units reads as 3, not 2.9999934).
  const storedQuotaAmount =
    storedQuota.limit === 0
      ? ''
      : String(quotaUnitsToEditableAmount(storedQuota.limit))
  // What the organization can still promise: its quota minus every member
  // ceiling already configured, this member's stored one included.
  const allocatableQuota =
    storedQuota.organization_quota - storedQuota.allocated

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
  const quotaReady =
    !props.canManageMemberQuotas ||
    (!quotaQuery.isLoading && !quotaQuery.isError && !quotaQuery.isFetching)
  const canSave = !isSaving && accessReady && discountsReady && quotaReady

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
  if (props.canManageMemberQuotas) {
    descriptionSentences.push(
      t(
        'Set the total this member may consume inside the organization. Leave it empty for no limit.'
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

  const renderQuota = () => {
    if (quotaQuery.isLoading) {
      return <p className='text-muted-foreground text-sm'>{t('Loading')}</p>
    }
    if (quotaQuery.isError) {
      return <p className='text-destructive text-sm'>{t('Failed to load')}</p>
    }
    return (
      <>
        <div className='space-y-2'>
          <Label htmlFor='organization-member-quota-limit'>
            {t('Cumulative consumption limit')}
          </Label>
          <Input
            id='organization-member-quota-limit'
            // A number input discards text it cannot represent (1e999 becomes
            // an empty value), which this dialog would read as "no limit" — the
            // exact silent clearing the validation exists to prevent. Keeping
            // the raw text lets save() reject it by name instead.
            type='text'
            inputMode='decimal'
            autoComplete='off'
            placeholder={t('No limit')}
            // An empty field is "no limit"; the backend reads a zero limit the
            // same way, so both map to the one value it understands.
            value={draftQuota ?? storedQuotaAmount}
            onChange={(event) => {
              setDraftQuota(event.target.value)
              setQuotaAmountError(null)
            }}
            aria-invalid={quotaAmountError !== null}
            disabled={isSaving}
          />
          {quotaAmountError ? (
            <p className='text-destructive text-sm' role='alert'>
              {quotaAmountError}
            </p>
          ) : null}
          <p className='text-muted-foreground text-sm'>
            {t(
              "Counts this member's organization consumption only; personal wallet usage is not included."
            )}
          </p>
          <p className='text-muted-foreground text-sm'>
            {t(
              'Member-created keys use the remaining member quota, or 1% of the organization quota when no limit is set. A set member quota caps cumulative consumption across keys; the 1% default is a per-key cap.'
            )}
          </p>
        </div>

        <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm'>
          <dt className='text-muted-foreground'>{t('Used')}</dt>
          <dd>{formatQuota(storedQuota.used)}</dd>
          <dt className='text-muted-foreground'>{t('Limit')}</dt>
          <dd>
            {storedQuota.limit === 0
              ? t('No limit')
              : formatQuota(storedQuota.limit)}
          </dd>
          <dt className='text-muted-foreground'>
            {t('Remaining allocatable')}
          </dt>
          <dd>{formatQuota(allocatableQuota)}</dd>
        </dl>

        {storedQuota.over_allocated ? (
          <p className='text-destructive text-sm'>
            {t(
              "Member quota limits exceed the organization quota. Lower a member's limit or raise the organization quota before setting another."
            )}
          </p>
        ) : null}
      </>
    )
  }

  const save = async () => {
    // The ceiling is the only field whose text needs validation before it is
    // written, and it is checked before any write: a refused amount must not
    // land the other records and then strand the dialog open on it. null means
    // the operator never touched the field, so nothing about the ceiling is
    // sent at all — this save must not overwrite a concurrent limit change
    // with the value the dialog read when it opened.
    let quotaToSave: number | null = null
    if (props.canManageMemberQuotas && draftQuota !== null) {
      const typedText = draftQuota.trim()
      if (typedText === '') {
        // An emptied field means "no limit", which the backend reads as zero.
        quotaToSave = 0
      } else {
        const typedAmount = Number(typedText)
        if (!Number.isFinite(typedAmount) || typedAmount < 0) {
          setQuotaAmountError(
            t(
              'Enter a non-negative amount, or leave the field empty for no limit.'
            )
          )
          return
        }
        quotaToSave = parseQuotaFromDollars(typedAmount)
        // A positive amount that rounds down to zero units would silently mean
        // "no limit" — the opposite of a limit.
        if (quotaToSave === 0 && typedAmount > 0) {
          setQuotaAmountError(
            t(
              'The amount is too small to be a limit. Enter a larger amount or leave the field empty for no limit.'
            )
          )
          return
        }
      }
    }
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
      // The ceiling is independent of the two records above; it goes last so a
      // refusal leaves them landed and the dialog open on the edit to retry.
      if (props.canManageMemberQuotas && quotaToSave !== null) {
        await updateOrganizationMemberQuota(
          props.organizationId,
          member.user_id,
          quotaToSave,
          surface
        )
        queryClient.setQueryData(memberQuotaQueryKey, {
          ...storedQuota,
          limit: quotaToSave,
        })
        void queryClient.invalidateQueries({
          queryKey: memberQuotaQueryKey,
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
      title={t('Member Settings')}
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

      {props.canManageMemberQuotas ? (
        <div className='space-y-3'>
          <div className='text-sm font-medium'>{t('Quota Limit')}</div>
          {renderQuota()}
        </div>
      ) : null}
    </Dialog>
  )
}
