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

import { Button } from '@/components/ui/button'
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from '@/components/ui/card'
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
import { Textarea } from '@/components/ui/textarea'

import { updateOrganization } from '../api'
import {
  organizationFormSchema,
  ORGANIZATION_NAME_MAX_LENGTH,
  transformOrganizationToFormDefaults,
  type OrganizationFormValues,
} from '../lib'
import type { Organization } from '../types'
import { OrganizationSection, OrganizationSectionEmpty } from './organization-section'

type OrganizationSettingsSectionProps = {
  organization: Organization
  /** The caller may rename or re-describe the organization. */
  canUpdate: boolean
  /** Writes are unavailable; the form is not rendered at all. */
  readOnly: boolean
  /** Reload the detail payload after a successful save. */
  onUpdated: () => Promise<unknown> | unknown
}

/**
 * The organization profile: its name and description.
 *
 * Everything else about an organization is managed from where it belongs — the
 * group from the organization center's edit drawer, membership from the members
 * section, the status and the dissolution from the platform administration — so
 * this section stays a plain two-field form.
 */
export function OrganizationSettingsSection(
  props: OrganizationSettingsSectionProps
) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<OrganizationFormValues>({
    resolver: zodResolver(organizationFormSchema),
    defaultValues: transformOrganizationToFormDefaults(props.organization),
  })

  // The organization can be renamed elsewhere — by a platform administrator, or
  // by another owner in a second tab — and the section must show the stored
  // value rather than whatever was on screen when it mounted.
  useEffect(() => {
    form.reset(transformOrganizationToFormDefaults(props.organization))
  }, [form, props.organization])

  if (!props.canUpdate) {
    return (
      <OrganizationSectionEmpty
        icon='settings'
        title={t('Organization Settings')}
        message={t(
          'Only an organization owner or administrator can change these settings.'
        )}
      />
    )
  }

  const onSubmit = async (values: OrganizationFormValues) => {
    setIsSubmitting(true)
    try {
      const result = await updateOrganization(props.organization.id, {
        name: values.name.trim(),
        description: values.description?.trim() ?? '',
      })
      if (!result.success) return
      toast.success(t('Organization updated'))
      await props.onUpdated()
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <OrganizationSection
      icon='settings'
      title={t('Organization Settings')}
      description={t(
        'Rename the organization or change how it is described. The name must be unique.'
      )}
      scroll
    >
      <Card className='max-w-2xl'>
        <CardHeader>
          <CardTitle>{t('Profile')}</CardTitle>
          <CardDescription>
            {props.readOnly
              ? t('This organization is read-only, so the profile cannot be changed.')
              : t('Changes are recorded in the organization audit log.')}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <Form {...form}>
            <form
              onSubmit={form.handleSubmit(onSubmit)}
              className='flex flex-col gap-4'
            >
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input
                        {...field}
                        maxLength={ORGANIZATION_NAME_MAX_LENGTH}
                        autoComplete='off'
                        disabled={props.readOnly || isSubmitting}
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
                        disabled={props.readOnly || isSubmitting}
                      />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <div className='flex justify-end'>
                <Button type='submit' disabled={props.readOnly || isSubmitting}>
                  {isSubmitting ? t('Saving...') : t('Save')}
                </Button>
              </div>
            </form>
          </Form>
        </CardContent>
      </Card>
    </OrganizationSection>
  )
}
