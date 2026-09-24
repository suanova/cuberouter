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
import { UserCog } from 'lucide-react'
import { useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Skeleton } from '@/components/ui/skeleton'
import { Textarea } from '@/components/ui/textarea'

import {
  OrganizationSection,
  OrganizationSectionRefresh,
} from '@/features/organization/components/organization-section'
import { ERROR_MESSAGES } from '@/features/organization/constants'
import {
  listOrganizationMembers,
  transferOrganizationOwner,
} from '@/features/organization/api'
import {
  createIdempotencyKey,
  isOrganizationSlugConfirmed,
} from '@/features/organization/lib'
import { collectOrganizationRows } from '@/features/organization/lib/organization-pagination'
import type { OrganizationMemberRow } from '@/features/organization/types'

import {
  getPlatformOrganizationOwnerOptions,
  platformOrganizationOwnerTransferSchema,
  transformPlatformOrganizationOwnerTransfer,
  type PlatformOrganizationOwnerTransferValues,
} from '../lib'

type PlatformOrganizationOwnerRepairProps = {
  organizationId: number
  /** The slug the operator retypes before the transfer is accepted. */
  slug: string
  /** The member who owns the organization right now. */
  ownerUserId: number
  onTransferred: () => void
}

const EMPTY_VALUES: PlatformOrganizationOwnerTransferValues = {
  owner_user_id: 0,
  reason: '',
}

/**
 * Hands an organization to another of its members.
 *
 * This is the repair the platform side exists for: an organization whose owner
 * has left the company, lost their account or simply stopped answering cannot be
 * handed over by anyone inside it — the member-side transfer needs the current
 * owner to ask for it. The backend grants the platform root this operation
 * against any active membership for exactly that reason.
 *
 * The current owner is shown by name where the roster still holds them. An owner
 * whose membership was already disabled is the case this panel is most often
 * opened for, and they are precisely the one the picker cannot offer, so the
 * name is read from the whole roster rather than from the candidates.
 */
export function PlatformOrganizationOwnerRepair(
  props: PlatformOrganizationOwnerRepairProps
) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [confirmSlug, setConfirmSlug] = useState('')
  // One key per intent: a retry after a timeout has to replay the same request,
  // and a fresh intent after a completed transfer has to be a new one.
  const [idempotencyKey, setIdempotencyKey] = useState(createIdempotencyKey)

  const {
    data: roster = [],
    isLoading,
    isFetching,
    refetch,
  } = useQuery({
    queryKey: ['platform-organization-owner-roster', props.organizationId],
    queryFn: async (): Promise<OrganizationMemberRow[]> => {
      try {
        const collected = await collectOrganizationRows<OrganizationMemberRow>(
          async (page, pageSize) => {
            const result = await listOrganizationMembers(
              'admin',
              props.organizationId,
              { p: page, page_size: pageSize }
            )
            if (!result.success || !result.data) {
              return { page, page_size: pageSize, total: 0, items: [] }
            }
            return result.data
          }
        )
        return collected.items
      } catch {
        // The interceptor has already reported it; the picker stays empty
        // rather than offering a target the backend never confirmed.
        return []
      }
    },
  })

  const form = useForm<PlatformOrganizationOwnerTransferValues>({
    // The schema coerces the selected user id, so its input type is `unknown`
    // and the resolver has to be pinned to the parsed shape.
    resolver: zodResolver(
      platformOrganizationOwnerTransferSchema
    ) as unknown as Resolver<PlatformOrganizationOwnerTransferValues>,
    defaultValues: EMPTY_VALUES,
  })

  const currentOwner = roster.find(
    (member) => member.user_id === props.ownerUserId
  )
  const ownerOptions = getPlatformOrganizationOwnerOptions(
    roster,
    props.ownerUserId
  )
  // The trigger resolves its text from the items the root is given, so the list
  // is built once here and the options are drawn from it.
  const ownerItems = ownerOptions.map((option) => ({
    value: String(option.value),
    label: option.label,
  }))
  const isSlugConfirmed = isOrganizationSlugConfirmed(props.slug, confirmSlug)

  const onSubmit = async (values: PlatformOrganizationOwnerTransferValues) => {
    if (!isSlugConfirmed) return
    setIsSubmitting(true)
    try {
      const result = await transferOrganizationOwner(
        'admin',
        props.organizationId,
        transformPlatformOrganizationOwnerTransfer(values),
        idempotencyKey
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to transfer ownership'))
        return
      }
      toast.success(t('Ownership transferred'))
      setIdempotencyKey(createIdempotencyKey())
      setConfirmSlug('')
      form.reset(EMPTY_VALUES)
      props.onTransferred()
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
    <OrganizationSection
      icon='members'
      iconComponent={UserCog}
      title={t('Transfer Ownership')}
      description={t(
        'Hand the organization to another member. Used when its current owner can no longer act on its behalf.'
      )}
      actions={
        <OrganizationSectionRefresh
          onClick={() => void refetch()}
          isFetching={isFetching}
        />
      }
      scroll
    >
      <div className='flex max-w-2xl flex-col gap-6 pb-2'>
        <dl className='grid grid-cols-1 gap-3 rounded-xl border p-4 sm:grid-cols-3'>
          <div className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{t('Organization')}</dt>
            <dd className='truncate text-sm font-medium'>{props.slug}</dd>
          </div>
          <div className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{t('Current owner')}</dt>
            <dd className='truncate text-sm font-medium'>
              {isLoading ? (
                <Skeleton className='h-4 w-32' />
              ) : (
                formatMemberIdentity(currentOwner) || `#${props.ownerUserId}`
              )}
            </dd>
          </div>
          <div className='min-w-0'>
            <dt className='text-muted-foreground text-xs'>{t('New owner')}</dt>
            <dd className='truncate text-sm font-medium'>
              {formatMemberIdentity(
                roster.find(
                  (member) =>
                    member.user_id === form.watch('owner_user_id')
                )
              ) || '-'}
            </dd>
          </div>
        </dl>

        <Form {...form}>
          <form
            id='platform-organization-owner-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className='flex flex-col gap-5'
          >
            <FormField
              control={form.control}
              name='owner_user_id'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('New owner')}</FormLabel>
                  <FormControl>
                    <Select
                      items={ownerItems}
                      value={field.value > 0 ? String(field.value) : ''}
                      onValueChange={(value) =>
                        field.onChange(Number(value))
                      }
                    >
                      <SelectTrigger className='w-full'>
                        <SelectValue
                          placeholder={t('Select a member')}
                        />
                      </SelectTrigger>
                      <SelectContent>
                        {ownerItems.map((item) => (
                          <SelectItem key={item.value} value={item.value}>
                            {item.label}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Any active member can take over. Members whose account is switched off are not offered, because they could not sign in to act as owner.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='reason'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Reason')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      rows={3}
                      placeholder={t('Why is ownership being transferred?')}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Recorded in the organization audit log.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <div className='flex flex-col gap-2'>
              <Label htmlFor='platform-organization-owner-confirm-slug'>
                {t('Type the organization slug to confirm:')}{' '}
                <span className='font-semibold'>{props.slug}</span>
              </Label>
              <Input
                id='platform-organization-owner-confirm-slug'
                value={confirmSlug}
                onChange={(event) => setConfirmSlug(event.target.value)}
                autoComplete='off'
              />
            </div>

            <div>
              <Button
                type='submit'
                disabled={isSubmitting || !isSlugConfirmed}
              >
                {isSubmitting ? t('Transferring...') : t('Transfer ownership')}
              </Button>
            </div>
          </form>
        </Form>
      </div>
    </OrganizationSection>
  )
}

/** How a roster row is named wherever a member has to be identified. */
function formatMemberIdentity(
  member: OrganizationMemberRow | undefined
): string {
  if (!member) return ''
  const name = member.display_name || member.username
  if (name && member.email) return `${name} · ${member.email}`
  return name || member.email || ''
}
