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
import { createOrganization } from '@/features/organization/api'
import {
  ERROR_MESSAGES,
  SUCCESS_MESSAGES,
} from '@/features/organization/constants'
import {
  organizationFormSchema,
  ORGANIZATION_FORM_DEFAULT_VALUES,
  ORGANIZATION_NAME_MAX_LENGTH,
  transformOrganizationFormToPayload,
  type OrganizationFormValues,
} from '@/features/organization/lib'
import { getServerErrorMessageKey } from '@/lib/server-error-message'

type PlatformOrganizationCreateDrawerProps = {
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The organization exists; whatever is showing the list is stale. */
  onCreated: (organizationId: number) => void
}

/**
 * Creates an organization from the platform side.
 *
 * Only the name and the description: a new organization always starts on the
 * default group, with no quota, and the create endpoint accepts neither a group
 * nor a reason. Those are the edit drawer's, once there is something to edit.
 *
 * The creator becomes the owner, so this drawer is the only way an organization
 * comes into being — the organization center has no create entry, because an
 * ordinary member may not create one.
 */
export function PlatformOrganizationCreateDrawer(
  props: PlatformOrganizationCreateDrawerProps
) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<OrganizationFormValues>({
    resolver: zodResolver(organizationFormSchema),
    defaultValues: ORGANIZATION_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (!props.open) return
    form.reset(ORGANIZATION_FORM_DEFAULT_VALUES)
  }, [props.open, form])

  // A rejected create comes back as a stable code plus an English message. The
  // code is what the operator should read, in their own language; the message is
  // only useful when there is no code to look up.
  const createFailureMessage = (failure: unknown, fallback: string) => {
    const messageKey = getServerErrorMessageKey(failure)
    return messageKey ? t(messageKey) : fallback
  }

  const onSubmit = async (values: OrganizationFormValues) => {
    setIsSubmitting(true)
    try {
      // The payload carries only what the create endpoint reads; `group` is for
      // updates, and there is no reason to give for a creation.
      const result = await createOrganization(
        transformOrganizationFormToPayload(values),
        { skipErrorHandler: true }
      )
      if (!result.success || !result.data) {
        toast.error(
          createFailureMessage(
            result,
            result.message || t(ERROR_MESSAGES.CREATE_FAILED)
          )
        )
        return
      }
      toast.success(t(SUCCESS_MESSAGES.CREATED))
      props.onOpenChange(false)
      props.onCreated(result.data.id)
    } catch (error) {
      // A rejected create is a typed 4xx — the name is taken, or the creator is
      // at their organization limit. The drawer stays open so the name can be
      // corrected in place.
      toast.error(
        createFailureMessage(
          error,
          error instanceof Error && error.message
            ? error.message
            : t(ERROR_MESSAGES.UNEXPECTED)
        )
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={props.open} onOpenChange={props.onOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-[540px]')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>{t('Create Organization')}</SheetTitle>
          <SheetDescription>
            {t(
              'Creating an organization makes you its owner. Others can be invited afterwards.'
            )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='platform-organization-create-form'
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
            </SideDrawerSection>
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <SheetClose render={<Button variant='outline' />}>
            {t('Cancel')}
          </SheetClose>
          <Button
            form='platform-organization-create-form'
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
