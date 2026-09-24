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
import { Loader2, Search, X } from 'lucide-react'
import { useEffect, useState } from 'react'
import { useForm, type Control, type FieldPath, type FieldValues } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import z from 'zod'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { Dialog } from '@/components/dialog'
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
import { Textarea } from '@/components/ui/textarea'
import { searchUsers } from '@/features/users/api'

import {
  addOrganizationMember,
  exitOrganization,
  removeOrganizationMember,
  updateOrganizationMember,
} from '../api'
import {
  ORGANIZATION_ASSIGNABLE_ROLES,
  organizationRoleLabelKey,
} from '../constants'
import { useOrganizationMemberOptions } from '../hooks/use-organization-member-options'
import {
  buildOrganizationMemberRoleUpdatePayload,
  isOrganizationMemberDemotion,
  type OrganizationMemberOption,
} from '../lib'
import type { OrganizationMemberRow } from '../types'
import { useOrganizationSurface } from './organization-page-provider'

/** How long the user picker waits after the last keystroke before searching. */
const USER_SEARCH_DEBOUNCE_MS = 300

const ADD_MEMBER_FORM_ID = 'organization-add-member-form'
const REMOVE_MEMBER_FORM_ID = 'organization-remove-member-form'
const DEMOTE_MEMBER_FORM_ID = 'organization-demote-member-form'

type SearchableUser = { label: string; value: number }

function toSearchableUser(user: {
  id: number
  username?: string
  display_name?: string
  email?: string
}): SearchableUser {
  const name =
    user.username || user.display_name || user.email || String(user.id)
  return {
    value: user.id,
    label: user.email ? `${name} · ${user.email}` : name,
  }
}

/**
 * Picks a platform user by searching the platform's own user directory.
 *
 * The search runs on the server and its results are shown as they come back,
 * without a second round of filtering here: the directory may have matched on a
 * field the label does not spell out, and narrowing again would hide a hit the
 * server found.
 *
 * The parent remounts this whenever the dialog opens, which is what clears the
 * keyword and the previous selection.
 */
function MemberUserPicker(props: {
  value: number | undefined
  onChange: (value: number | undefined) => void
}) {
  const { t } = useTranslation()
  const [keyword, setKeyword] = useState('')
  const [options, setOptions] = useState<SearchableUser[]>([])
  const [selected, setSelected] = useState<SearchableUser | null>(null)
  const [isSearching, setIsSearching] = useState(false)

  useEffect(() => {
    const text = keyword.trim()
    if (!text) {
      setOptions([])
      setIsSearching(false)
      return
    }

    // Guards against a slow response for an earlier keyword overwriting the
    // results for a later one.
    let active = true
    setIsSearching(true)
    const timer = setTimeout(() => {
      void (async () => {
        try {
          const result = await searchUsers({
            keyword: text,
            p: 1,
            page_size: 20,
          })
          if (!active) return
          setOptions((result.data?.items ?? []).map(toSearchableUser))
        } catch {
          if (!active) return
          // The interceptor has already reported it. Showing no matches beats
          // showing the previous keyword's matches under the new one.
          setOptions([])
        } finally {
          if (active) setIsSearching(false)
        }
      })()
    }, USER_SEARCH_DEBOUNCE_MS)

    return () => {
      active = false
      clearTimeout(timer)
    }
  }, [keyword])

  const select = (user: SearchableUser) => {
    setSelected(user)
    props.onChange(user.value)
    setKeyword('')
    setOptions([])
  }

  const clear = () => {
    setSelected(null)
    props.onChange(undefined)
  }

  if (selected) {
    return (
      <div className='border-input flex items-center justify-between gap-2 rounded-md border px-3 py-2 text-sm'>
        <span className='truncate'>{selected.label}</span>
        <Button
          type='button'
          size='icon'
          variant='ghost'
          className='size-6 shrink-0'
          onClick={clear}
        >
          <X className='size-3.5' />
          <span className='sr-only'>{t('Clear')}</span>
        </Button>
      </div>
    )
  }

  return (
    <div className='flex flex-col gap-2'>
      <div className='relative'>
        <Search className='text-muted-foreground pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2' />
        <Input
          value={keyword}
          onChange={(event) => setKeyword(event.target.value)}
          placeholder={t('Search username, display name, or email')}
          className='pl-9'
        />
        {isSearching ? (
          <Loader2 className='text-muted-foreground absolute top-1/2 right-3 size-4 -translate-y-1/2 animate-spin' />
        ) : null}
      </div>

      {options.length > 0 ? (
        <ul className='border-border max-h-48 overflow-y-auto rounded-md border'>
          {options.map((option) => (
            <li key={option.value}>
              <button
                type='button'
                className='hover:bg-accent w-full px-3 py-2 text-start text-sm'
                onClick={() => select(option)}
              >
                {option.label}
              </button>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  )
}

/**
 * The person who takes over keys from the member being removed or demoted.
 *
 * The empty option is not a placeholder that vanishes on selection — it is a
 * real choice meaning "let the backend's default policy decide", so it stays
 * selectable and is worded as its consequence rather than as the word "None".
 *
 * Generic over the form's values because the remove and demote dialogs have
 * different validation rules but share this one field.
 */
function TransferTargetField<TValues extends FieldValues>(props: {
  control: Control<TValues>
  options: OrganizationMemberOption[]
  isLoading: boolean
  emptyOptionLabel: string
}) {
  const { t } = useTranslation()

  // The trigger resolves its text from the items the root is given, so the
  // options below are built once here and the select is drawn from them.
  const items = [
    { value: '', label: props.emptyOptionLabel },
    ...props.options.map((option) => ({
      value: String(option.value),
      label: option.label,
    })),
  ]

  return (
    <FormField
      control={props.control}
      name={'transfer_to_user_id' as FieldPath<TValues>}
      render={({ field }) => (
        <FormItem>
          <FormLabel>{t('Transfer responsible API keys to')}</FormLabel>
          <Select
            items={items}
            value={field.value === undefined ? '' : String(field.value)}
            onValueChange={(value) =>
              field.onChange(value === '' || value === null ? undefined : Number(value))
            }
            disabled={props.isLoading}
          >
            <FormControl>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('Select a member')} />
              </SelectTrigger>
            </FormControl>
            <SelectContent>
              {items.map((item) => (
                <SelectItem key={item.value} value={item.value}>
                  {item.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <FormDescription>
            {t('Only active owners and administrators can take over API keys.')}
          </FormDescription>
          <FormMessage />
        </FormItem>
      )}
    />
  )
}

// ============================================================================
// Add Member
// ============================================================================

const addMemberSchema = z.object({
  user_id: z.number({ error: 'Select a user.' }),
  role: z.enum(['admin', 'member']),
  reason: z.string().optional(),
})

type AddMemberValues = z.infer<typeof addMemberSchema>

/**
 * Adds a user straight to the organization, without an invitation.
 *
 * This is a platform administrator action rather than an organization one: it
 * posts to `/api/admin/organizations/:id/members`, so the caller only reaches
 * this dialog when the backend has already granted `can_add_members_directly`.
 */
export function OrganizationAddMemberDialog(props: {
  organizationId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onAdded: () => Promise<unknown> | unknown
}) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const form = useForm<AddMemberValues>({
    resolver: zodResolver(addMemberSchema),
    defaultValues: { user_id: undefined, role: 'member', reason: '' },
  })

  const onSubmit = async (values: AddMemberValues) => {
    setIsSubmitting(true)
    try {
      const result = await addOrganizationMember(props.organizationId, {
        user_id: values.user_id,
        role: values.role,
        reason: (values.reason ?? '').trim(),
      })
      if (!result.success) {
        toast.error(result.message || t('Failed to add the member'))
        return
      }
      toast.success(t('Member added'))
      props.onOpenChange(false)
      await props.onAdded()
    } finally {
      setIsSubmitting(false)
    }
  }

  const roleItems = ORGANIZATION_ASSIGNABLE_ROLES.map((role) => ({
    value: role,
    label: t(organizationRoleLabelKey(role)),
  }))

  return (
    <Dialog
      open={props.open}
      onOpenChange={(open) => {
        if (open) form.reset({ user_id: undefined, role: 'member', reason: '' })
        props.onOpenChange(open)
      }}
      title={t('Add member')}
      description={t(
        'The user joins immediately, without an invitation. The action is recorded in the audit trail.'
      )}
      bodyClassName='space-y-4'
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
          <Button type='submit' form={ADD_MEMBER_FORM_ID} disabled={isSubmitting}>
            {isSubmitting ? <Loader2 className='animate-spin' /> : null}
            {isSubmitting ? t('Adding...') : t('Add')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={ADD_MEMBER_FORM_ID}
          onSubmit={form.handleSubmit(onSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='user_id'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('User')}</FormLabel>
                <MemberUserPicker
                  key={String(props.open)}
                  value={field.value}
                  onChange={field.onChange}
                />
                <FormDescription>
                  {t('Search the platform user directory.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='role'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Role')}</FormLabel>
                <Select
                  items={roleItems}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {roleItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
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
                    rows={3}
                    placeholder={t('Enter operation reason (optional)')}
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}

// ============================================================================
// Remove Member
// ============================================================================

const removeMemberSchema = z.object({
  transfer_to_user_id: z.number().optional(),
  reason: z.string().min(1, 'Enter an operation reason.'),
})

type RemoveMemberValues = z.infer<typeof removeMemberSchema>

/**
 * Removes a member, and decides what becomes of the keys they were responsible
 * for.
 *
 * The reason is required here and optional elsewhere because this is the
 * destructive action: the reason is the only account of it that survives.
 */
export function OrganizationRemoveMemberDialog(props: {
  organizationId: number
  member: OrganizationMemberRow | null
  onOpenChange: (open: boolean) => void
  onRemoved: () => Promise<unknown> | unknown
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const member = props.member
  const { options, isLoading } = useOrganizationMemberOptions(
    surface,
    props.organizationId,
    member?.user_id,
    member !== null
  )

  const form = useForm<RemoveMemberValues>({
    resolver: zodResolver(removeMemberSchema),
    defaultValues: { transfer_to_user_id: undefined, reason: '' },
  })

  if (!member) return null

  const onSubmit = async (values: RemoveMemberValues) => {
    setIsSubmitting(true)
    try {
      const result = await removeOrganizationMember(
        surface,
        props.organizationId,
        member.user_id,
        {
          transfer_to_user_id: values.transfer_to_user_id,
          reason: values.reason.trim(),
        }
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to remove the member'))
        return
      }
      toast.success(t('Member removed'))
      props.onOpenChange(false)
      await props.onRemoved()
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onOpenChange(false)
      }}
      title={t('Remove member')}
      description={t(
        'The member loses access immediately. Leave the transfer target empty to move their public API keys to the owner and delete their private ones.'
      )}
      bodyClassName='space-y-4'
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
            form={REMOVE_MEMBER_FORM_ID}
            variant='destructive'
            disabled={isSubmitting}
          >
            {isSubmitting ? <Loader2 className='animate-spin' /> : null}
            {isSubmitting ? t('Removing...') : t('Remove')}
          </Button>
        </>
      }
    >
      <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm'>
        <dt className='text-muted-foreground'>{t('Username')}</dt>
        <dd className='truncate'>{member.username || '-'}</dd>
        <dt className='text-muted-foreground'>{t('Email')}</dt>
        <dd className='truncate'>{member.email || '-'}</dd>
        <dt className='text-muted-foreground'>{t('Role')}</dt>
        <dd>{t(organizationRoleLabelKey(member.role))}</dd>
      </dl>

      <Form {...form}>
        <form
          id={REMOVE_MEMBER_FORM_ID}
          onSubmit={form.handleSubmit(onSubmit)}
          className='space-y-4'
        >
          <TransferTargetField
            control={form.control}
            options={options}
            isLoading={isLoading}
            emptyOptionLabel={t(
              'Move public API keys to the owner and delete private ones'
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
                    rows={3}
                    placeholder={t('Enter operation reason')}
                    {...field}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}

// ============================================================================
// Demote Administrator
// ============================================================================

const demoteSchema = z.object({
  transfer_to_user_id: z.number().optional(),
  reason: z.string().optional(),
})

type DemoteValues = z.infer<typeof demoteSchema>

/**
 * Demotes an administrator to member.
 *
 * Demotion can strand the keys the administrator was responsible for, so the
 * backend refuses it without a transfer target. Asking for one here turns that
 * refusal into a choice instead of an error the operator has to decode.
 */
export function OrganizationDemoteAdminDialog(props: {
  organizationId: number
  member: OrganizationMemberRow | null
  onOpenChange: (open: boolean) => void
  onDemoted: () => Promise<unknown> | unknown
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const member = props.member
  const { options, isLoading } = useOrganizationMemberOptions(
    surface,
    props.organizationId,
    member?.user_id,
    member !== null
  )

  const form = useForm<DemoteValues>({
    resolver: zodResolver(demoteSchema),
    defaultValues: { transfer_to_user_id: undefined, reason: '' },
  })

  if (!member) return null

  const onSubmit = async (values: DemoteValues) => {
    setIsSubmitting(true)
    try {
      const result = await updateOrganizationMember(
        surface,
        props.organizationId,
        member.user_id,
        buildOrganizationMemberRoleUpdatePayload(member, 'member', {
          transferToUserId: values.transfer_to_user_id,
          reason: values.reason,
        })
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to update the member'))
        return
      }
      toast.success(t('Member updated'))
      props.onOpenChange(false)
      await props.onDemoted()
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onOpenChange(false)
      }}
      title={t('Demote administrator')}
      description={t(
        'The administrator becomes a member. Leave the transfer target empty to transfer their responsible API keys to the organization owner.'
      )}
      bodyClassName='space-y-4'
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
            form={DEMOTE_MEMBER_FORM_ID}
            disabled={isSubmitting}
          >
            {isSubmitting ? <Loader2 className='animate-spin' /> : null}
            {isSubmitting ? t('Saving...') : t('Demote')}
          </Button>
        </>
      }
    >
      <Form {...form}>
        <form
          id={DEMOTE_MEMBER_FORM_ID}
          onSubmit={form.handleSubmit(onSubmit)}
          className='space-y-4'
        >
          <TransferTargetField
            control={form.control}
            options={options}
            isLoading={isLoading}
            emptyOptionLabel={t('Transfer responsible API keys to the owner')}
          />

          <FormField
            control={form.control}
            name='reason'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Reason')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('Enter operation reason (optional)')}
                    {...field}
                    value={field.value ?? ''}
                  />
                </FormControl>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}

// ============================================================================
// Edit Member (platform surface)
// ============================================================================

const EDIT_MEMBER_FORM_ID = 'organization-edit-member-form'

/**
 * The reason is required here, though the backend accepts an empty one on a
 * promotion and falls back to its own wording on a demotion.
 *
 * That is deliberate: this dialog belongs to the platform surface, where every
 * change is made to an organization the caller does not belong to, and the
 * reason is the only part of it the organization's own members read.
 */
const editMemberSchema = z.object({
  role: z.enum(['admin', 'member']),
  status: z.enum(['active', 'disabled']),
  transfer_to_user_id: z.number().optional(),
  reason: z.string().trim().min(1, 'Enter an operation reason.'),
})

type EditMemberValues = z.infer<typeof editMemberSchema>

const EMPTY_EDIT_VALUES: EditMemberValues = {
  role: 'member',
  status: 'active',
  transfer_to_user_id: undefined,
  reason: '',
}

/**
 * Changes a member's role and status together, on the platform surface.
 *
 * The organization center changes a role from the row itself and a status from
 * the row's menu, and neither asks why. That is fine for an organization
 * deciding its own business; it is not fine for an administrator reaching into
 * an organization they are not part of, where the reason is the entire account
 * of the change as far as its members are concerned. So on the platform surface
 * both go through this dialog, and the reason is required.
 *
 * Only what actually moved is sent. The endpoint refuses a request that carries
 * neither a role nor a status, and sending an unchanged role alongside a status
 * change would make the audit entry claim a role change that did not happen.
 */
export function OrganizationMemberEditDialog(props: {
  organizationId: number
  member: OrganizationMemberRow | null
  onOpenChange: (open: boolean) => void
  onUpdated: () => Promise<unknown> | unknown
}) {
  const { t } = useTranslation()
  const surface = useOrganizationSurface()
  const [isSubmitting, setIsSubmitting] = useState(false)
  const member = props.member

  const form = useForm<EditMemberValues>({
    resolver: zodResolver(editMemberSchema),
    defaultValues: EMPTY_EDIT_VALUES,
  })

  useEffect(() => {
    if (!member) return
    form.reset({
      // An owner's row never reaches this dialog — the backend refuses every
      // member operation against it — so the fallback is only here to keep an
      // unexpected role from blanking the picker.
      role: member.role === 'member' ? 'member' : 'admin',
      status: member.status === 'disabled' ? 'disabled' : 'active',
      transfer_to_user_id: undefined,
      reason: '',
    })
  }, [member, form])

  const role = form.watch('role')
  const status = form.watch('status')
  const isDemoting = member ? isOrganizationMemberDemotion(member, role) : false
  // Demotion can strand the keys this member was responsible for, which is what
  // makes a transfer target worth asking for before the request is refused. The
  // picker only loads while the dialog is actually offering it.
  const { options, isLoading } = useOrganizationMemberOptions(
    surface,
    props.organizationId,
    member?.user_id,
    member !== null && isDemoting
  )

  if (!member) return null

  const hasChanges = role !== member.role || status !== member.status
  // A member the organization disabled can be re-enabled by the organization; a
  // member the platform switched off cannot, which is why the note below says so
  // rather than leaving the picker to explain it.
  const isPlatformDisabled =
    member.status === 'disabled' && member.disabled_source === 'platform'

  const onSubmit = async (values: EditMemberValues) => {
    const payload: {
      role?: string
      status?: string
      transfer_to_user_id?: number
      reason: string
    } = { reason: values.reason.trim() }

    if (values.role !== member.role) payload.role = values.role
    if (values.status !== member.status) payload.status = values.status
    if (payload.role === undefined && payload.status === undefined) return
    // Only a demotion moves keys; disabling on its own leaves them where they
    // are and switches them off, so a target would be a choice with no effect.
    if (isDemoting && values.transfer_to_user_id !== undefined) {
      payload.transfer_to_user_id = values.transfer_to_user_id
    }

    setIsSubmitting(true)
    try {
      const result = await updateOrganizationMember(
        surface,
        props.organizationId,
        member.user_id,
        payload
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to update the member'))
        return
      }
      toast.success(t('Member updated'))
      props.onOpenChange(false)
      await props.onUpdated()
    } finally {
      setIsSubmitting(false)
    }
  }

  const roleItems = ORGANIZATION_ASSIGNABLE_ROLES.map((role) => ({
    value: role,
    label: t(organizationRoleLabelKey(role)),
  }))
  const statusItems = [
    { value: 'active', label: t('Active') },
    { value: 'disabled', label: t('Disabled') },
  ]

  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open) props.onOpenChange(false)
      }}
      title={t('Edit member')}
      description={t(
        'Change the role or status of this member. The change is recorded in the organization audit log.'
      )}
      bodyClassName='space-y-4'
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
            form={EDIT_MEMBER_FORM_ID}
            disabled={isSubmitting || !hasChanges}
          >
            {isSubmitting ? <Loader2 className='animate-spin' /> : null}
            {isSubmitting ? t('Saving...') : t('Save')}
          </Button>
        </>
      }
    >
      <dl className='grid grid-cols-[auto_1fr] gap-x-3 gap-y-1 text-sm'>
        <dt className='text-muted-foreground'>{t('Username')}</dt>
        <dd className='truncate'>{member.username || '-'}</dd>
        <dt className='text-muted-foreground'>{t('Email')}</dt>
        <dd className='truncate'>{member.email || '-'}</dd>
      </dl>

      <Form {...form}>
        <form
          id={EDIT_MEMBER_FORM_ID}
          onSubmit={form.handleSubmit(onSubmit)}
          className='space-y-4'
        >
          <FormField
            control={form.control}
            name='role'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Role')}</FormLabel>
                <Select
                  items={roleItems}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {roleItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='status'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Status')}</FormLabel>
                <Select
                  items={statusItems}
                  value={field.value}
                  onValueChange={field.onChange}
                >
                  <FormControl>
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                  </FormControl>
                  <SelectContent>
                    {statusItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>
                        {item.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {status === 'disabled' && (
                  <FormDescription>
                    {t(
                      'Their API keys in this organization stop working. Disabling from here is recorded as a platform action, so the organization cannot undo it by itself.'
                    )}
                  </FormDescription>
                )}
                <FormMessage />
              </FormItem>
            )}
          />

          {isPlatformDisabled && (
            <p className='text-muted-foreground text-sm'>
              {t(
                'This member was disabled by the platform. The organization cannot re-enable them by itself.'
              )}
            </p>
          )}

          {isDemoting && (
            <TransferTargetField
              control={form.control}
              options={options}
              isLoading={isLoading}
              emptyOptionLabel={t('Transfer responsible API keys to the owner')}
            />
          )}

          <FormField
            control={form.control}
            name='reason'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Reason')}</FormLabel>
                <FormControl>
                  <Textarea
                    rows={3}
                    placeholder={t('Why is this change being made?')}
                    {...field}
                  />
                </FormControl>
                <FormDescription>
                  {t('Recorded in the organization audit log.')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />
        </form>
      </Form>
    </Dialog>
  )
}

// ============================================================================
// Leave Organization
// ============================================================================

/**
 * Leaves the organization.
 *
 * The caller is the subject, so there is nothing to fill in — the backend
 * decides what happens to the keys they were responsible for. Succeeding means
 * the page they are standing on no longer exists for them, which the caller is
 * responsible for handling.
 */
export function OrganizationExitDialog(props: {
  organizationId: number
  open: boolean
  onOpenChange: (open: boolean) => void
  onExited: () => Promise<unknown> | unknown
}) {
  const { t } = useTranslation()
  const [isSubmitting, setIsSubmitting] = useState(false)

  const handleConfirm = async () => {
    setIsSubmitting(true)
    try {
      const result = await exitOrganization(props.organizationId)
      if (!result.success) {
        toast.error(result.message || t('Failed to leave the organization'))
        return
      }
      toast.success(t('Left the organization'))
      props.onOpenChange(false)
      await props.onExited()
    } finally {
      setIsSubmitting(false)
    }
  }

  return (
    <ConfirmDialog
      open={props.open}
      onOpenChange={props.onOpenChange}
      title={t('Leave organization')}
      desc={t(
        'You will lose access to this organization immediately. Responsibility for any API keys you hold is transferred by policy.'
      )}
      confirmText={t('Leave')}
      destructive
      isLoading={isSubmitting}
      handleConfirm={() => void handleConfirm()}
    />
  )
}
