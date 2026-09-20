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
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { Power, PowerOff, Trash2, UserPlus } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import { Button } from '@/components/ui/button'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { StatusBadge } from '@/components/status-badge'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatTimestamp } from '@/lib/format'

import { listOrganizationMembers, updateOrganizationMember } from '../api'
import {
  ORGANIZATION_ASSIGNABLE_ROLES,
  organizationMemberStatusMeta,
  organizationRoleLabelKey,
} from '../constants'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import {
  buildOrganizationMemberRoleUpdatePayload,
  getOrganizationMemberActionFlags,
  isOrganizationMemberDemotion,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationMemberRow } from '../types'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
  OrganizationSectionRefresh,
} from './organization-section'
import {
  OrganizationAddMemberDialog,
  OrganizationDemoteAdminDialog,
  OrganizationExitDialog,
  OrganizationRemoveMemberDialog,
} from './organization-member-dialogs'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

const MEMBERS_COLUMN_VISIBILITY_STORAGE_KEY = 'organization-members-column-visibility'

type OrganizationMembersSectionProps = {
  organizationId: number
  /** The caller's own organization role; empty for an outside administrator. */
  actorRole: string
  /** The caller's platform user id, so their own row can be left alone. */
  currentUserId: number
  /** The caller may read the organization at all. */
  canView: boolean
  canManageMembers: boolean
  canAddMembersDirectly: boolean
  canExitOrganization: boolean
  /** The caller may not write to this organization. */
  readOnly: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
  /** Leaves the page after the caller leaves the organization themselves. */
  onLeftOrganization: () => Promise<unknown> | unknown
}

/**
 * Who belongs to the organization, and what each of them may do.
 *
 * The list has no filters because the endpoint has none — it takes paging and
 * nothing else — so the toolbar is replaced by the section's own actions rather
 * than showing a search box that would filter nothing.
 */
export function OrganizationMembersSection(props: OrganizationMembersSectionProps) {
  const { t } = useTranslation()
  const [isAddOpen, setIsAddOpen] = useState(false)
  const [isExiting, setIsExiting] = useState(false)
  const [removing, setRemoving] = useState<OrganizationMemberRow | null>(null)
  const [demoting, setDemoting] = useState<OrganizationMemberRow | null>(null)

  const {
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
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

  const members = useOrganizationPagedSection<OrganizationMemberRow>({
    organizationId: props.organizationId,
    resource: 'members',
    params,
    query: () => listOrganizationMembers(props.organizationId, params),
    enabled: props.canView,
    onForbidden: props.onForbidden,
  })

  /**
   * A demotion can strand the keys an administrator was responsible for, so the
   * backend refuses it without a transfer target — which is why the role select
   * opens a dialog instead of submitting straight away. A promotion has no such
   * consequence and goes through immediately.
   */
  const changeRole = async (member: OrganizationMemberRow, nextRole: string) => {
    if (isOrganizationMemberDemotion(member, nextRole)) {
      setDemoting(member)
      return
    }
    const result = await updateOrganizationMember(
      props.organizationId,
      member.user_id,
      buildOrganizationMemberRoleUpdatePayload(member, nextRole)
    )
    if (!result.success) {
      toast.error(result.message || t('Failed to update the member'))
      return
    }
    toast.success(t('Member updated'))
    members.refetch()
  }

  const changeStatus = async (
    member: OrganizationMemberRow,
    status: 'active' | 'disabled'
  ) => {
    const result = await updateOrganizationMember(
      props.organizationId,
      member.user_id,
      { status }
    )
    if (!result.success) {
      toast.error(result.message || t('Failed to update the member'))
      return
    }
    toast.success(t('Member updated'))
    members.refetch()
  }

  const columns = useMemo<ColumnDef<OrganizationMemberRow>[]>(
    () =>
      buildMemberColumns({
        t,
        // The role select follows the per-row flags, so an actor who may not
        // touch this particular row sees a plain label rather than a control
        // that would be rejected on submit.
        canManageMembers: props.canManageMembers,
        readOnly: props.readOnly,
        actorRole: props.actorRole,
        currentUserId: props.currentUserId,
        onChangeRole: (member, nextRole) => void changeRole(member, nextRole),
        onChangeStatus: (member, status) => void changeStatus(member, status),
        onRemove: (member) => setRemoving(member),
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      t,
      props.canManageMembers,
      props.readOnly,
      props.actorRole,
      props.currentUserId,
      props.organizationId,
      members.refetch,
    ]
  )

  const { table } = useDataTable({
    data: members.items,
    columns,
    pagination,
    onPaginationChange,
    manualPagination: true,
    totalCount: members.total,
    ensurePageInRange,
    columnVisibilityStorageKey: MEMBERS_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  if (!props.canView) {
    return (
      <OrganizationSectionEmpty
        icon='members'
        title={t('Members')}
        message={t('You do not have access to this organization.')}
      />
    )
  }

  const canSelfLeave = props.canExitOrganization && !props.readOnly

  return (
    <OrganizationSection
      icon='members'
      title={t('Members')}
      description={t('Everyone who belongs to this organization, and their role.')}
      count={members.total}
      actions={
        <div className='flex items-center gap-2'>
          <OrganizationSectionRefresh
            onClick={() => void members.refetch()}
            isFetching={members.isFetching}
          />
          {props.canAddMembersDirectly && !props.readOnly ? (
            <Button size='sm' onClick={() => setIsAddOpen(true)}>
              <UserPlus data-icon='inline-start' />
              {t('Add member')}
            </Button>
          ) : null}
          {canSelfLeave ? (
            <Button
              size='sm'
              variant='outline'
              onClick={() => setIsExiting(true)}
            >
              {t('Leave organization')}
            </Button>
          ) : null}
        </div>
      }
    >
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={members.isLoading}
        isFetching={members.isFetching}
        emptyTitle={t('No members yet')}
        skeletonKeyPrefix='organization-members-skeleton'
        applyHeaderSize
        // The endpoint accepts paging and nothing else, so there is no filter
        // or search surface to render.
        toolbarProps={null}
      />

      <OrganizationAddMemberDialog
        organizationId={props.organizationId}
        open={isAddOpen}
        onOpenChange={setIsAddOpen}
        onAdded={() => members.refetch()}
      />

      <OrganizationRemoveMemberDialog
        organizationId={props.organizationId}
        member={removing}
        onOpenChange={(open) => !open && setRemoving(null)}
        onRemoved={() => members.refetch()}
      />

      <OrganizationDemoteAdminDialog
        organizationId={props.organizationId}
        member={demoting}
        onOpenChange={(open) => !open && setDemoting(null)}
        onDemoted={() => members.refetch()}
      />

      <OrganizationExitDialog
        organizationId={props.organizationId}
        open={isExiting}
        onOpenChange={setIsExiting}
        onExited={props.onLeftOrganization}
      />
    </OrganizationSection>
  )
}

type MemberColumnContext = {
  t: (key: string) => string
  actorRole: string
  currentUserId: number
  canManageMembers: boolean
  readOnly: boolean
  onChangeRole: (member: OrganizationMemberRow, nextRole: string) => void
  onChangeStatus: (
    member: OrganizationMemberRow,
    status: 'active' | 'disabled'
  ) => void
  onRemove: (member: OrganizationMemberRow) => void
}

function buildMemberColumns(
  context: MemberColumnContext
): ColumnDef<OrganizationMemberRow>[] {
  const { t } = context

  return [
    {
      accessorKey: 'username',
      header: t('Username'),
      enableSorting: false,
      size: 200,
      cell: ({ row }) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>
            {row.original.display_name || row.original.username || '-'}
          </div>
          <div className='text-muted-foreground truncate text-xs'>
            {row.original.username || '-'}
          </div>
        </div>
      ),
    },
    {
      accessorKey: 'email',
      header: t('Email'),
      enableSorting: false,
      size: 240,
      cell: ({ row }) => (
        <span className='whitespace-nowrap'>{row.original.email || '-'}</span>
      ),
    },
    {
      accessorKey: 'role',
      header: t('Role'),
      enableSorting: false,
      size: 150,
      cell: ({ row }) => {
        const flags = memberActionFlags(context, row.original)
        if (!flags.canChangeRole) {
          return t(organizationRoleLabelKey(row.original.role))
        }
        return (
          <Select
            value={row.original.role}
            onValueChange={(value) =>
              // The select can report a cleared value; a role is never absent,
              // so there is nothing to submit in that case.
              value !== null && context.onChangeRole(row.original, value)
            }
          >
            <SelectTrigger size='sm' className='w-[120px]'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ORGANIZATION_ASSIGNABLE_ROLES.map((role) => (
                <SelectItem key={role} value={role}>
                  {t(organizationRoleLabelKey(role))}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )
      },
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      enableSorting: false,
      size: 130,
      cell: ({ row }) => {
        const meta = organizationMemberStatusMeta(row.original.status)
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
      header: t('Joined At'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {row.original.created_at
            ? formatTimestamp(row.original.created_at)
            : '-'}
        </span>
      ),
    },
    {
      id: 'actions',
      header: t('Actions'),
      enableSorting: false,
      enableHiding: false,
      size: 80,
      cell: ({ row }) => <MemberRowActions context={context} member={row.original} />,
    },
  ]
}

function memberActionFlags(
  context: MemberColumnContext,
  member: OrganizationMemberRow
) {
  return getOrganizationMemberActionFlags({
    actorRole: context.actorRole,
    targetRole: member.role,
    canManageMembers: context.canManageMembers,
    readOnly: context.readOnly,
    isCurrentUser: member.user_id === context.currentUserId,
  })
}

/**
 * The row's own menu, which is absent rather than disabled when the actor may
 * not touch this row: an owner's row and one's own row are both untouchable, and
 * a greyed-out menu would suggest otherwise.
 */
function MemberRowActions(props: {
  context: MemberColumnContext
  member: OrganizationMemberRow
}) {
  const { t } = useTranslation()
  const flags = memberActionFlags(props.context, props.member)

  if (!flags.canChangeStatus && !flags.canRemove) return null

  return (
    <DataTableRowActionMenu ariaLabel={t('Open menu')} contentClassName='w-40'>
      {props.member.status === 'active' ? (
        <DropdownMenuItem
          onSelect={() => props.context.onChangeStatus(props.member, 'disabled')}
          disabled={!flags.canChangeStatus}
        >
          {t('Disable')}
          <DropdownMenuShortcut>
            <PowerOff size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      ) : (
        <DropdownMenuItem
          onSelect={() => props.context.onChangeStatus(props.member, 'active')}
          disabled={!flags.canChangeStatus}
        >
          {t('Enable')}
          <DropdownMenuShortcut>
            <Power size={16} />
          </DropdownMenuShortcut>
        </DropdownMenuItem>
      )}

      <DropdownMenuSeparator />

      <DropdownMenuItem
        onSelect={() => props.context.onRemove(props.member)}
        className='text-destructive focus:text-destructive'
        disabled={!flags.canRemove}
      >
        {t('Remove')}
        <DropdownMenuShortcut>
          <Trash2 size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>
    </DataTableRowActionMenu>
  )
}
