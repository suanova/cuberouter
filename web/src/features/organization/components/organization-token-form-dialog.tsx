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
import { formatQuota } from '@/lib/format'

import {
  batchCreateOrganizationTokens,
  createOrganizationToken,
  getOrganizationMemberModelAccess,
  updateOrganizationToken,
  updateOrganizationTokenResponsibility,
} from '../api'
import { useOrganizationTokenResponsibleOptions } from '../hooks/use-organization-member-options'
import {
  isOrganizationTokenHandover,
  organizationTokenErrorText,
  organizationTokenFormSchema,
  organizationTokenFullKey,
  organizationTokenOwnerName,
  organizationTokenVisibilityMeta,
  responsibleUserIdOf,
  ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS,
  ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
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
import { useOrganizationSurface } from './organization-page-provider'
import { OrganizationTokenKeyCell } from './organization-token-key-cell'

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
  /** The caller is a platform administrator, even when the workspace role hides that. */
  isPlatformAdministrator?: boolean
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
  // The backend exempts platform administrators from the member-only defaults
  // even when their workspace role does not carry `CanManageAllTokens`; the
  // form must not hide fields the server would accept.
  const canConfigure =
    props.canManageAllTokens || Boolean(props.isPlatformAdministrator)

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
  const modelLimits = form.watch('model_limits')

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
    enabled: props.open && canConfigure,
    staleTime: 0,
  })

  // The models a key may be limited to belong to the member who holds it, not
  // to whoever is operating the dialog: the backend refuses a key restricted to
  // a model its responsible member cannot use. The watched value is absent only
  // before the form has been reset, so an edit falls back to the stored holder —
  // the same resolution the save payload makes.
  const responsibleUserId =
    form.watch('responsible_user_id') ||
    (editing ? responsibleUserIdOf(editing) : 0)

  const { data: memberAccess, isError: memberAccessFailed } = useQuery({
    queryKey: [
      'organization-member-model-access',
      surface,
      props.organizationId,
      responsibleUserId,
    ],
    queryFn: () =>
      getOrganizationMemberModelAccess(
        props.organizationId,
        responsibleUserId,
        surface
      ),
    // The read is an administrator's view of a member's allow-list, and it is
    // registered on the member surface only. A member editing their own key has
    // no such view — the backend enforces the same restriction on save — and the
    // platform administrator surface has no mirrored route, so neither asks.
    enabled:
      props.open &&
      props.canManageAllTokens &&
      surface === 'member' &&
      responsibleUserId > 0,
  })

  // A restricted member's allow-list is the only set a key held by them may
  // name; an empty list means "unrestricted", not "no models". A read that is
  // loading, refused or failed falls back to the operator's own models rather
  // than to an empty picker — a refetch failure keeps the previous success in
  // `data`, which is stale and must not narrow the picker either. The save is
  // still refused by the backend if it names a model the member cannot use.
  const accessModels = memberAccessFailed ? [] : (memberAccess?.access ?? [])
  const restrictedModels = accessModels.length > 0 ? accessModels : null
  const modelOptions = restrictedModels ?? models?.data ?? []

  // Reloaded whenever the target changes, so reopening on another key never
  // shows the previous one's numbers.
  useEffect(() => {
    if (!props.open) return
    const defaults: OrganizationTokenFormValues = editing
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
    if (!canConfigure) {
      // The expiry field is hidden for members, so a stored deadline in the
      // past would make the invisible schema refinement reject every save.
      // The backend preserves the stored value; leave the field empty here.
      defaults.expired_time = undefined
    }
    form.reset(defaults)
  }, [
    props.open,
    editing,
    canConfigure,
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

  // The picked models follow the holder: a key switched to a member whose
  // allow-list is narrower keeps only the intersection, rather than submitting
  // entries that no longer have an option behind them. An unrestricted fallback
  // never trims — it is not a narrowing, only a wider set to choose from. An
  // empty intersection keeps the stored selection and is reported instead: an
  // empty limit list means "no restriction" on save, so trimming it here would
  // silently widen the key.
  useEffect(() => {
    if (!restrictedModels) return
    const allowed = new Set(restrictedModels)
    // Read at effect time rather than from the watched value above: on the
    // commit that resets this form for another key, that value is still the
    // previous key's selection and would overwrite the stored limits that were
    // just loaded.
    const current = form.getValues('model_limits')
    const next = current.filter((model) => allowed.has(model))
    if (next.length === 0 && current.length > 0) {
      form.setError('model_limits', {
        type: 'manual',
        message: 'None of the selected models are available to this member',
      })
      return
    }
    form.clearErrors('model_limits')
    if (next.length === current.length) return
    form.setValue('model_limits', next)
  }, [restrictedModels, modelLimits, form])

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
    if (!canConfigure) {
      const result = await createOrganizationToken(props.organizationId, {
        name: values.name.trim(),
      })
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
      toast.success(t('Organization key created'))
      props.onOpenChange(false)
      await props.onSaved()
      return
    }
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
          isUpdate ? t('Edit organization key') : t('Create organization key')
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
            <Button type='submit' form={TOKEN_FORM_ID} disabled={isSubmitting}>
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

              {!isUpdate && canConfigure ? (
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

              {isUpdate ? (
                <div className='space-y-2'>
                  <div className='text-sm leading-none font-medium'>
                    {t('Owner')}
                  </div>
                  <div className='text-muted-foreground text-sm'>
                    {organizationTokenOwnerName(editing)}
                  </div>
                  <p className='text-muted-foreground text-xs'>
                    {t(
                      'The member who created this key. The owner never changes.'
                    )}
                  </p>
                </div>
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
                            <SelectValue placeholder={t('Select a member')} />
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
                          'The member this key belongs to. Only an organization owner or administrator can change the holder.'
                        )}
                      </FormDescription>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
            ) : null}

            {canConfigure ? (
              <>
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
                        {t(
                          'Token group, defaults to the organization base group'
                        )}
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
                          options={modelOptions.map((model) => ({
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
              </>
            ) : null}

            {!canConfigure && !isUpdate ? (
              <p className='text-muted-foreground text-sm'>
                {t(
                  "Quota, expiration and model access are set by the organization's policy."
                )}
              </p>
            ) : null}

            {!canConfigure && isUpdate && editing ? (
              <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm'>
                <dt className='text-muted-foreground'>{t('Quota')}</dt>
                <dd>
                  {editing.unlimited_quota
                    ? t('Unlimited')
                    : formatQuota(editing.remain_quota)}
                </dd>
                <dt className='text-muted-foreground'>
                  {t('Expiration Time')}
                </dt>
                <dd>
                  {editing.expired_time > 0
                    ? new Date(editing.expired_time * 1000).toLocaleString()
                    : t('Never expires')}
                </dd>
                <dt className='text-muted-foreground'>{t('Model Limits')}</dt>
                <dd>
                  {editing.model_limits_enabled && editing.model_limits
                    ? editing.model_limits
                    : t('Unlimited')}
                </dd>
                <dt className='text-muted-foreground'>{t('Token group')}</dt>
                <dd>{editing.group || t("The organization's group")}</dd>
              </dl>
            ) : null}
          </form>
        </Form>
      </Dialog>

      <CreatedTokensDialog result={created} onClose={() => setCreated(null)} />
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
          <div className='text-muted-foreground mt-2 flex flex-wrap gap-x-3 gap-y-1 text-xs'>
            <span>
              {t('Quota')}:{' '}
              {token.unlimited_quota
                ? t('Unlimited')
                : formatQuota(token.remain_quota)}
            </span>
            <span>
              {token.expired_time > 0
                ? new Date(token.expired_time * 1000).toLocaleString()
                : t('Never expires')}
            </span>
          </div>
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
      const key = organizationTokenFullKey(token)
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
