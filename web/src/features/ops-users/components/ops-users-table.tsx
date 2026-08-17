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
import { useQuery } from '@tanstack/react-query'
import type {
  ColumnDef,
  OnChangeFn,
  PaginationState,
} from '@tanstack/react-table'
import type { TFunction } from 'i18next'
import { Loader2 } from 'lucide-react'
import { useTranslation } from 'react-i18next'

import { BadgeCell, DataTablePage, useDataTable } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { LongText } from '@/components/long-text'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { Button } from '@/components/ui/button'
import { Checkbox } from '@/components/ui/checkbox'
import { formatQuota, formatTimestamp } from '@/lib/format'
import { getRoleLabel } from '@/lib/roles'

import { USER_STATUSES } from '../../users/constants'
import { getOpsUserColumns } from '../api'
import type { OpsUser, OpsUserColumnMeta } from '../types'
import { OpsUsersColumnSelector } from './ops-users-column-selector'

const OPS_USERS_VISIBLE_COLUMNS_STORAGE_KEY = 'ops-users-visible-columns'

// Maps server column keys to their i18n label keys (see
// controller/ops_user_columns.go — the two lists are kept in sync).
const OPS_USER_COLUMN_LABEL_KEYS: Record<string, string> = {
  id: 'ID',
  username: 'Username',
  display_name: 'Display Name',
  phone: 'Phone',
  role: 'Role',
  status: 'Status',
  group: 'Group',
  quota: 'Quota',
  used_quota: 'Used Quota',
  request_count: 'Requests',
  total_prompt_tokens: 'Prompt Tokens',
  total_completion_tokens: 'Completion Tokens',
  aff_code: 'Aff Code',
  aff_count: 'Invite Count',
  created_at: 'Created At',
}

// Fallback while /columns is loading (or if it fails); the server order and
// required flags always win once the metadata arrives.
const DEFAULT_OPS_USER_COLUMNS: OpsUserColumnMeta[] = [
  { key: 'id', label: 'ID', required: true },
  { key: 'username', label: 'Username', required: true },
  { key: 'display_name', label: 'Display Name', required: false },
  { key: 'phone', label: 'Phone', required: false },
  { key: 'role', label: 'Role', required: false },
  { key: 'status', label: 'Status', required: false },
  { key: 'group', label: 'Group', required: false },
  { key: 'quota', label: 'Quota', required: false },
  { key: 'used_quota', label: 'Used Quota', required: false },
  { key: 'request_count', label: 'Requests', required: false },
  { key: 'total_prompt_tokens', label: 'Prompt Tokens', required: false },
  {
    key: 'total_completion_tokens',
    label: 'Completion Tokens',
    required: false,
  },
  { key: 'aff_code', label: 'Aff Code', required: false },
  { key: 'aff_count', label: 'Invite Count', required: false },
  { key: 'created_at', label: 'Created At', required: false },
]

type OpsUsersTableProps = {
  data: OpsUser[]
  isLoading: boolean
  isFetching: boolean
  totalCount: number
  pagination: PaginationState
  onPaginationChange: OnChangeFn<PaginationState>
  globalFilter: string
  onGlobalFilterChange: OnChangeFn<string>
  isExporting: boolean
  onExport: (selectedIds: number[]) => void
}

export function OpsUsersTable({
  data,
  isLoading,
  isFetching,
  totalCount,
  pagination,
  onPaginationChange,
  globalFilter,
  onGlobalFilterChange,
  isExporting,
  onExport,
}: OpsUsersTableProps) {
  const { t } = useTranslation()

  const { data: columnMetaData } = useQuery({
    queryKey: ['ops-users-columns'],
    queryFn: getOpsUserColumns,
    // Column metadata is hand-maintained server-side and only changes on
    // redeploy; no need to refetch on every mount.
    staleTime: Infinity,
  })

  const columnMeta = columnMetaData?.data
  const columns = buildOpsUserColumns(t, columnMeta ?? DEFAULT_OPS_USER_COLUMNS)

  const { table } = useDataTable({
    data,
    columns,
    enableRowSelection: true,
    // Selection must survive page changes: without a stable row id,
    // TanStack keys selected rows by their index on the current page.
    getRowId: (row) => String(row.id),
    globalFilter,
    onGlobalFilterChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount,
    columnVisibilityStorageKey: OPS_USERS_VISIBLE_COLUMNS_STORAGE_KEY,
  })

  const selectedIds = table
    .getSelectedRowModel()
    .rows.map((row) => row.original.id)

  return (
    <DataTablePage
      table={table}
      columns={columns}
      isLoading={isLoading}
      isFetching={isFetching}
      emptyTitle={t('No invitees found')}
      skeletonKeyPrefix='ops-users-skeleton'
      applyHeaderSize
      toolbarProps={{
        searchPlaceholder: t('Search invitees'),
        hideViewOptions: true,
        preActions: (
          <>
            <Button
              variant='outline'
              className='shrink-0'
              disabled={isExporting}
              onClick={() => onExport(selectedIds)}
            >
              {isExporting && <Loader2 className='animate-spin' />}
              {t('Export')}
            </Button>
            <OpsUsersColumnSelector
              table={table}
              columns={columnMeta ?? DEFAULT_OPS_USER_COLUMNS}
            />
          </>
        ),
      }}
    />
  )
}

function buildOpsUserColumns(
  t: TFunction,
  metaList: OpsUserColumnMeta[]
): ColumnDef<OpsUser>[] {
  return [
    selectColumnDef,
    ...metaList.map((meta) => buildOpsUserColumn(meta, t)),
  ]
}

const selectColumnDef: ColumnDef<OpsUser> = {
  id: 'select',
  header: ({ table }) => (
    <Checkbox
      checked={table.getIsAllPageRowsSelected()}
      indeterminate={table.getIsSomePageRowsSelected()}
      onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
      aria-label='Select all'
      className='translate-y-[2px]'
    />
  ),
  cell: ({ row }) => (
    <Checkbox
      checked={row.getIsSelected()}
      onCheckedChange={(value) => row.toggleSelected(!!value)}
      aria-label='Select row'
      className='translate-y-[2px]'
    />
  ),
  enableSorting: false,
  enableHiding: false,
  size: 40,
}

function buildOpsUserColumn(
  meta: OpsUserColumnMeta,
  t: TFunction
): ColumnDef<OpsUser> {
  const columnId = meta.key
  const header = OPS_USER_COLUMN_LABEL_KEYS[columnId]
    ? t(OPS_USER_COLUMN_LABEL_KEYS[columnId])
    : meta.label

  switch (columnId) {
    case 'id':
      return {
        accessorKey: 'id',
        header,
        enableHiding: !meta.required,
        meta: { mobileHidden: true },
        cell: ({ row }) => (
          <TableId value={row.getValue('id') as number} className='w-[60px]' />
        ),
        size: 80,
      }
    case 'username':
      return {
        accessorKey: 'username',
        header,
        enableHiding: !meta.required,
        meta: { mobileTitle: true },
        cell: ({ row }) => (
          <LongText className='max-w-[160px] font-medium'>
            {row.getValue('username') as string}
          </LongText>
        ),
        size: 180,
      }
    case 'display_name':
      return {
        accessorKey: 'display_name',
        header,
        cell: ({ row }) => (
          <span className='text-sm'>
            {row.getValue('display_name') as string}
          </span>
        ),
        size: 160,
      }
    case 'phone':
      return {
        accessorKey: 'phone',
        header,
        cell: ({ row }) => {
          const phone = row.getValue('phone') as string | undefined
          return (
            <span className='font-mono text-sm'>
              {phone || '-'}
            </span>
          )
        },
        size: 140,
      }
    case 'role':
      return {
        accessorKey: 'role',
        header,
        cell: ({ row }) => (
          <span className='text-sm'>
            {getRoleLabel(row.getValue('role') as number)}
          </span>
        ),
        size: 110,
      }
    case 'status':
      return {
        accessorKey: 'status',
        header,
        meta: { mobileBadge: true },
        cell: ({ row }) => {
          const statusConfig =
            USER_STATUSES[row.getValue('status') as keyof typeof USER_STATUSES]
          if (!statusConfig) {
            return null
          }
          return (
            <StatusBadge
              label={t(statusConfig.labelKey)}
              variant={statusConfig.variant}
              copyable={false}
              className='-ml-1.5'
            />
          )
        },
        size: 110,
      }
    case 'group':
      return {
        accessorKey: 'group',
        header,
        cell: ({ row }) => (
          <BadgeCell>
            <GroupBadge group={row.getValue('group') as string} />
          </BadgeCell>
        ),
        size: 140,
      }
    case 'quota':
    case 'used_quota':
      return {
        accessorKey: columnId,
        header,
        cell: ({ row }) => (
          <span className='font-mono text-sm'>
            {formatQuota(row.getValue(columnId) as number)}
          </span>
        ),
        size: 120,
      }
    case 'request_count':
    case 'total_prompt_tokens':
    case 'total_completion_tokens':
      return {
        accessorKey: columnId,
        header,
        cell: ({ row }) => (
          <span className='font-mono text-sm'>
            {(row.getValue(columnId) as number).toLocaleString()}
          </span>
        ),
        size: 130,
      }
    case 'aff_code':
      return {
        accessorKey: 'aff_code',
        header,
        cell: ({ row }) => (
          <span className='font-mono text-sm'>
            {row.getValue('aff_code') as string}
          </span>
        ),
        size: 140,
      }
    case 'aff_count':
      return {
        accessorKey: 'aff_count',
        header,
        cell: ({ row }) => (
          <span className='text-sm'>
            {(row.getValue('aff_count') as number).toLocaleString()}
          </span>
        ),
        size: 110,
      }
    case 'created_at':
      return {
        accessorKey: 'created_at',
        header,
        meta: { mobileHidden: true },
        cell: ({ row }) => {
          const ts = row.getValue('created_at') as number | undefined
          return (
            <span className='text-muted-foreground text-sm'>
              {ts ? formatTimestamp(ts) : '-'}
            </span>
          )
        },
        size: 180,
      }
    default:
      // Unknown server columns render as plain text with the
      // server-provided label; the backend keeps this list in sync.
      return {
        accessorKey: columnId,
        header,
        cell: ({ row }) => {
          const value = row.getValue(columnId)
          return <span className='text-sm'>{String(value ?? '')}</span>
        },
      }
  }
}
