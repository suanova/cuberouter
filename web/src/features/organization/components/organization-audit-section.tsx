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
import { useTranslation } from 'react-i18next'

import { DataTablePage, useDataTable } from '@/components/data-table'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { useTableUrlState } from '@/hooks/use-table-url-state'
import { formatTimestamp } from '@/lib/format'

import { listOrganizationAuditLogs } from '../api'
import {
  buildOrganizationAuditTargetView,
  formatOrganizationAuditReason,
  organizationAuditActionLabelKey,
  organizationAuditActionOptions,
  organizationAuditActionTone,
  organizationAuditTargetTone,
  organizationAuditTargetTypeLabelKey,
  organizationAuditTargetTypeOptions,
  type OrganizationAuditTargetView,
  type Translate,
} from '../lib'
import { ORGANIZATION_DEFAULT_PAGE_SIZE } from '../lib/organization-pagination'
import { useOrganizationPagedSection } from '../hooks/use-organization-paged-query'
import type { OrganizationAuditLogRow } from '../types'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
} from './organization-section'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

const AUDIT_COLUMN_VISIBILITY_STORAGE_KEY = 'organization-audit-column-visibility'

/** The backend filters on one value per field, so a multi-select sends its first. */
function firstFilterValue(
  columnFilters: Array<{ id: string; value: unknown }>,
  columnId: string
): string | undefined {
  const value = columnFilters.find((filter) => filter.id === columnId)?.value
  if (Array.isArray(value)) return value[0] as string | undefined
  return typeof value === 'string' && value ? value : undefined
}

type OrganizationAuditSectionProps = {
  organizationId: number
  /** The caller may read the organization's audit trail. */
  canViewAudit: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * The organization's audit trail: who did what, to which object, and why.
 *
 * Read-only by construction — the backend writes these records, and there is no
 * endpoint to amend them.
 */
export function OrganizationAuditSection(props: OrganizationAuditSectionProps) {
  const { t } = useTranslation()
  // The translate helpers are plain functions so they can be unit tested; this
  // adapts the component's `t` to that narrower signature.
  const translate: Translate = (key, options) => t(key, options)
  const {
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    ensurePageInRange,
  } = useTableUrlState({
    search: route.useSearch(),
    navigate: route.useNavigate(),
    pagination: { defaultPage: 1, defaultPageSize: ORGANIZATION_DEFAULT_PAGE_SIZE },
    globalFilter: { enabled: false },
    columnFilters: [
      { columnId: 'action_type', searchKey: 'auditAction', type: 'array' },
      {
        columnId: 'target_type',
        searchKey: 'auditTargetType',
        type: 'array',
      },
    ],
  })

  const params = {
    p: pagination.pageIndex + 1,
    page_size: pagination.pageSize,
    action_type: firstFilterValue(columnFilters, 'action_type'),
    target_type: firstFilterValue(columnFilters, 'target_type'),
  }

  const audit = useOrganizationPagedSection<OrganizationAuditLogRow>({
    organizationId: props.organizationId,
    resource: 'audit-logs',
    params,
    query: () =>
      listOrganizationAuditLogs(props.organizationId, {
        p: params.p,
        page_size: params.page_size,
        action_type: params.action_type,
        target_type: params.target_type,
      }),
    enabled: props.canViewAudit,
    onForbidden: props.onForbidden,
  })

  const columns = buildAuditColumns(translate)

  const { table } = useDataTable({
    data: audit.items,
    columns,
    columnFilters,
    onColumnFiltersChange,
    pagination,
    onPaginationChange,
    manualPagination: true,
    manualFiltering: true,
    totalCount: audit.total,
    ensurePageInRange,
    columnVisibilityStorageKey: AUDIT_COLUMN_VISIBILITY_STORAGE_KEY,
  })

  if (!props.canViewAudit) {
    return (
      <OrganizationSectionEmpty
        icon='audit-logs'
        title={t('Audit Logs')}
        message={t(
          'Only an organization owner or administrator can read the audit trail.'
        )}
      />
    )
  }

  return (
    <OrganizationSection
      icon='audit-logs'
      title={t('Audit Logs')}
      description={t(
        'Every change made to this organization, its members, its API keys and its invitations.'
      )}
      count={audit.total}
    >
      <DataTablePage
        table={table}
        columns={columns}
        isLoading={audit.isLoading}
        isFetching={audit.isFetching}
        emptyTitle={t('No audit records yet')}
        skeletonKeyPrefix='organization-audit-skeleton'
        applyHeaderSize
        toolbarProps={{
          // The audit endpoint has no keyword search: it filters by action,
          // target, operator and time, all of which are exact matches.
          customSearch: null,
          hideViewOptions: true,
          filters: [
            {
              columnId: 'action_type',
              title: t('Action'),
              options: organizationAuditActionOptions(translate),
              singleSelect: true,
            },
            {
              columnId: 'target_type',
              title: t('Target Type'),
              options: organizationAuditTargetTypeOptions(translate),
              singleSelect: true,
            },
          ],
        }}
      />
    </OrganizationSection>
  )
}

function buildAuditColumns(translate: Translate): ColumnDef<OrganizationAuditLogRow>[] {
  const t = translate
  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      enableSorting: false,
      enableHiding: false,
      size: 90,
      cell: ({ row }) => <TableId value={row.original.id} className='w-[60px]' />,
    },
    {
      accessorKey: 'created_at',
      header: t('Time'),
      enableSorting: false,
      size: 180,
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm whitespace-nowrap'>
          {row.original.created_at ? formatTimestamp(row.original.created_at) : '-'}
        </span>
      ),
    },
    {
      accessorKey: 'action_type',
      header: t('Action'),
      enableSorting: false,
      size: 190,
      cell: ({ row }) => (
        <StatusBadge
          label={t(organizationAuditActionLabelKey(row.original.action_type))}
          variant={organizationAuditActionTone(row.original.action_type)}
          copyable={false}
        />
      ),
    },
    {
      accessorKey: 'operator_username',
      header: t('Operator'),
      enableSorting: false,
      size: 170,
      cell: ({ row }) => (
        <span className='font-medium whitespace-nowrap'>
          {row.original.operator_display_name ||
            row.original.operator_username ||
            '-'}
        </span>
      ),
    },
    {
      accessorKey: 'target_type',
      header: t('Target Type'),
      enableSorting: false,
      size: 150,
      cell: ({ row }) => (
        <StatusBadge
          label={t(
            organizationAuditTargetTypeLabelKey(row.original.target_type)
          )}
          variant={organizationAuditTargetTone(row.original.target_type)}
          copyable={false}
        />
      ),
    },
    {
      accessorKey: 'target_id',
      header: t('Target'),
      enableSorting: false,
      size: 280,
      cell: ({ row }) => (
        <AuditTargetCell target={buildOrganizationAuditTargetView(t, row.original)} />
      ),
    },
    {
      accessorKey: 'reason',
      header: t('Reason'),
      enableSorting: false,
      size: 180,
      cell: ({ row }) => {
        const reason = formatOrganizationAuditReason(t, row.original.reason)
        if (reason === '-') {
          return <span className='text-muted-foreground'>-</span>
        }
        return <span className='text-sm'>{reason}</span>
      },
    },
  ]
}

/**
 * The target is a name on screen and a record on hover.
 *
 * The audit trail exists to answer "what exactly changed", and the row's own
 * columns cannot carry that — the ids, the previous role, the masked key all
 * come from the row's snapshots, which are shown together in the tooltip rather
 * than squeezed into the cell.
 */
function AuditTargetCell(props: { target: OrganizationAuditTargetView }) {
  if (props.target.details.length === 0) {
    return <span className='text-sm'>{props.target.main}</span>
  }

  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='text-primary inline-flex max-w-full cursor-help truncate align-middle text-sm' />
          }
        >
          {props.target.main}
        </TooltipTrigger>
        <TooltipContent className='max-w-[360px]'>
          <div className='flex flex-col gap-1'>
            {props.target.details.map((line) => (
              <div key={line.label} className='flex min-w-0 gap-2'>
                <span className='shrink-0 opacity-70'>{line.label}</span>
                <span className={line.nowrap ? 'whitespace-nowrap' : 'break-all'}>
                  {line.value}
                </span>
              </div>
            ))}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}
