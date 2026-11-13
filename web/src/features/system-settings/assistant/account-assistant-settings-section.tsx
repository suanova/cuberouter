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
import { useEffect } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { JsonCodeEditor } from '@/components/json-code-editor'
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
import { Switch } from '@/components/ui/switch'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'

const isValidFAQJSON = (value: string | undefined) => {
  if (!value || value.trim() === '') return true
  try {
    const parsed = JSON.parse(value)
    if (!Array.isArray(parsed)) return false
    return parsed.every(
      (entry) =>
        entry !== null &&
        typeof entry === 'object' &&
        (typeof entry.title === 'string' || typeof entry.content === 'string')
    )
  } catch {
    return false
  }
}

const createAccountAssistantSchema = (t: (key: string) => string) =>
  z.object({
    'account_assistant.enabled': z.boolean(),
    'account_assistant.model': z.string(),
    'account_assistant.billing_user_id': z.number().int().min(0),
    'account_assistant.group': z.string(),
    'account_assistant.max_tokens': z.number().int().min(64).max(4096),
    'account_assistant.max_tool_calls': z.number().int().min(1).max(8),
    'account_assistant.max_rounds': z.number().int().min(1).max(4),
    'account_assistant.hourly_limit': z.number().int().min(1).max(1000),
    'account_assistant.daily_limit': z.number().int().min(1).max(10000),
    'account_assistant.daily_token_budget': z
      .number()
      .int()
      .min(1000)
      .max(50_000_000),
    'account_assistant.faq_json': z.string().refine(isValidFAQJSON, {
      message: t('FAQ must be a JSON array of { title, content } objects'),
    }),
    'account_assistant.support_url': z.string(),
  })

type AccountAssistantFormValues =
  ReturnType<typeof createAccountAssistantSchema> extends infer T
    ? T extends z.ZodTypeAny
      ? z.infer<T>
      : never
    : never

type AccountAssistantSettingsSectionProps = {
  defaultValues: AccountAssistantFormValues
}

export function AccountAssistantSettingsSection({
  defaultValues,
}: AccountAssistantSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()

  const accountAssistantSchema = createAccountAssistantSchema(t)

  const form = useForm<AccountAssistantFormValues>({
    resolver: zodResolver(accountAssistantSchema),
    mode: 'onChange',
    defaultValues,
  })

  useEffect(() => {
    form.reset(defaultValues)
  }, [defaultValues, form])

  const onSubmit = async (values: AccountAssistantFormValues) => {
    const updates = Object.entries(values).filter(
      ([key, value]) =>
        value !== defaultValues[key as keyof AccountAssistantFormValues]
    )
    for (const [key, value] of updates) {
      await updateOption.mutateAsync({ key, value: value ?? '' })
    }
  }

  const numberFieldChange =
    (onChange: (value: number) => void) =>
    (event: React.ChangeEvent<HTMLInputElement>) =>
      onChange(Number.parseInt(event.target.value, 10) || 0)

  return (
    <SettingsSection title={t('Account Assistant')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save assistant settings'
          />

          <FormField
            control={form.control}
            name='account_assistant.enabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable Account Assistant')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Shows a floating assistant ball to logged-in users. Inactive until a model and channel are configured.'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.model'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Assistant Model')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('e.g. gpt-4o-mini')} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Model used to answer. Must be served by a channel in the billing group.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.billing_user_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Billing Account ID')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={0}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Model calls are billed to this account; 0 = the asking user pays.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.group'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Group')}</FormLabel>
                <FormControl>
                  <Input placeholder={t('e.g. vip')} {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Group used for model calls; empty = the billing account default group.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.max_tokens'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max Tokens')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={64}
                    max={4096}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Max tokens per completion.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.max_tool_calls'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max Tool Calls')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={8}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t('How many tools may run to answer one question.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.max_rounds'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Max Rounds')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={4}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Tool request rounds before a forced final answer.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.hourly_limit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Hourly Limit')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={1000}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Questions each user may ask per hour.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.daily_limit'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Daily Limit')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1}
                    max={10000}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t('Questions each user may ask per day.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.daily_token_budget'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Daily Token Budget')}</FormLabel>
                <FormControl>
                  <Input
                    type='number'
                    min={1000}
                    max={50000000}
                    step={1}
                    {...field}
                    onChange={numberFieldChange(field.onChange)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Daily model token budget per user; refuse once exceeded.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.faq_json'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Help Docs (FAQ)')}</FormLabel>
                <FormControl>
                  <JsonCodeEditor
                    value={field.value ?? ''}
                    onChange={field.onChange}
                    name={field.name}
                    onBlur={field.onBlur}
                    textareaRef={field.ref}
                    placeholder={`[\n  {\n    "title": "How do I renew?",\n    "content": "..."\n  }\n]`}
                    aria-invalid={Boolean(
                      form.formState.errors['account_assistant.faq_json']
                    )}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Entries the assistant searches for non-account questions. JSON array of title/content objects.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='account_assistant.support_url'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Support URL')}</FormLabel>
                <FormControl>
                  <Input placeholder='https://…' {...field} />
                </FormControl>
                <FormDescription>
                  {t(
                    'Link for the "Contact human support" action. Empty = jump to the About page.'
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
