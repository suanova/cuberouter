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
import { useEffect, useRef } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Textarea } from '@/components/ui/textarea'

import { SettingsForm } from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const OPTION_KEY = 'security_setting.swagger_api_ip_whitelist'

const whitelistSchema = z.object({
  whitelist: z.string(),
})

type WhitelistFormValues = z.output<typeof whitelistSchema>
type WhitelistFormInput = z.input<typeof whitelistSchema>

type ApiDocsWhitelistSectionProps = {
  defaultValues: {
    whitelist: string[]
  }
}

const splitLines = (value: string) =>
  value
    .split('\n')
    .map((entry) => entry.trim())
    .filter(Boolean)

export function ApiDocsWhitelistSection(props: ApiDocsWhitelistSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const baselineRef = useRef<string[]>(props.defaultValues.whitelist)

  const form = useForm<WhitelistFormInput, unknown, WhitelistFormValues>({
    resolver: zodResolver(whitelistSchema),
    defaultValues: {
      whitelist: props.defaultValues.whitelist.join('\n'),
    },
  })

  useEffect(() => {
    baselineRef.current = props.defaultValues.whitelist
    form.reset({ whitelist: props.defaultValues.whitelist.join('\n') })
  }, [props.defaultValues.whitelist, form])

  const onSubmit = async (data: WhitelistFormValues) => {
    const whitelist = splitLines(data.whitelist)
    if (JSON.stringify(whitelist) === JSON.stringify(baselineRef.current)) {
      toast.info(t('No changes to save'))
      return
    }
    // Backend rejects non-IP/CIDR entries and shows the reason in a toast;
    // only move the baseline after the server accepted the new list.
    const result = await updateOption.mutateAsync({
      key: OPTION_KEY,
      value: JSON.stringify(whitelist),
    })
    if (!result.success) return
    baselineRef.current = whitelist
  }

  return (
    <SettingsSection title={t('API Access Whitelist')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save API access whitelist'
          />
          <FormField
            control={form.control}
            name='whitelist'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Allowed IPs / CIDRs')}</FormLabel>
                <FormControl>
                  <Textarea
                    placeholder={t('203.0.113.10&#10;10.0.0.0/8')}
                    rows={4}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Only these client IPs may access the Swagger documentation and the /api/v2 API. One IP or CIDR range per line; leave empty to allow all clients.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
