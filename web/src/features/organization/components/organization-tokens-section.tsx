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
import { useQuery } from '@tanstack/react-query'
import { getRouteApi } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { Plus, Trash2 } from 'lucide-react'
import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import {
  DataTablePage,
  DISABLED_ROW_DESKTOP,
  DISABLED_ROW_MOBILE,
  useDataTable,
} from '@/components/data-table'
import { BadgeListCell } from '@/components/data-table/core/badge-list-cell'
import { DataTableRowActionMenu } from '@/components/data-table/core/row-action-menu'
import { StatusBadge } from '@/components/status-badge'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuShortcut,
} from '@/components/ui/dropdown-menu'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatQuota, formatTimestamp } from '@/lib/format'

import {
  getOrganizationGroups,
  listOrganizationTokens,
  updateOrganizationToken,
} from '../api'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import { useOrganizationTokenResponsibleOptions } from '../hooks/use-organization-member-options'
import {
  buildOrganizationGroupOptions,
  buildOrganizationTokenStatusPayload,
  getOrganizationTokenActionFlags,
  organizationTokenErrorText,
  organizationTokenStatusMeta,
  organizationTokenVisibilityMeta,
  ORGANIZATION_TOKEN_STATUS,
  ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS,
  ORGANIZATION_TOKEN_VISIBILITIES,
  type OrganizationGroupOption,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import type { OrganizationTokenRow } from '../types'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
  OrganizationSectionRefresh,
} from './organization-section'
import { OrganizationTokenKeyCell } from './organization-token-key-cell'
import { OrganizationTokensBulkActions } from './organization-token-bulk-actions'
import { OrganizationTokenDeleteDialog } from './organization-token-delete-dialogs'
import { OrganizationTokenFormDialog } from './organization-token-form-dialog'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

const TOKENS_COLUMN_VISIBILITY_STORAGE_KEY = 'organization-tokens-column-visibility'

/** The backend filters on one value per field, so a multi-select sends its first. */
function firstFilterValue(
  columnFilters: Array<{ id: string; value: unknown }>,
  columnId: string
): string | undefined {
  const value = columnFilters.find((filter) => filter.id === columnId)?.value
  if (Array.isArray(value)) return value[0] as string | undefined
  return typeof value === 'string' && value ? value : undefined
}

type OrganizationTokensSectionProps = {
  organizationId: number
  /** The organization's own group, which a key with none of its own inherits. */
  organizationGroup: string
  /** The caller may read the organization's keys. */
  canView: boolean
  /** The caller may publish keys and hand them to anyone; otherwise only their own. */
  canManageAllTokens: boolean
  currentUserId: number
  isOrganizationMember: boolean
  /** The caller may not write to this organization. */
  readOnly: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * The organization's API keys.
 *
 * What is shown depends on who is looking. A manager sees every key in the
 * organization; an ordinary member sees their own private keys and the public
 * ones, because that is what the backend answers — the list is narrowed at the
 * source rather than filtered here, so the row count is honest. The per-row
 * actions follow the same split: a member may change a private key they hold,
 * but a public key is usable by all and editable by none of them.
 */
export function OrganizationTokensSection(props: OrganizationTokensSectionProps) {
  const { t } = useTranslation()
  const [creating, setCreating] = useState(false)
  const [editing, setEditing] = useState<OrganizationTokenRow | null>(null)
  const [deleting, setDeleting] = useState<OrganizationTokenRow | null>(null)

  const {
    globalFilter,
    onGlobalFilterChange,
    pagination,
    onPaginationChange,
    columnFilters,
    onColumnFiltersChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: {
      defaultPage: 1,
      defaultPageSize: ORGANIZATION_DEFAULT_PAGE_SIZE,
    },
    globalFilter: { enabled: true, key: 'tokenFilter' },
    columnFilters: [
      { columnId: 'status', searchKey: 'tokenStatus', type: 'array' },
      { columnId: 'visibility', searchKey: 'tokenVisibility', type: 'array' },
      { columnId: 'group', searchKey: 'tokenGroup', type: 'array' },
      {
        columnId: 'responsible_user_id',
        searchKey: 'tokenResponsible',
        type: 'array',
      },
    ],
  })

  const params = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    keyword: globalFilter || undefined,
    status: Number.parseInt(firstFilterValue(columnFilters, 'status') ?? '', 10) || undefined,
    visibility: firstFilterValue(columnFilters, 'visibility'),
    group: firstFilterValue(columnFilters, 'group'),
    responsible_user_id:
      Number.parseInt(
        firstFilterValue(columnFilters, 'responsible_user_id') ?? '',
        10
      ) || undefined,
  }

  const tokens = useOrganizationPagedSection<OrganizationTokenRow>({
    organizationId: props.organizationId,
    resource: 'tokens',
    params,
    query: () => listOrganizationTokens(props.organizationId, params),
    enabled: props.canView,
    onForbidden: props.onForbidden,
  })

  // The group picker offers what the organization may actually use: the backend
  // refuses a key whose group is outside that set, so listing every group a
  // deployment defines would offer choices that cannot be saved.
  const { data: groupsData } = useQuery({
    queryKey: ['organization-groups', props.organizationId],
    queryFn: () => getOrganizationGroups(props.organizationId),
    enabled: props.canView,
    staleTime: 5 * 60 * 1000,
  })
  const groupOptions = useMemo<OrganizationGroupOption[]>(
    () => buildOrganizationGroupOptions(groupsData?.data),
    [groupsData]
  )

  // The responsible-user filter lists anyone eligible to hold a key; a public
  // key only ever belongs to an owner or an administrator.
  const { options: responsibleFilterOptions } =
    useOrganizationTokenResponsibleOptions(
      props.organizationId,
      'private',
      props.canView
    )

  const flagsFor = (token: OrganizationTokenRow) =>
    getOrganizationTokenActionFlags(token, {
      canManageAllTokens: props.canManageAllTokens,
      currentUserId: props.currentUserId,
      readOnly: props.readOnly,
    })

  /**
   * Enabling re-checks the responsible member, so it can be refused for a reason
   * that has nothing to do with the caller — the holder was disabled, or the
   * platform account was. That reason is surfaced rather than being flattened
   * into a generic failure.
   */
  const changeStatus = async (token: OrganizationTokenRow, status: number) => {
    try {
      const result = await updateOrganizationToken(
        props.organizationId,
        token.id,
        buildOrganizationTokenStatusPayload(token, status)
      )
      if (!result.success) {
        toast.error(result.message || t('Failed to update organization key'))
        return
      }
      toast.success(
        status === ORGANIZATION_TOKEN_STATUS.ENABLED
          ? t('Organization key enabled')
          : t('Organization key disabled')
      )
      tokens.refetch()
    } catch (error) {
      toast.error(
        organizationTokenErrorText(
          error,
          t('Failed to update organization key')
        )
      )
    }
  }

  const columns = useMemo<ColumnDef<OrganizationTokenRow>[]>(
    () =>
      buildTokenColumns({
        t,
        organizationGroup: props.organizationGroup,
        groupOptions,
        flagsFor,
        onEdit: (token) => setEditing(token),
        onDelete: (token) => setDeleting(token),
        onChangeStatus: (token, status) => void changeStatus(token, status),
      }),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [
      t,
      props.organizationGroup,
      props.organizationId,
      props.canManageAllTokens,
      props.currentUserId,
      props.readOnly,
      groupOptions,
      tokens.refetch,
    ]
  )

  const { table } = useDataTable({
    data: tokens.items,
    columns,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: tokens.total,
    enableRowSelection: true,
    // Rows are identified by key id so a selection survives a refetch, which is
    // what the bulk actions act on.
    getRowId: (row) => String(row.id),
    globalFilter,
    onGlobalFilterChange,
    columnFilters,
    onColumnFiltersChange,
    ensurePageInRange,
    columnVisibilityStorageKey: TOKENS_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  if (!props.canView) {
    return (
      <OrganizationSectionEmpty
        icon='tokens'
        title={t('API Keys')}
        message={t('You do not have access to this organization.')}
      />
    )
  }

  return (
    <OrganizationSection
      icon='tokens'
      title={t('API Keys')}
      description={t(
        'Keys that spend the organization quota. A key belongs to the organization, not to the member holding it.'
      )}
      count={tokens.total}
      actions={
        <div className='flex items-center gap-2'>
          <OrganizationSectionRefresh
            onClick={() => void tokens.refetch()}
            isFetching={tokens.isFetching}
          />
          {!props.readOnly ? (
            <Button size='sm' onClick={() => setCreating(true)}>
              <Plus data-icon='inline-start' />
              {t('Create key')}
            </Button>
          ) : null}
        </div>
      }
    >
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={tokens.isLoading}
        isFetching={tokens.isFetching}
        emptyTitle={t('No keys yet')}
        emptyDescription={t(
          'No organization keys match the current filters.'
        )}
        skeletonKeyPrefix='organization-tokens-skeleton'
        applyHeaderSize
        toolbarProps={{
          searchPlaceholder: t('Search by key name...'),
          searchDebounceMs: 500,
          filters: [
            {
              columnId: 'status',
              title: t('Status'),
              singleSelect: true,
              options: ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS.map((option) => ({
                value: option.value,
                label: t(option.labelKey),
              })),
            },
            {
              columnId: 'visibility',
              title: t('Visibility'),
              singleSelect: true,
              options: Object.entries(ORGANIZATION_TOKEN_VISIBILITIES).map(
                ([value, meta]) => ({
                  value,
                  label: t(meta.labelKey),
                })
              ),
            },
            {
              columnId: 'group',
              title: t('Token group'),
              singleSelect: true,
              options: groupOptions.map((option) => ({
                value: option.value,
                label: option.label,
              })),
            },
            ...(props.canManageAllTokens
              ? [
                  {
                    columnId: 'responsible_user_id',
                    title: t('Responsible user'),
                    singleSelect: true,
                    options: responsibleFilterOptions.map((option) => ({
                      value: String(option.value),
                      label: option.label,
                    })),
                  },
                ]
              : []),
          ],
        }}
        // A disabled key is dimmed, because a row that still looks usable is
        // the one kind of row in this table that would be misread.
        getRowClassName={(row, { isMobile }) => {
          if (row.original.status !== ORGANIZATION_TOKEN_STATUS.DISABLED) {
            return undefined
          }
          return isMobile ? DISABLED_ROW_MOBILE : DISABLED_ROW_DESKTOP
        }}
        bulkActions={
          <OrganizationTokensBulkActions
            organizationId={props.organizationId}
            table={table}
            canDeleteToken={(token) => flagsFor(token).canDelete}
            canCopyToken={(token) => flagsFor(token).canCopy}
            onDeleted={() => tokens.refetch()}
          />
        }
      />

      <OrganizationTokenFormDialog
        organizationId={props.organizationId}
        open={creating}
        onOpenChange={setCreating}
        editing={null}
        canManageAllTokens={props.canManageAllTokens}
        currentUserId={props.currentUserId}
        isOrganizationMember={props.isOrganizationMember}
        groupOptions={groupOptions}
        onSaved={() => tokens.refetch()}
      />

      <OrganizationTokenFormDialog
        organizationId={props.organizationId}
        open={editing !== null}
        onOpenChange={(open) => !open && setEditing(null)}
        editing={editing}
        canManageAllTokens={props.canManageAllTokens}
        currentUserId={props.currentUserId}
        isOrganizationMember={props.isOrganizationMember}
        groupOptions={groupOptions}
        onSaved={() => tokens.refetch()}
      />

      <OrganizationTokenDeleteDialog
        organizationId={props.organizationId}
        token={deleting}
        onOpenChange={(open) => !open && setDeleting(null)}
        onDeleted={() => tokens.refetch()}
      />
    </OrganizationSection>
  )
}

type TokenColumnContext = {
  t: (key: string) => string
  organizationGroup: string
  groupOptions: OrganizationGroupOption[]
  flagsFor: (token: OrganizationTokenRow) => ReturnType<
    typeof getOrganizationTokenActionFlags
  >
  onEdit: (token: OrganizationTokenRow) => void
  onDelete: (token: OrganizationTokenRow) => void
  onChangeStatus: (token: OrganizationTokenRow, status: number) => void
}

function buildTokenColumns(
  context: TokenColumnContext
): ColumnDef<OrganizationTokenRow>[] {
  const { t } = context

  return [
    {
      accessorKey: 'name',
      header: t('Name'),
      enableSorting: false,
      size: 160,
      cell: ({ row }) => (
        <div className='min-w-0'>
          <div className='truncate font-medium'>{row.original.name || '-'}</div>
          <div className='text-muted-foreground text-xs'>#{row.original.id}</div>
        </div>
      ),
    },
    {
      accessorKey: 'key',
      header: t('Key'),
      enableSorting: false,
      size: 240,
      cell: ({ row }) => (
        <OrganizationTokenKeyCell
          token={row.original}
          canCopy={context.flagsFor(row.original).canCopy}
        />
      ),
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => {
        const meta = organizationTokenStatusMeta(row.original.status)
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
      accessorKey: 'responsible_user_id',
      header: t('Responsible user'),
      enableSorting: false,
      size: 140,
      cell: ({ row }) => (
        <span className='truncate font-medium'>
          {row.original.responsible_username ||
            row.original.responsible_display_name ||
            '-'}
        </span>
      ),
    },
    {
      accessorKey: 'visibility',
      header: t('Visibility'),
      enableSorting: false,
      size: 110,
      cell: ({ row }) => (
        <span className='text-sm'>
          {t(organizationTokenVisibilityMeta(row.original.visibility).labelKey)}
        </span>
      ),
    },
    {
      accessorKey: 'group',
      header: t('Token group'),
      enableSorting: false,
      size: 140,
      cell: ({ row }) => {
        const group = row.original.group || context.organizationGroup || 'default'
        const option = context.groupOptions.find((item) => item.value === group)
        return (
          <div className='min-w-0'>
            <div className='truncate'>{group}</div>
            {option?.desc && option.desc !== group ? (
              <div className='text-muted-foreground truncate text-xs'>
                {option.desc}
              </div>
            ) : null}
          </div>
        )
      },
    },
    {
      accessorKey: 'remain_quota',
      header: t('Remaining quota'),
      enableSorting: false,
      size: 130,
      cell: ({ row }) =>
        row.original.unlimited_quota ? (
          <Badge variant='secondary'>{t('Unlimited')}</Badge>
        ) : (
          <span className='whitespace-nowrap tabular-nums'>
            {formatQuota(row.original.remain_quota ?? 0)}
          </span>
        ),
    },
    {
      accessorKey: 'used_quota',
      header: t('Used quota'),
      enableSorting: false,
      size: 120,
      cell: ({ row }) => (
        <span className='whitespace-nowrap tabular-nums'>
          {formatQuota(row.original.used_quota ?? 0)}
        </span>
      ),
    },
    {
      accessorKey: 'model_limits',
      header: t('Model limits'),
      enableSorting: false,
      size: 160,
      cell: ({ row }) => {
        if (!row.original.model_limits_enabled || !row.original.model_limits) {
          return (
            <span className='text-muted-foreground text-xs'>{t('No limit')}</span>
          )
        }
        const models = row.original.model_limits.split(',').filter(Boolean)
        return (
          <BadgeListCell
            items={models.map((model) => (
              <Badge key={model} variant='outline' className='font-mono text-xs'>
                {model}
              </Badge>
            ))}
          />
        )
      },
    },
    {
      accessorKey: 'allow_ips',
      header: t('IP limits'),
      enableSorting: false,
      size: 160,
      cell: ({ row }) => {
        const ips = String(row.original.allow_ips ?? '')
          .split('\n')
          .map((ip) => ip.trim())
          .filter(Boolean)
        if (ips.length === 0) {
          return (
            <span className='text-muted-foreground text-xs'>{t('No limit')}</span>
          )
        }
        return (
          <BadgeListCell
            items={ips.map((ip) => (
              <Badge key={ip} variant='outline' className='font-mono text-xs'>
                {ip}
              </Badge>
            ))}
          />
        )
      },
    },
    {
      accessorKey: 'created_time',
      header: t('Created At'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {row.original.created_time
            ? formatTimestamp(row.original.created_time)
            : '-'}
        </span>
      ),
    },
    {
      accessorKey: 'accessed_time',
      header: t('Last Used'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {row.original.accessed_time
            ? formatTimestamp(row.original.accessed_time)
            : '-'}
        </span>
      ),
    },
    {
      accessorKey: 'expired_time',
      header: t('Expires At'),
      enableSorting: false,
      size: 150,
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {!row.original.expired_time || row.original.expired_time === -1
            ? t('Never expires')
            : formatTimestamp(row.original.expired_time)}
        </span>
      ),
    },
    {
      id: 'actions',
      header: t('Actions'),
      enableSorting: false,
      enableHiding: false,
      size: 80,
      cell: ({ row }) => (
        <TokenRowActions context={context} token={row.original} />
      ),
    },
  ]
}

/**
 * The row's own menu, absent rather than disabled when the caller may do nothing
 * to this key: a public key held by someone else is readable but not editable,
 * and a greyed-out menu would suggest a permission that is not there.
 */
function TokenRowActions(props: {
  context: TokenColumnContext
  token: OrganizationTokenRow
}) {
  const { t } = useTranslation()
  const flags = props.context.flagsFor(props.token)

  if (!flags.canEdit && !flags.canDelete) return null

  const isEnabled = props.token.status === ORGANIZATION_TOKEN_STATUS.ENABLED

  return (
    <DataTableRowActionMenu ariaLabel={t('Open menu')} contentClassName='w-44'>
      <DropdownMenuItem
        onSelect={() =>
          props.context.onChangeStatus(
            props.token,
            isEnabled
              ? ORGANIZATION_TOKEN_STATUS.DISABLED
              : ORGANIZATION_TOKEN_STATUS.ENABLED
          )
        }
        disabled={!flags.canEdit}
      >
        {isEnabled ? t('Disable') : t('Enable')}
      </DropdownMenuItem>

      <DropdownMenuItem
        onSelect={() => props.context.onEdit(props.token)}
        disabled={!flags.canEdit}
      >
        {t('Edit')}
      </DropdownMenuItem>

      <DropdownMenuSeparator />

      <DropdownMenuItem
        onSelect={() => props.context.onDelete(props.token)}
        className='text-destructive focus:text-destructive'
        disabled={!flags.canDelete}
      >
        {t('Delete')}
        <DropdownMenuShortcut>
          <Trash2 size={16} />
        </DropdownMenuShortcut>
      </DropdownMenuItem>
    </DataTableRowActionMenu>
  )
}
