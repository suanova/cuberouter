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
import { Loader2 } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DateTimePicker } from '@/components/datetime-picker'
import { Dialog } from '@/components/dialog'
import { MultiSelect } from '@/components/multi-select'
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
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Switch } from '@/components/ui/switch'
import { Textarea } from '@/components/ui/textarea'
import { getUserModels } from '@/lib/api'
import { copyToClipboard } from '@/lib/copy-to-clipboard'

import {
  batchCreateOrganizationTokens,
  createOrganizationToken,
  updateOrganizationToken,
  updateOrganizationTokenResponsibility,
} from '../api'
import { useOrganizationTokenResponsibleOptions } from '../hooks/use-organization-member-options'
import {
  isOrganizationTokenHandover,
  organizationTokenErrorText,
  organizationTokenFormSchema,
  organizationTokenVisibilityMeta,
  ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS,
  ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
  ORGANIZATION_TOKEN_QUOTA_PRESET_AMOUNTS,
  ORGANIZATION_TOKEN_VISIBILITIES,
  transformOrganizationTokenFormToPayload,
  transformOrganizationTokenToFormDefaults,
  type OrganizationGroupOption,
  type OrganizationTokenFormValues,
} from '../lib'
import type {
  OrganizationTokenBatchCreateResult,
  OrganizationTokenRow,
} from '../types'
import { OrganizationTokenKeyCell } from './organization-token-key-cell'
import { useOrganizationSurface } from './organization-page-provider'

const TOKEN_FORM_ID = 'organization-token-form'

const MEMBER_OPTION_NONE = 'none'

type OrganizationTokenFormDialogProps = {
  organizationId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  /** The key being edited. `null` creates a new one. */
  editing: OrganizationTokenRow | null
  /** The caller may choose the holder and publish a key. */
  canManageAllTokens: boolean
  currentUserId: number
  /**
   * Whether the caller holds a membership here. A platform administrator who
   * does not is not eligible to hold a key — the backend checks membership when
   * it resolves the holder — so they must name one rather than default to
   * themselves.
   */
  isOrganizationMember: boolean
  groupOptions: OrganizationGroupOption[]
  /** Reload the list after a write that changed it. */
  onSaved: () => Promise<unknown> | unknown
}

/**
 * Creates an organization key, or changes one that exists.
 *
 * The two share a form because they share every field but the name of the
 * button: a key is a key whether it is being written for the first time or
 * amended. The differences that remain are the batch count — only a new key can
 * be created in numbers — and the holder, which an edit can hand over to someone
 * else as a separate, separately audited step.
 */
export function OrganizationTokenFormDialog(
  props: OrganizationTokenFormDialogProps
) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const [created, setCreated] =
    useState<OrganizationTokenBatchCreateResult | null>(null)

  const editing = props.editing
  const isUpdate = editing !== null

  const form = useForm<OrganizationTokenFormValues>({
    resolver: zodResolver(organizationTokenFormSchema),
    defaultValues: ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
  })

  // Watched rather than read at submit time: the quota field appears and
  // disappears with the switch, and the group field changes whether a
  // cross-group retry is even a question.
  const unlimitedQuota = form.watch('unlimited_quota')
  const group = form.watch('group')
  const visibility = form.watch('visibility')

  const { options: responsibleOptions, isLoading: isLoadingResponsible } =
    useOrganizationTokenResponsibleOptions(
      surface,
      props.organizationId,
      visibility,
      props.open && props.canManageAllTokens
    )

  const { data: models } = useQuery({
    queryKey: ['user-models'],
    queryFn: getUserModels,
    enabled: props.open,
    staleTime: 0,
  })

  // Reloaded whenever the target changes, so reopening on another key never
  // shows the previous one's numbers.
  useEffect(() => {
    if (!props.open) return
    form.reset(
      editing
        ? transformOrganizationTokenToFormDefaults(editing)
        : {
            ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
            // A new key starts out held by whoever is creating it, which is
            // what the backend would choose anyway — making it visible beats
            // leaving the field blank and silently deciding it later.
            responsible_user_id:
              props.canManageAllTokens && props.isOrganizationMember
                ? props.currentUserId
                : undefined,
            group: '',
          }
    )
  }, [
    props.open,
    editing,
    props.canManageAllTokens,
    props.isOrganizationMember,
    props.currentUserId,
    form,
  ])

  // A holder who cannot hold the key being edited — switching to public drops
  // every member who is not an owner or an administrator — is cleared rather
  // than left selected but invisible, which would submit an id the operator
  // never saw.
  useEffect(() => {
    if (!props.open || !props.canManageAllTokens) return
    if (isLoadingResponsible || responsibleOptions.length === 0) return
    const current = form.getValues('responsible_user_id')
    if (current === undefined) return
    if (responsibleOptions.some((option) => option.value === current)) return
    form.setValue('responsible_user_id', undefined)
  }, [
    props.open,
    props.canManageAllTokens,
    isLoadingResponsible,
    responsibleOptions,
    form,
  ])

  const submit = async (values: OrganizationTokenFormValues) => {
    const payload = transformOrganizationTokenFormToPayload(values, {
      canManageAllTokens: props.canManageAllTokens,
      editing: editing ?? undefined,
    })

    if (isUpdate && editing) {
      const result = await updateOrganizationToken(
        surface,
        props.organizationId,
        editing.id,
        payload
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to save organization key'))
        return
      }
      // A hand-over is recorded through its own endpoint: the update already
      // moved the key, and this is what writes the transfer into the audit
      // trail as an event of its own rather than as a field that changed.
      if (
        props.canManageAllTokens &&
        isOrganizationTokenHandover(editing, values.responsible_user_id)
      ) {
        await updateOrganizationTokenResponsibility(
          surface,
          props.organizationId,
          editing.id,
          { responsible_user_id: Number(values.responsible_user_id) }
        )
      }
      toast.success(t('Organization key updated'))
      props.onOpenChange(false)
      await props.onSaved()
      return
    }

    const count = values.token_count ?? 1
    if (count > 1) {
      const result = await batchCreateOrganizationTokens(props.organizationId, {
        ...payload,
        token_count: count,
      })
      if (!result.success || !result.data) {
        toast.error(result.message || t('Failed to create organization key'))
        return
      }
      setCreated(result.data)
    } else {
      const result = await createOrganizationToken(
        props.organizationId,
        payload
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to create organization key'))
        return
      }
      if (result.data) {
        setCreated({
          tokens: [result.data],
          token_count: 1,
          secret_available: Boolean(result.data.key),
        })
      }
    }

    toast.success(t('Organization key created'))
    props.onOpenChange(false)
    await props.onSaved()
  }

  const onSubmit = async (values: OrganizationTokenFormValues) => {
    setIsSubmitting(true)
    try {
      await submit(values)
    } catch (error) {
      toast.error(
        organizationTokenErrorText(error, t('Failed to save organization key'))
      )
    } finally {
      setIsSubmitting(false)
    }
  }

  const quotaPresets = ORGANIZATION_TOKEN_QUOTA_PRESET_AMOUNTS

  // Each picker resolves its trigger text from the items the root is given, so
  // the list is built once here and is what both the options and the registry
  // are drawn from — the two cannot drift apart.
  const visibilityItems = Object.entries(ORGANIZATION_TOKEN_VISIBILITIES).map(
    ([value, meta]) => ({ value, label: t(meta.labelKey) })
  )
  const responsibleItems = responsibleOptions.map((option) => ({
    value: String(option.value),
    label: option.label,
  }))
  const groupItems = [
    { value: MEMBER_OPTION_NONE, label: t("The organization's group") },
    ...props.groupOptions.map((option) => ({
      value: option.value,
      label: option.desc ? `${option.label} · ${option.desc}` : option.label,
    })),
  ]

  return (
    <>
      <Dialog
        open={props.open}
        onOpenChange={props.onOpenChange}
        title={
          isUpdate
            ? t('Edit organization key')
            : t('Create organization key')
        }
        description={
          isUpdate
            ? t('Change what this key may do. Its secret does not change.')
            : t(
                'The key belongs to the organization and is charged to it, not to your personal quota.'
              )
        }
        contentHeight='min(80vh, 720px)'
        bodyClassName='space-y-5'
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
              form={TOKEN_FORM_ID}
              disabled={isSubmitting}
            >
              {isSubmitting ? <Loader2 className='animate-spin' /> : null}
              {isUpdate ? t('Save changes') : t('Create')}
            </Button>
          </>
        }
      >
        <Form {...form}>
          <form
            id={TOKEN_FORM_ID}
            onSubmit={form.handleSubmit(onSubmit)}
            className='space-y-5'
          >
            <div className='grid gap-4 sm:grid-cols-2'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('Name')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('ci key')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              {!isUpdate ? (
                <FormField
                  control={form.control}
                  name='token_count'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Create count')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='1'
                          max='100'
                          onChange={(event) =>
                            field.onChange(
                              Number.parseInt(event.target.value, 10) || 1
                            )
                          }
                        />
                      </FormControl>
                      <FormDescription>
                        {t(
                          'More than one creates a batch, and every key in it is returned once.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ) : null}
            </div>

            {props.canManageAllTokens ? (
              <div className='grid gap-4 sm:grid-cols-2'>
                <FormField
                  control={form.control}
                  name='visibility'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Visibility')}</FormLabel>
                      <Select
                        items={visibilityItems}
                        value={field.value}
                        onValueChange={field.onChange}
                      >
                        <FormControl>
                          <SelectTrigger className='w-full'>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {visibilityItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {t(
                          organizationTokenVisibilityMeta(field.value)
                            .descriptionKey
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name='responsible_user_id'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Responsible user')}</FormLabel>
                      <Select
                        items={responsibleItems}
                        value={
                          field.value === undefined ? '' : String(field.value)
                        }
                        onValueChange={(value) =>
                          field.onChange(
                            value === '' || value === null
                              ? undefined
                              : Number(value)
                          )
                        }
                        disabled={isLoadingResponsible}
                      >
                        <FormControl>
                          <SelectTrigger className='w-full'>
                            <SelectValue
                              placeholder={t('Select a member')}
                            />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {responsibleItems.map((item) => (
                            <SelectItem key={item.value} value={item.value}>
                              {item.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormDescription>
                        {t(
                          'The member this key belongs to. Only they, and an administrator, may change it.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            ) : null}

            <div className='space-y-4 rounded-lg border p-4'>
              <FormField
                control={form.control}
                name='unlimited_quota'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-start justify-between gap-4'>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-sm'>
                        {t('Unlimited Quota')}
                      </FormLabel>
                      <FormDescription className='text-xs'>
                        {t(
                          'Usage is still limited by what the organization has left.'
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

              {!unlimitedQuota ? (
                <FormField
                  control={form.control}
                  name='remain_quota_dollars'
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>{t('Key quota limit')}</FormLabel>
                      <FormControl>
                        <Input
                          {...field}
                          type='number'
                          min='0'
                          step='0.01'
                          placeholder={t('Enter key quota limit')}
                          onChange={(event) =>
                            field.onChange(
                              Number.parseFloat(event.target.value) || 0
                            )
                          }
                        />
                      </FormControl>
                      <div className='flex flex-wrap gap-1 pt-1'>
                        {quotaPresets.map((amount) => (
                          <Button
                            key={amount}
                            type='button'
                            variant='outline'
                            size='xs'
                            onClick={() =>
                              form.setValue('remain_quota_dollars', amount, {
                                shouldValidate: true,
                              })
                            }
                          >
                            {t('{{amount}} dollars', { amount })}
                          </Button>
                        ))}
                      </div>
                      <FormDescription>
                        {t(
                          'Limits what this one key may spend. The organization pays for whatever it does use.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              ) : null}
            </div>

            <FormField
              control={form.control}
              name='expired_time'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Expiration Time')}</FormLabel>
                  <FormControl>
                    <DateTimePicker
                      value={field.value}
                      onChange={field.onChange}
                      placeholder={t('Select expiration time')}
                    />
                  </FormControl>
                  <div className='flex flex-wrap gap-1 pt-1'>
                    {ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS.map((shortcut) => (
                      <Button
                        key={shortcut.labelKey}
                        type='button'
                        variant='outline'
                        size='xs'
                        onClick={() =>
                          field.onChange(
                            shortcut.seconds === null
                              ? undefined
                              : new Date(
                                  Date.now() + shortcut.seconds * 1000
                                )
                          )
                        }
                      >
                        {t(shortcut.labelKey)}
                      </Button>
                    ))}
                  </div>
                  <FormDescription>
                    {t(
                      'Left empty, the key never expires. It still stops working if the responsible member is disabled.'
                    )}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='group'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Token group')}</FormLabel>
                  <Select
                    items={groupItems}
                    value={field.value || MEMBER_OPTION_NONE}
                    onValueChange={(value) =>
                      field.onChange(
                        value === MEMBER_OPTION_NONE || value === null
                          ? ''
                          : value
                      )
                    }
                  >
                    <FormControl>
                      <SelectTrigger className='w-full'>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      {groupItems.map((item) => (
                        <SelectItem key={item.value} value={item.value}>
                          {item.label}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                  <FormDescription>
                    {t('Token group, defaults to the organization base group')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            {group === 'auto' ? (
              <FormField
                control={form.control}
                name='cross_group_retry'
                render={({ field }) => (
                  <FormItem className='flex flex-row items-start justify-between gap-4'>
                    <div className='flex flex-col gap-0.5'>
                      <FormLabel className='text-sm'>
                        {t('Cross-group retry')}
                      </FormLabel>
                      <FormDescription className='text-xs'>
                        {t(
                          'Retry a failed request on another group in the Auto pool.'
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
            ) : null}

            <FormField
              control={form.control}
              name='model_limits'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('Model Limits')}</FormLabel>
                  <FormControl>
                    <MultiSelect
                      options={(models?.data ?? []).map((model) => ({
                        label: model,
                        value: model,
                      }))}
                      selected={field.value}
                      onChange={field.onChange}
                      placeholder={t(
                        'Select allowed models; leave empty for no limit'
                      )}
                    />
                  </FormControl>
                  <FormDescription>
                    {t('Limit which models can be used with this key')}
                  </FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='allow_ips'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('IP limits')}</FormLabel>
                  <FormControl>
                    <Textarea
                      {...field}
                      className='min-h-20 resize-none'
                      rows={3}
                      placeholder={t(
                        'One IP per line; leave empty for no limit'
                      )}
                    />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />
          </form>
        </Form>
      </Dialog>

      <CreatedTokensDialog
        result={created}
        onClose={() => setCreated(null)}
      />
    </>
  )
}

/**
 * The secrets of a batch that was just created.
 *
 * A batch is idempotent, so a replayed request returns keys that already exist
 * rather than new ones — and their secrets may not come back with them. The two
 * cases are told apart rather than shown the same way, because only one of them
 * still has something to copy.
 */
function CreatedTokensDialog(props: {
  result: OrganizationTokenBatchCreateResult | null
  onClose: () => void
}) {
  const { t } = useTranslation()
  const result = props.result
  const tokens = result?.tokens ?? []
  const secretAvailable = Boolean(result?.secret_available)
  const total = result?.token_count ?? tokens.length

  return (
    <Dialog
      open={result !== null}
      onOpenChange={(open) => !open && props.onClose()}
      title={t('Created organization keys')}
      description={
        secretAvailable
          ? t(
              'These keys were created. You can copy them now, or view and copy authorized keys later from the list.'
            )
          : t(
              'This batch request was safely replayed. Existing keys are shown as summaries here; authorized keys can be viewed and copied from the list.'
            )
      }
      contentHeight='min(70vh, 560px)'
      bodyClassName='space-y-2'
      footer={
        <>
          {secretAvailable ? (
            <Button
              type='button'
              onClick={() =>
                void copyCreatedTokens(
                  tokens,
                  t('Copied to clipboard!'),
                  t('Failed to copy to clipboard')
                )
              }
            >
              {t('Copy all created keys')}
            </Button>
          ) : null}
          <Button type='button' variant='outline' onClick={props.onClose}>
            {t('Close')}
          </Button>
        </>
      }
    >
      {tokens.map((token, index) => (
        <div key={token.id} className='rounded-lg border p-3'>
          <div className='mb-2 flex items-center justify-between gap-2'>
            <span className='truncate text-sm font-medium'>
              {token.name || t('Created key {{index}}', { index: index + 1 })}
            </span>
            <span className='text-muted-foreground font-mono text-xs'>
              #{token.id}
            </span>
          </div>
          <OrganizationTokenKeyCell token={token} canCopy={secretAvailable} />
        </div>
      ))}
      <p className='text-muted-foreground pt-2 text-xs'>
        {t('{{count}} organization key(s) affected.', { count: total })}
      </p>
    </Dialog>
  )
}

async function copyCreatedTokens(
  tokens: OrganizationTokenRow[],
  successMessage: string,
  failureMessage: string
) {
  const lines = tokens
    .map((token) => {
      const key = (token.key ?? '').trim()
      return key ? `${token.name || token.id}\t${key}` : ''
    })
    .filter(Boolean)

  if (lines.length === 0) {
    toast.error(failureMessage)
    return
  }
  const ok = await copyToClipboard(lines.join('\n'))
  if (ok) {
    toast.success(successMessage)
  } else {
    toast.error(failureMessage)
  }
}
