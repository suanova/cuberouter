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
import type { ColumnDef } from '@tanstack/react-table'
import { Loader2, MailPlus, RotateCw, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ConfirmDialog } from '@/components/confirm-dialog'
import { DataTablePage, useDataTable } from '@/components/data-table'
import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import {
  DropdownMenuItem,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import { StatusBadge } from '@/components/status-badge'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatTimestamp } from '@/lib/format'

import {
  createOrganizationInvite,
  listOrganizationInvites,
  revokeOrganizationInvite,
} from '../api'
import { organizationRoleLabelKey } from '../constants'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import {
  organizationInviteCanRetry,
  organizationInviteDeliveryErrorMessageKey,
  organizationInviteStatusMeta,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationInviteRow } from '../types'
import { OrganizationInviteDialog } from './organization-invite-dialog'
import {
  useOrganizationSectionRoute,
  useOrganizationSurface,
} from './organization-page-provider'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
  OrganizationSectionRefresh,
} from './organization-section'

const INVITES_COLUMN_VISIBILITY_STORAGE_KEY = 'organization-invites-column-visibility'

type OrganizationInvitesSectionProps = {
  organizationId: number
  canViewInvites: boolean
  canCreateInvites: boolean
  canRevokeInvites: boolean
  /** The caller may not write to this organization. */
  readOnly: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * Invitations to the organization, and how each one is actually doing.
 *
 * A pending row is not a settled fact: the record is written before the mail is
 * sent, so the status column folds in the delivery outcome — see
 * `organizationInviteStatusMeta` — and a failed send can be retried from here
 * without going back to the invitation form.
 */
export function OrganizationInvitesSection(
  props: OrganizationInvitesSectionProps
) {
  const { t } = useTranslation()
  const { search, navigate } = useOrganizationSectionRoute()
  const surface = useOrganizationSurface()
  const [isInviteOpen, setIsInviteOpen] = useState(false)
  const [revoking, setRevoking] = useState<OrganizationInviteRow | null>(null)
  const [isRevoking, setIsRevoking] = useState(false)
  const [retryingId, setRetryingId] = useState<number | null>(null)

  const { pagination, onPaginationChange, ensurePageInRange } = useTableUrlState({
    search,
    navigate,
    pagination: {
      defaultPage: 1,
      defaultPageSize: ORGANIZATION_DEFAULT_PAGE_SIZE,
    },
    globalFilter: { enabled: false },
    columnFilters: [],
  })

  const params = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
  }

  const invites = useOrganizationPagedSection<OrganizationInviteRow>({
    surface,
    organizationId: props.organizationId,
    resource: 'invitations',
    params,
    query: () => listOrganizationInvites(props.organizationId, params),
    enabled: props.canViewInvites,
    onForbidden: props.onForbidden,
  })

  const handleRevoke = async () => {
    if (!revoking) return
    setIsRevoking(true)
    try {
      const result = await revokeOrganizationInvite(
        props.organizationId,
        revoking.id
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to revoke the invitation'))
        return
      }
      toast.success(t('Invitation revoked'))
      setRevoking(null)
      invites.refetch()
    } finally {
      setIsRevoking(false)
    }
  }

  /**
   * Sends the invitation again.
   *
   * A retry is a fresh send rather than a resend of the same link, and the list
   * is re-read even when it fails: the failed attempt may still have written a
   * record, so what is on screen is no longer trustworthy either way.
   */
  const handleRetry = async (invite: OrganizationInviteRow) => {
    setRetryingId(invite.id)
    try {
      const result = await createOrganizationInvite(props.organizationId, {
        email: invite.target_email,
        role: invite.role,
      })
      if (result.success) {
        toast.success(t('Invitation sent'))
      } else {
        toast.error(result.message || t('Failed to resend the invitation'))
      }
    } catch (error) {
      const messageKey = organizationInviteDeliveryErrorMessageKey(error)
      toast.error(messageKey ? t(messageKey) : t('Failed to resend the invitation'))
    } finally {
      setRetryingId(null)
      invites.refetch()
    }
  }

  const columns = useMemo<ColumnDef<OrganizationInviteRow>[]>(
    () =>
      buildInviteColumns({
        t,
        readOnly: props.readOnly,
        canCreateInvites: props.canCreateInvites,
        canRevokeInvites: props.canRevokeInvites,
        retryingId,
        onRetry: (invite) => void handleRetry(invite),
        onRevoke: (invite) => setRevoking(invite),
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      t,
      props.readOnly,
      props.canCreateInvites,
      props.canRevokeInvites,
      retryingId,
    ]
  )

  const { table } = useDataTable({
    data: invites.items,
    columns,
    pagination,
    onPaginationChange,
    manualPagination: true,
    totalCount: invites.total,
    ensurePageInRange,
    columnVisibilityStorageKey: INVITES_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  if (!props.canViewInvites) {
    return (
      <OrganizationSectionEmpty
        icon='invitations'
        title={t('Invitations')}
        message={t(
          'Only an organization owner or administrator can view invitations.'
        )}
      />
    )
  }

  return (
    <OrganizationSection
      icon='invitations'
      title={t('Invitations')}
      description={t('Invite people to the organization and track each one.')}
      count={invites.total}
      actions={
        <div className='flex items-center gap-2'>
          <OrganizationSectionRefresh
            onClick={() => invites.refetch()}
            isFetching={invites.isFetching}
          />
          {props.canCreateInvites && !props.readOnly ? (
            <Button size='sm' onClick={() => setIsInviteOpen(true)}>
              <MailPlus data-icon='inline-start' />
              {t('Invite member')}
            </Button>
          ) : null}
        </div>
      }
    >
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={invites.isLoading}
        isFetching={invites.isFetching}
        emptyTitle={t('No invitations yet')}
        skeletonKeyPrefix='organization-invites-skeleton'
        applyHeaderSize
        // The endpoint takes paging and nothing else, so there is no filter or
        // search surface to render.
        toolbarProps={null}
      />

      <OrganizationInviteDialog
        organizationId={props.organizationId}
        open={isInviteOpen}
        onOpenChange={setIsInviteOpen}
        onCreated={() => invites.refetch()}
      />

      <ConfirmDialog
        open={revoking !== null}
        onOpenChange={(open) => !open && setRevoking(null)}
        title={t('Revoke invitation')}
        desc={t(
          'The invitation link stops working immediately. The person can still be invited again afterwards.'
        )}
        confirmText={t('Revoke')}
        destructive
        isLoading={isRevoking}
        handleConfirm={() => void handleRevoke()}
      />
    </OrganizationSection>
  )
}

type InviteColumnContext = {
  t: (key: string) => string
  readOnly: boolean
  canCreateInvites: boolean
  canRevokeInvites: boolean
  retryingId: number | null
  onRetry: (invite: OrganizationInviteRow) => void
  onRevoke: (invite: OrganizationInviteRow) => void
}

function buildInviteColumns(
  context: InviteColumnContext
): ColumnDef<OrganizationInviteRow>[] {
  const { t } = context

  return [
    {
      accessorKey: 'target_email',
      header: t('Email'),
      enableSorting: false,
      size: 240,
      cell: ({ row }) => (
        <span className='whitespace-nowrap'>
          {row.original.target_email || '-'}
        </span>
      ),
    },
    {
      accessorKey: 'role',
      header: t('Role'),
      enableSorting: false,
      size: 120,
      cell: ({ row }) => <span>{t(organizationRoleLabelKey(row.original.role))}</span>,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => {
        const meta = organizationInviteStatusMeta(
          row.original.status,
          row.original.delivery_status
        )
        return (
          <StatusBadge
            label={t(meta.labelKey)}
            variant={meta.variant}
            copyable={false}
          />
        )
      },
    },
    {
      accessorKey: 'created_at',
      header: t('Created At'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => <InviteTimestamp value={row.original.created_at} />,
    },
    {
      accessorKey: 'updated_at',
      header: t('Updated At'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => <InviteTimestamp value={row.original.updated_at} />,
    },
    {
      id: 'actions',
      header: t('Actions'),
      enableSorting: false,
      enableHiding: false,
      size: 80,
      cell: ({ row }) => (
        <InviteRowActions context={context} invite={row.original} />
      ),
    },
  ]
}

function InviteTimestamp(props: { value?: number }) {
  return (
    <span className='text-muted-foreground text-sm whitespace-nowrap'>
      {props.value ? formatTimestamp(props.value) : '-'}
    </span>
  )
}

/**
 * Actions exist only while the invitation is pending: once it has been accepted,
 * expired or revoked, nothing here can change it.
 *
 * Retry is offered for a send that failed or whose outcome is unknown, and
 * revoke for anything still pending — including one whose send failed, since the
 * record is what the link resolves to.
 */
function InviteRowActions(props: {
  context: InviteColumnContext
  invite: OrganizationInviteRow
}) {
  const { t } = useTranslation()
  const { context, invite } = props

  if (invite.status !== 'pending' || context.readOnly) return null

  const canRetry = organizationInviteCanRetry(
    invite,
    context.canCreateInvites,
    context.readOnly
  )
  if (!canRetry && !context.canRevokeInvites) return null

  return (
    <DataTableRowActionMenu ariaLabel={t('Open menu')} contentClassName='w-40'>
      {canRetry ? (
        <DropdownMenuItem
          onSelect={() => context.onRetry(invite)}
          disabled={context.retryingId === invite.id}
        >
          {t('Retry')}
          <DropdownMenuShortcut>
            {context.retryingId === invite.id ? (
              <Loader2 className='animate-spin' size={16} />
            ) : (
              <RotateCw size={16} />
            )}
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      ) : null}

      <DropdownMenuItem
        onSelect={() => context.onRevoke(invite)}
        className='text-destructive focus:text-destructive'
        disabled={!context.canRevokeInvites}
      >
        {t('Revoke')}
        <DropdownMenuShortcut>
          <Trash2 size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>
    </DataTableRowActionMenu>
  )
}
