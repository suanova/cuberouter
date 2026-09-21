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
import { useForm } from 'react-hook-form'
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
import { ApiKeyGroupCombobox } from '@/features/keys/components/api-key-group-combobox'
import { refreshAccountContexts } from '@/lib/account-context'

import { getOrganizationGroups, updateOrganization } from '../api'
import { ERROR_MESSAGES, SUCCESS_MESSAGES } from '../constants'
import {
  buildOrganizationGroupOptions,
  organizationFormSchema,
  ORGANIZATION_FORM_DEFAULT_VALUES,
  ORGANIZATION_NAME_MAX_LENGTH,
  transformOrganizationFormToPayload,
  transformOrganizationToFormDefaults,
  type OrganizationFormValues,
} from '../lib'
import type { UserOrganization } from '../types'
import { useOrganizations } from './organizations-provider'

type OrganizationsEditDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  organization?: UserOrganization
}

/**
 * Editing an organization from inside it.
 *
 * There is no create here: only platform administrators may create an
 * organization, and they do it from the platform page, not from the
 * organization center. This drawer edits an organization the caller already
 * belongs to.
 */
export function OrganizationsEditDrawer({
  open,
  onOpenChange,
  organization,
}: OrganizationsEditDrawerProps) {
  const { t } = useTranslation()
  const { triggerRefresh } = useOrganizations()
  const [isSubmitting, setIsSubmitting] = useState(false)

  // Moving an organization to another group is a platform-level decision, so
  // only the members who hold that capability see the picker at all.
  const canChangeGroup = Boolean(
    organization?.capabilities.can_modify_organization_group
  )
  const organizationId = organization?.id ?? 0
  const { data: groupsData } = useQuery({
    queryKey: ['organization-groups', organizationId],
    queryFn: () => getOrganizationGroups(organizationId),
    enabled: open && canChangeGroup && organizationId > 0,
    staleTime: 5 * 60 * 1000,
  })
  const groupOptions = buildOrganizationGroupOptions(groupsData?.data)

  const form = useForm<OrganizationFormValues>({
    resolver: zodResolver(organizationFormSchema),
    defaultValues: ORGANIZATION_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (!open) return
    form.reset(
      organization
        ? transformOrganizationToFormDefaults(organization)
        : ORGANIZATION_FORM_DEFAULT_VALUES
    )
  }, [open, organization, form])

  const onSubmit = async (values: OrganizationFormValues) => {
    if (!organization) return
    setIsSubmitting(true)
    try {
      const payload = transformOrganizationFormToPayload(values, {
        organizationId: organization.id,
        includeGroup: canChangeGroup,
      })
      const result = await updateOrganization(
        'member',
        organization.id,
        payload
      )
      if (!result.success) {
        toast.error(result.message || t(ERROR_MESSAGES.UPDATE_FAILED))
        return
      }
      toast.success(t(SUCCESS_MESSAGES.UPDATED))
      onOpenChange(false)
      // Name and group both feed the account-context switcher.
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
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[540px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Edit Organization')}</SheetTitle>
          <SheetDescription>
            {t('Update the organization name, description and group.')}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='organization-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName()}
          >
            <SideDrawerSection>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        placeholder={t('Enter organization name')}
                        maxLength={ORGANIZATION_NAME_MAX_LENGTH}
                        autoComplete='off'
                      />
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

              {canChangeGroup && (
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
              )}

              <FormField
                control={form.control}
                name='reason'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Reason')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        value={field.value ?? ''}
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
            form='organization-form'
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
