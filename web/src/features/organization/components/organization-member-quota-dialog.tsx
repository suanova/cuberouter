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
import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

import {
  getOrganizationMemberQuota,
  updateOrganizationMemberQuota,
} from '../api'
import type { MemberQuota, OrganizationMemberRow } from '../types'
import { useOrganizationSurface } from './organization-page-provider'

/**
 * Adds to a member's cumulative consumption limit inside the organization.
 *
 * It has its own entry because it answers a different question from the model
 * settings: not what the member may call and at what price, but how much they
 * may spend in total. The operator enters an amount to add; the dialog sends
 * the resulting absolute limit. A member with no limit (0) starts at the amount
 * added — the first grant turns "unlimited" into a concrete ceiling — and a
 * later grant raises that ceiling by the same rule.
 */
export function OrganizationMemberQuotaDialog(props: {
  organizationId: number
  /** The row whose quota is being edited; null while the dialog is closed. */
  member: OrganizationMemberRow | null
  onOpenChange: (open: boolean) => void
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const queryClient = useQueryClient()
  const member = props.member
  /**
   * The amount being added, as the display amount the operator entered; null
   * until they edit it. Keeping the text rather than quota units is what lets
   * a fractional amount survive keystroke by keystroke — converting on every
   * change would reload the field from a rounded unit count.
   */
  const [draftQuota, setDraftQuota] = useState<string | null>(null)
  /**
   * The complaint about the amount currently in the field, if any. It is set
   * when a save is attempted with a value that cannot become a limit and
   * cleared as soon as the operator edits the field.
   */
  const [quotaAmountError, setQuotaAmountError] = useState<string | null>(null)
  const [isSaving, setIsSaving] = useState(false)

  const memberQuotaQueryKey = [
    'organization-member-quota',
    surface,
    props.organizationId,
    member?.user_id,
    getAccountContextCacheKey(),
  ]
  const quotaQuery = useQuery({
    queryKey: memberQuotaQueryKey,
    queryFn: () =>
      getOrganizationMemberQuota(
        props.organizationId,
        member?.user_id ?? 0,
        surface
      ),
    enabled: member !== null,
  })

  // The dialog outlives a close (the section keeps it mounted and only drops
  // the row it holds), so edits left over from the previous visit must not be
  // shown for the next member.
  useEffect(() => {
    setDraftQuota(null)
    setQuotaAmountError(null)
  }, [member])

  if (!member) return null

  const storedQuota: MemberQuota = quotaQuery.data ?? {
    limit: 0,
    used: 0,
    organization_quota: 0,
    allocated: 0,
    over_allocated: false,
  }
  // What the organization can still promise: its quota minus every member
  // ceiling already configured, this member's stored one included.
  const allocatableQuota =
    storedQuota.organization_quota - storedQuota.allocated
  const typedAmount = Number((draftQuota ?? '').trim())
  const hasAmount = (draftQuota ?? '').trim() !== ''
  const addedUnits =
    Number.isFinite(typedAmount) && typedAmount > 0
      ? parseQuotaFromDollars(typedAmount)
      : 0
  // A member with no limit (0) starts at the amount added; an existing ceiling
  // is raised by it.
  const limitAfterAdding = storedQuota.limit + addedUnits
  const quotaReady =
    !quotaQuery.isLoading && !quotaQuery.isError && !quotaQuery.isFetching
  const canSave = !isSaving && quotaReady && hasAmount

  const renderQuota = () => {
    if (quotaQuery.isLoading) {
      return <p className='text-muted-foreground text-sm'>{t('Loading')}</p>
    }
    if (quotaQuery.isError) {
      return <p className='text-destructive text-sm'>{t('Failed to load')}</p>
    }
    return (
      <>
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

        <div className='space-y-2'>
          <Label htmlFor='organization-member-quota-limit'>
            {t('Amount to add')}
          </Label>
          <Input
            id='organization-member-quota-limit'
            // A number input discards text it cannot represent (1e999 becomes
            // an empty value), which this dialog would read as no change — the
            // exact silent no-op the validation exists to prevent. Keeping the
            // raw text lets save() reject it by name instead.
            type='text'
            inputMode='decimal'
            autoComplete='off'
            placeholder={t('Enter an amount')}
            value={draftQuota ?? ''}
            onChange={(event) => {
              setDraftQuota(event.target.value)
              setQuotaAmountError(null)
            }}
            aria-invalid={quotaAmountError !== null}
            disabled={isSaving}
          />
          <div className='flex items-center justify-between text-sm'>
            <span className='text-muted-foreground'>
              {t('Limit after adding')}
            </span>
            <span className='font-medium'>
              {formatQuota(limitAfterAdding)}
            </span>
          </div>
          {quotaAmountError ? (
            <p className='text-destructive text-sm' role='alert'>
              {quotaAmountError}
            </p>
          ) : null}
        </div>
      </>
    )
  }

  const save = async () => {
    const addedAmount = Number((draftQuota ?? '').trim())
    if (!Number.isFinite(addedAmount) || addedAmount <= 0) {
      setQuotaAmountError(t('Enter an amount greater than zero.'))
      return
    }
    const addedQuota = parseQuotaFromDollars(addedAmount)
    // A positive amount that rounds down to zero units would silently add
    // nothing.
    if (addedQuota === 0) {
      setQuotaAmountError(
        t('The amount is too small to add. Enter a larger amount.')
      )
      return
    }
    const nextLimit = storedQuota.limit + addedQuota

    setIsSaving(true)
    try {
      await updateOrganizationMemberQuota(
        props.organizationId,
        member.user_id,
        nextLimit,
        surface
      )
      queryClient.setQueryData(memberQuotaQueryKey, {
        ...storedQuota,
        limit: nextLimit,
        allocated: storedQuota.allocated + addedQuota,
      })
      void queryClient.invalidateQueries({ queryKey: memberQuotaQueryKey })
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
      title={t('Add quota')}
      description={t(
        'Add to this member\'s cumulative consumption limit. An unlimited member starts at the amount added.'
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
            {isSaving ? t('Saving...') : t('Add')}
          </Button>
        </>
      }
    >
      <div className='space-y-1'>
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
        <dt className='text-muted-foreground'>{t('Username')}</dt>
        <dd className='truncate'>{member.username || '-'}</dd>
        <dt className='text-muted-foreground'>{t('Email')}</dt>
        <dd className='truncate'>{member.email || '-'}</dd>
      </dl>

      {renderQuota()}
    </Dialog>
  )
}
