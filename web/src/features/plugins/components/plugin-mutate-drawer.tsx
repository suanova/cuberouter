/*
Copyright (C) 2023-2026 QuantumNous

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
import { Loader2, PlugZap } from 'lucide-react'
import { useEffect, useMemo, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  sideDrawerContentClassName,
  sideDrawerFooterClassName,
  sideDrawerFormClassName,
  sideDrawerHeaderClassName,
  sideDrawerSwitchItemClassName,
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
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'

import { createPlugin, testPluginConnection, updatePlugin } from '../api'
import {
  getPluginFormSchema,
  PLUGIN_FORM_DEFAULT_VALUES,
  type PluginFormValues,
} from '../lib/plugin-form'
import type { Plugin } from '../types'

type PluginMutateDrawerProps = {
  open: boolean
  plugin: Plugin | null
  onOpenChange: (open: boolean) => void
  onSaved: () => Promise<void> | void
}

export function PluginMutateDrawer({
  open,
  plugin,
  onOpenChange,
  onSaved,
}: PluginMutateDrawerProps) {
  const { t } = useTranslation()
  const isEditing = plugin !== null
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [isTesting, setIsTesting] = useState(false)

  const schema = useMemo(() => getPluginFormSchema(t), [t])

  const form = useForm<PluginFormValues>({
    resolver: zodResolver(schema),
    defaultValues: PLUGIN_FORM_DEFAULT_VALUES,
  })

  useEffect(() => {
    if (!open) return
    if (plugin) {
      form.reset({
        name: plugin.name,
        slug: plugin.slug,
        description: plugin.description,
        mcp_url: plugin.mcp_url,
        auth_header: plugin.auth_header ?? '',
        // The API masks auth_token in responses; keep the field empty and
        // let the backend preserve the stored token on save.
        auth_token: '',
        skill_source: plugin.skill_source,
        enabled: plugin.enabled,
      })
    } else {
      form.reset(PLUGIN_FORM_DEFAULT_VALUES)
    }
  }, [open, plugin, form])

  const handleOpenChange = (nextOpen: boolean) => {
    onOpenChange(nextOpen)
    if (!nextOpen) {
      form.reset(PLUGIN_FORM_DEFAULT_VALUES)
    }
  }

  const handleTestConnection = async () => {
    const mcpUrl = form.getValues('mcp_url').trim()
    if (!mcpUrl) {
      toast.error(t('Please enter MCP URL first'))
      return
    }
    setIsTesting(true)
    try {
      const result = await testPluginConnection(
        mcpUrl,
        form.getValues('auth_token'),
        form.getValues('auth_header')
      )
      if (result.tools.length === 0) {
        toast.info(t('Connection succeeded, but no tools were returned'))
      } else {
        toast.success(
          t('Connection succeeded: {{count}} tool(s): {{tools}}', {
            count: result.tools.length,
            tools: result.tools.join(', '),
          }),
          { duration: 8000 }
        )
      }
    } catch (error) {
      toast.error(
        error instanceof Error ? error.message : t('Connection test failed')
      )
    } finally {
      setIsTesting(false)
    }
  }

  const onSubmit = async (values: PluginFormValues) => {
    setIsSubmitting(true)
    try {
      const response = isEditing
        ? await updatePlugin({ ...values, id: plugin.id })
        : await createPlugin(values)
      if (!response.success) {
        // Business failures already surfaced by the global error toast.
        return
      }
      if (response.message) {
        // The backend still saved the plugin; message carries the
        // skill-fetch warning.
        toast.warning(response.message, { duration: 8000 })
      } else {
        toast.success(isEditing ? t('Plugin updated') : t('Plugin created'))
      }
      onOpenChange(false)
      form.reset(PLUGIN_FORM_DEFAULT_VALUES)
      await onSaved()
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={handleOpenChange}>
      <SheetContent className={sideDrawerContentClassName('sm:max-w-xl')}>
        <SheetHeader className={sideDrawerHeaderClassName()}>
          <SheetTitle>
            {isEditing ? t('Edit Plugin') : t('New Plugin')}
          </SheetTitle>
          <SheetDescription>
            {isEditing
              ? t('Update plugin configuration and save when you are done.')
              : t(
                  'Register an MCP server as a plugin, optionally with a skill document.'
                )}
          </SheetDescription>
        </SheetHeader>

        <Form {...form}>
          <form
            id='plugin-form'
            onSubmit={form.handleSubmit(onSubmit)}
            className={sideDrawerFormClassName('gap-5')}
          >
            <div className='grid gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name *')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('e.g. GitHub Tools')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
              <FormField
                control={form.control}
                name='slug'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Slug *')}</FormLabel>
                    <FormControl>
                      <Input
                        placeholder={t('e.g. github-tools')}
                        disabled={isEditing}
                        {...field}
                      />
                    </FormControl>
                    <FormDescription>
                      {isEditing
                        ? t('Slug cannot be changed after creation.')
                        : t(
                            'Used with @ in the playground, e.g. @github-tools.'
                          )}
                    </FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>

            <FormField
              control={form.control}
              name='description'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Description')}</FormLabel>
                  <FormControl>
                    <Textarea
                      rows={2}
                      placeholder={t('What does this plugin do?')}
                      {...field}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='mcp_url'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('MCP URL *')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://example.com/mcp')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Streamable HTTP endpoint of the MCP server.')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='auth_token'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auth Token')}</FormLabel>
                  <FormControl>
                    <Input
                      type='password'
                      placeholder={
                        isEditing
                          ? t('Leave blank to keep the current token')
                          : t('Optional auth token')
                      }
                      autoComplete='new-password'
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional. Sent as "Authorization: Bearer <token>" by default, or with the header below.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='auth_header'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Auth Header')}</FormLabel>
                  <FormControl>
                    <Input placeholder={t('e.g. X-Auth-Token')} {...field} />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional. Custom header name for the token. Leave blank for "Authorization: Bearer".'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='skill_source'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Skill Source')}</FormLabel>
                  <FormControl>
                    <Input
                      placeholder={t('https://github.com/owner/repo')}
                      {...field}
                    />
                  </FormControl>
                  <FormDescription>
                    {t(
                      'Optional. A GitHub repo or raw URL to fetch the skill document from.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='enabled'
              render={({ field }) => (
                <FormItem className={sideDrawerSwitchItemClassName()}>
                  <div className='flex flex-col gap-0.5'>
                    <FormLabel>{t('Enabled')}</FormLabel>
                    <FormDescription className='text-xs'>
                      {t(
                        'Only enabled plugins are available in the playground'
                      )}
                    </FormDescription>
                  </div>
                  <FormControl>
                    <Switch
                      checked={field.value}
                      onCheckedChange={field.onChange}
                    />
                  </FormControl>
                </FormItem>
              )}
            />
          </form>
        </Form>

        <SheetFooter className={sideDrawerFooterClassName()}>
          <Button
            type='button'
            variant='outline'
            disabled={isTesting || isSubmitting}
            onClick={() => void handleTestConnection()}
          >
            {isTesting ? (
              <Loader2 className='size-4 animate-spin' />
            ) : (
              <PlugZap className='size-4' />
            )}
            {t('Test Connection')}
          </Button>
          <SheetClose
            render={<Button variant='outline' disabled={isSubmitting} />}
          >
            {t('Cancel')}
          </SheetClose>
          <Button form='plugin-form' type='submit' disabled={isSubmitting}>
            {isSubmitting && <Loader2 className='size-4 animate-spin' />}
            {t('Save')}
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}
