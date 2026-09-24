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
import { Loader2 } from 'lucide-react'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
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

import { createOrganizationInvite } from '../api'
import {
  ORGANIZATION_ASSIGNABLE_ROLES,
  organizationRoleLabelKey,
} from '../constants'
import { organizationInviteCreateFailureAction } from '../lib'

const CREATE_INVITE_FORM_ID = 'organization-create-invite-form'

const inviteSchema = z.object({
  email: z.email({ error: 'Enter an email address.' }),
  role: z.enum(['admin', 'member']),
})

type InviteValues = z.infer<typeof inviteSchema>

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
  const [pendingResend, setPendingResend] = useState<InviteValues | null>(null)

  const form = useForm<InviteValues>({
    resolver: zodResolver(inviteSchema),
    defaultValues: { email: '', role: 'member' },
  })

  const send = async (
    values: InviteValues,
    forceRotate: boolean
  ): Promise<SendOutcome> => {
    try {
      const result = await createOrganizationInvite(props.organizationId, {
        email: values.email.trim(),
        role: values.role,
        force_rotate: forceRotate,
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

  const handleOutcome = async (outcome: SendOutcome, values: InviteValues) => {
    switch (outcome.kind) {
      case 'sent':
        toast.success(t('Invitation sent'))
        props.onOpenChange(false)
        await props.onCreated()
        return
      case 'confirmResend':
        setPendingResend(values)
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
    setIsSubmitting(true)
    try {
      await handleOutcome(await send(values, false), values)
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleResend = async () => {
    if (!pendingResend) return
    setIsResending(true)
    try {
      const values = pendingResend
      await handleOutcome(await send(values, true), values)
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
          if (open) form.reset({ email: '', role: 'member' })
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
