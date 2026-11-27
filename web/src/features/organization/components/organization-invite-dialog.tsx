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
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery } from '@tanstack/react-query'
import { ChevronDown, Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { MultiSelect } from '@/components/multi-select'
import { Button } from '@/components/ui/button'
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from '@/components/ui/collapsible'
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormDescription,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { formatQuota, parseQuotaFromDollars } from '@/lib/format'

import {
  createOrganizationInvite,
  getOrganizationMemberRestrictionOptions,
} from '../api'
import {
  ORGANIZATION_ASSIGNABLE_ROLES,
  organizationRoleLabelKey,
} from '../constants'
import { organizationInviteCreateFailureAction } from '../lib'

const CREATE_INVITE_FORM_ID = 'organization-create-invite-form'

const inviteSchema = z.object({
  email: z.email({ error: 'Enter an email address.' }),
  role: z.enum(['admin', 'member']),
  models: z.array(z.string()),
  quota_amount: z.string(),
})

type InviteValues = z.infer<typeof inviteSchema>

type PendingResend = {
  values: InviteValues
  quotaLimit: number | undefined
}

/**
 * What came back from a send attempt, once the failure modes the backend
 * distinguishes have been resolved into the *handling* each one needs.
 *
 * A delivery failure and an already-sent invitation are both refusals, but one
 * means "the record exists, re-read the list" and the other means "ask whether
 * to send another". Collapsing them into a message would lose that difference.
 */
type SendOutcome =
  | { kind: 'sent' }
  | { kind: 'confirmResend' }
  | { kind: 'deliveryFailure'; messageKey: string }
  | { kind: 'knownDeliveryFailure'; messageKey: string }
  | { kind: 'userAlreadyJoined' }
  | { kind: 'failed'; message: string }

type OrganizationInviteDialogProps = {
  organizationId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onCreated: () => Promise<unknown> | unknown
  /** The caller may set the member's model allow-list. */
  canManageMemberModels: boolean
  /** The caller may set the member's quota ceiling. */
  canManageMemberQuotas: boolean
}

/**
 * Invites someone by email.
 *
 * The send is the side effect and the record is written first, so a failure
 * after that point leaves a pending invitation behind: the list the operator
 * came from is stale even though the send failed, and this dialog refreshes it
 * rather than letting them carry on looking at a list that is no longer true.
 */
export function OrganizationInviteDialog(props: OrganizationInviteDialogProps) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isResending, setIsResending] = useState(false)
  const [pendingResend, setPendingResend] = useState<PendingResend | null>(null)
  const [quotaAmountError, setQuotaAmountError] = useState<string | null>(null)

  const form = useForm<InviteValues>({
    resolver: zodResolver(inviteSchema),
    defaultValues: { email: '', role: 'member', models: [], quota_amount: '' },
  })

  // The parent opens the dialog by flipping `open`, not by calling
  // onOpenChange, so the reset cannot live in that callback: a reopened dialog
  // would otherwise keep the previous invitation's email, role and
  // restrictions.
  useEffect(() => {
    if (!props.open) return
    form.reset({ email: '', role: 'member', models: [], quota_amount: '' })
    setQuotaAmountError(null)
    setPendingResend(null)
  }, [props.open, form])

  const restrictionOptionsQuery = useQuery({
    queryKey: ['organization-member-restriction-options', props.organizationId],
    queryFn: () =>
      getOrganizationMemberRestrictionOptions(props.organizationId),
    enabled:
      props.open &&
      (props.canManageMemberModels || props.canManageMemberQuotas),
  })

  const resolveQuotaLimit = (
    amountText: string
  ): { limit: number } | { error: string } => {
    const trimmed = amountText.trim()
    if (trimmed === '') {
      return { limit: 0 }
    }
    const amount = Number(trimmed)
    if (!Number.isFinite(amount) || amount < 0) {
      return { error: t('Enter a positive amount or leave it empty.') }
    }
    const limit = parseQuotaFromDollars(amount)
    if (amount > 0 && limit === 0) {
      return {
        error: t('The amount is too small to set. Enter a larger amount.'),
      }
    }
    const options = restrictionOptionsQuery.data
    if (
      limit > 0 &&
      options &&
      limit > options.organization_quota - options.allocated
    ) {
      return {
        error: t('The quota limit exceeds the remaining allocatable quota.'),
      }
    }
    return { limit }
  }

  let budgetHint: string | null = null
  if (restrictionOptionsQuery.isLoading) {
    budgetHint = t('Loading')
  } else if (restrictionOptionsQuery.isError) {
    budgetHint = t('Failed to load')
  } else if (restrictionOptionsQuery.data) {
    budgetHint = `${t('Remaining allocatable')}: ${formatQuota(
      restrictionOptionsQuery.data.organization_quota -
        restrictionOptionsQuery.data.allocated
    )}`
  }

  const send = async (
    values: InviteValues,
    forceRotate: boolean,
    quotaLimit: number | undefined
  ): Promise<SendOutcome> => {
    try {
      const result = await createOrganizationInvite(props.organizationId, {
        email: values.email.trim(),
        role: values.role,
        force_rotate: forceRotate,
        ...(props.canManageMemberModels ? { models: values.models } : {}),
        ...(props.canManageMemberQuotas && quotaLimit !== undefined
          ? { quota_limit: quotaLimit }
          : {}),
      })
      // `skipErrorHandler` keeps a non-2xx out of the interceptor, but a 2xx
      // carrying `success: false` is still a refusal.
      if (!result.success) {
        return {
          kind: 'failed',
          message: result.message || t('Failed to create the invitation'),
        }
      }
      return { kind: 'sent' }
    } catch (error) {
      const failure = organizationInviteCreateFailureAction(error)
      switch (failure.action) {
        case 'deliveryFailure':
          return { kind: 'deliveryFailure', messageKey: failure.messageKey }
        case 'knownDeliveryFailure':
          return {
            kind: 'knownDeliveryFailure',
            messageKey: failure.messageKey,
          }
        case 'userAlreadyJoined':
          return { kind: 'userAlreadyJoined' }
        case 'confirmResend':
          return { kind: 'confirmResend' }
        default:
          return {
            kind: 'failed',
            message: t('Failed to create the invitation'),
          }
      }
    }
  }

  const handleOutcome = async (outcome: SendOutcome, resend: PendingResend) => {
    switch (outcome.kind) {
      case 'sent':
        toast.success(t('Invitation sent'))
        props.onOpenChange(false)
        await props.onCreated()
        return
      case 'confirmResend':
        setPendingResend(resend)
        return
      case 'deliveryFailure':
        // The invitation was recorded and the mail did not go out, so the
        // dialog closes onto a list that has to be re-read.
        props.onOpenChange(false)
        toast.error(t(outcome.messageKey))
        await props.onCreated()
        return
      case 'knownDeliveryFailure':
        toast.error(t(outcome.messageKey))
        return
      case 'userAlreadyJoined':
        toast.error(t('The user has already joined this organization'))
        return
      default:
        toast.error(outcome.message)
    }
  }

  const onSubmit = async (values: InviteValues) => {
    let quotaLimit: number | undefined
    if (props.canManageMemberQuotas) {
      const resolved = resolveQuotaLimit(values.quota_amount)
      if ('error' in resolved) {
        setQuotaAmountError(resolved.error)
        return
      }
      quotaLimit = resolved.limit
    }
    setQuotaAmountError(null)
    setIsSubmitting(true)
    try {
      await handleOutcome(await send(values, false, quotaLimit), {
        values,
        quotaLimit,
      })
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleResend = async () => {
    if (!pendingResend) return
    setIsResending(true)
    try {
      await handleOutcome(
        await send(pendingResend.values, true, pendingResend.quotaLimit),
        pendingResend
      )
    } finally {
      setIsResending(false)
      setPendingResend(null)
    }
  }

  const roleItems = ORGANIZATION_ASSIGNABLE_ROLES.map((role) => ({
    value: role,
    label: t(organizationRoleLabelKey(role)),
  }))

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={(open) => {
          props.onOpenChange(open)
        }}
        title={t('Invite member')}
        description={t(
          'The recipient joins this organization by following the link in the email. The link expires on its own.'
        )}
        bodyClassName='space-y-4'
        footer={
          <>
            <Button
              type='button'
              variant='outline'
              onClick={() => props.onOpenChange(false)}
              disabled={isSubmitting}
            >
              {t('Cancel')}
            </Button>
            <Button
              type='submit'
              form={CREATE_INVITE_FORM_ID}
              disabled={isSubmitting}
            >
              {isSubmitting ? <Loader2 className='animate-spin' /> : null}
              {isSubmitting ? t('Sending...') : t('Send invitation')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={CREATE_INVITE_FORM_ID}
            onSubmit={form.handleSubmit(onSubmit)}
            className='space-y-4'
          >
            <FormField
              control={form.control}
              name='email'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Email')}</FormLabel>
                  <FormControl>
                    <Input
                      type='email'
                      placeholder={t('name@example.com')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('The invitation is only valid for this address.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='role'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Role')}</FormLabel>
                  <Select
                    items={roleItems}
                    value={field.value}
                    onValueChange={field.onChange}
                  >
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {roleItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormMessage />
                </FormItem>
              )}
            />

            {props.canManageMemberModels || props.canManageMemberQuotas ? (
              <Collapsible>
                <CollapsibleTrigger
                  type='button'
                  className='hover:bg-muted/50 flex w-full items-center justify-between rounded-lg border px-4 py-3 text-sm font-medium transition-colors'
                >
                  <span>{t('Model and quota restrictions')}</span>
                  <ChevronDown
                    className='text-muted-foreground size-4'
                    aria-hidden='true'
                  />
                </CollapsibleTrigger>
                <CollapsibleContent className='space-y-4 pt-3'>
                  {props.canManageMemberModels ? (
                    <FormField
                      control={form.control}
                      name='models'
                      render={({ field }) => (
                        <FormItem>
                          <div className='text-sm font-medium'>
                            {t('Model Access')}
                          </div>
                          <MultiSelect
                            options={(
                              restrictionOptionsQuery.data?.models ?? []
                            ).map((model) => ({ label: model, value: model }))}
                            selected={field.value}
                            onChange={field.onChange}
                            placeholder={t(
                              'Select allowed models; leave empty for no limit'
                            )}
                            disabled={isSubmitting || isResending}
                          />
                          <FormDescription>
                            {t(
                              'All models are available by default; select models to restrict this member. Clear the selection to remove the restriction.'
                            )}
                          </FormDescription>
                        </FormItem>
                      )}
                    />
                  ) : null}

                  {props.canManageMemberQuotas ? (
                    <FormField
                      control={form.control}
                      name='quota_amount'
                      render={({ field }) => (
                        <FormItem>
                          <FormLabel>{t('Quota limit')}</FormLabel>
                          <FormControl>
                            <Input
                              type='text'
                              inputMode='decimal'
                              autoComplete='off'
                              placeholder={t(
                                'Enter a quota limit; leave empty for no limit.'
                              )}
                              {...field}
                              onChange={(event) => {
                                field.onChange(event)
                                setQuotaAmountError(null)
                              }}
                              aria-invalid={quotaAmountError !== null}
                            />
                          </FormControl>
                          {budgetHint ? (
                            <FormDescription>{budgetHint}</FormDescription>
                          ) : null}
                          {restrictionOptionsQuery.data?.over_allocated ? (
                            <p className='text-destructive text-sm'>
                              {t(
                                "Member quota limits exceed the organization quota. Lower a member's limit or raise the organization quota before setting another."
                              )}
                            </p>
                          ) : null}
                          {quotaAmountError ? (
                            <p
                              className='text-destructive text-sm'
                              role='alert'
                            >
                              {quotaAmountError}
                            </p>
                          ) : null}
                        </FormItem>
                      )}
                    />
                  ) : null}

                  <p className='text-muted-foreground text-sm'>
                    {t(
                      'These are the initial restrictions when the member joins. They can be changed later.'
                    )}
                  </p>
                </CollapsibleContent>
              </Collapsible>
            ) : null}
          </form>
        </Form>
      </Dialog>

      <ConfirmDialog
        open={pendingResend !== null}
        onOpenChange={(open) => !open && setPendingResend(null)}
        title={t('Send another invitation?')}
        desc={t(
          'An invitation for this address is already pending. Sending another one replaces the link that is already in their inbox.'
        )}
        confirmText={t('Send invitation')}
        isLoading={isResending}
        handleConfirm={() => void handleResend()}
      />
    </>
  )
}
