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
import { useEffect, useState } from 'react'
import { useForm, type Resolver } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  SideDrawerSection,
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
} from '@/components/drawer-layout'
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
  Sheet,
  SheetClose,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '@/components/ui/sheet'
import { Textarea } from '@/components/ui/textarea'
import { formatQuota } from '@/lib/format'

import { ApiKeyGroupCombobox } from '@/features/keys/components/api-key-group-combobox'
import {
  adjustOrganizationQuota,
  getOrganizationGroupSource,
  updateOrganization,
} from '@/features/organization/api'
import { buildOrganizationGroupOptions } from '@/features/organization/lib'

import {
  platformOrganizationEditSchema,
  transformPlatformOrganizationEditToRequests,
  transformPlatformOrganizationToFormDefaults,
  type PlatformOrganizationEditTarget,
  type PlatformOrganizationEditValues,
} from '../lib'

type PlatformOrganizationEditDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  organization: PlatformOrganizationEditTarget | null
  /** The organization changed and whatever is showing it is stale. */
  onSaved: () => void
}

const EMPTY_VALUES: PlatformOrganizationEditValues = {
  name: '',
  description: '',
  group: 'default',
  remain_quota: 0,
  reason: '',
}

/**
 * Edits an organization from the platform side.
 *
 * One save can be two requests: the organization's own fields, and — only when
 * the remaining quota actually moved — a quota adjustment. They are separate
 * because the backend gates them with separate capabilities and records them as
 * separate audit entries, and because folding a no-op adjustment into every save
 * would fill the organization's ledger with rows that record nothing.
 *
 * A failed adjustment does not close the drawer. The edit has already landed at
 * that point, and closing would report both as done; leaving the drawer open
 * lets the operator retry, which is safe because the adjustment's idempotency
 * key is derived from the request's own contents.
 */
export function PlatformOrganizationEditDrawer(
  props: PlatformOrganizationEditDrawerProps
) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const organization = props.organization

  const { data: groupsData } = useQuery({
    queryKey: ['organization-group-source', 'admin', organization?.id ?? 0],
    queryFn: () => getOrganizationGroupSource('admin', organization?.id ?? 0),
    enabled: props.open && Boolean(organization),
    staleTime: 5 * 60 * 1000,
  })
  const groupOptions = buildOrganizationGroupOptions(groupsData?.data)

  const form = useForm<PlatformOrganizationEditValues>({
    // The schema's input type is `unknown` wherever it coerces (the quota field
    // arrives from the number input as a string), so the resolver has to be
    // pinned to the parsed shape the submit handler actually receives.
    resolver: zodResolver(
      platformOrganizationEditSchema
    ) as unknown as Resolver<PlatformOrganizationEditValues>,
    defaultValues: EMPTY_VALUES,
  })

  useEffect(() => {
    if (!props.open || !organization) return
    form.reset(transformPlatformOrganizationToFormDefaults(organization))
  }, [props.open, organization, form])

  if (!organization) return null

  const onSubmit = async (values: PlatformOrganizationEditValues) => {
    setIsSubmitting(true)
    try {
      const { edit, quotaDelta } = transformPlatformOrganizationEditToRequests(
        values,
        organization
      )

      const saved = await updateOrganization('admin', organization.id, edit)
      if (!saved.success) {
        toast.error(saved.message || t('Failed to update the organization'))
        return
      }

      if (quotaDelta !== null) {
        const adjustment = await adjustOrganizationQuota(organization.id, {
          quota_delta: quotaDelta,
          reason: edit.reason,
        })
        if (!adjustment.success) {
          toast.error(
            adjustment.message ||
              t('The organization was saved, but the quota adjustment failed')
          )
          // The fields did change, so whatever shows the organization is stale
          // whatever happened to the adjustment.
          props.onSaved()
          return
        }
      }

      toast.success(t('Organization updated'))
      props.onOpenChange(false)
      props.onSaved()
    } catch (error) {
      toast.error(
        error instanceof Error && error.message
          ? error.message
          : t('An unexpected error occurred')
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[560px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Edit Organization')}</SheetTitle>
          <SheetDescription>
            {t(
              'Change the organization name, group or quota. The change is recorded in its audit log.'
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='platform-organization-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              {/* Read-only identity: neither can be changed once the
                  organization exists — the slug is what the operator retypes to
                  confirm destructive actions, so it may not move under them. */}
              <div className='grid grid-cols-2 gap-3'>
                <div className='grid gap-2'>
                  <Label htmlFor='platform-organization-id'>{t('ID')}</Label>
                  <Input
                    id='platform-organization-id'
                    value={organization.id}
                    readOnly
                    disabled
                  />
                </div>
                <div className='grid gap-2'>
                  <Label htmlFor='platform-organization-slug'>{t('Slug')}</Label>
                  <Input
                    id='platform-organization-slug'
                    value={organization.slug}
                    readOnly
                    disabled
                  />
                </div>
              </div>

              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input {...field} autoComplete='off' />
                    </FormControl>
                    <FormDescription>
                      {t('The name must be unique across all organizations.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Description')}</FormLabel>
                    <FormControl>
                      <Textarea
                        {...field}
                        value={field.value ?? ''}
                        rows={3}
                        placeholder={t('What is this organization for?')}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='group'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Group')}</FormLabel>
                    <FormControl>
                      <ApiKeyGroupCombobox
                        options={groupOptions}
                        value={field.value}
                        onValueChange={field.onChange}
                        placeholder={t('Select a group')}
                        size='compact'
                      />
                    </FormControl>
                    <FormDescription>
                      {t(
                        'Requests made with this organization use the selected group.'
                      )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='remain_quota'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Remaining quota')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        type='number'
                        min={0}
                        step={1}
                        value={field.value ?? 0}
                        autoComplete='off'
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Quota units, worth {{amount}}.', {
                        amount: formatQuota(Number(field.value) || 0),
                      })}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <p className='text-muted-foreground text-xs'>
                {t('Current grant:')} {formatQuota(organization.quota)} ·{' '}
                {t('Used:')} {formatQuota(organization.used_quota)}
              </p>

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
                        placeholder={t('Why is this change being made?')}
                      />
                    </FormControl>
                    <FormDescription>
                      {t('Recorded in the organization audit log.')}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            form='platform-organization-form'
            type='submit'
            disabled={isSubmitting}
          >
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
