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

import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { formatTimestamp } from '@/lib/format'

import {
  buildOrganizationAuditTargetView,
  formatOrganizationAuditReason,
  organizationAuditActionLabelKey,
  organizationAuditActionTone,
  organizationAuditTargetTone,
  organizationAuditTargetTypeLabelKey,
  type OrganizationAuditTargetView,
  type Translate,
} from '../lib'
import type { OrganizationAuditLogRow } from '../types'

/**
 * The audit trail as a row, for every table that shows one.
 *
 * An organization's own page and the platform's cross-organization page show
 * the same records, so they read them the same way here: an action has to be
 * named and coloured identically wherever an administrator meets it, or the two
 * pages become two dialects of one trail. A caller that needs more prepends its
 * own column — the platform page names the organization, which is the one thing
 * a single organization's trail cannot vary.
 */
export function buildOrganizationAuditColumns(
  translate: Translate
): ColumnDef<OrganizationAuditLogRow>[] {
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
        renderAuditTarget(buildOrganizationAuditTargetView(t, row.original))
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
function renderAuditTarget(target: OrganizationAuditTargetView) {
  if (target.details.length === 0) {
    return <span className='text-sm'>{target.main}</span>
  }

  return (
    <TooltipProvider delay={150}>
      <Tooltip>
        <TooltipTrigger
          render={
            <span className='text-primary inline-flex max-w-full cursor-help truncate align-middle text-sm' />
          }
        >
          {target.main}
        </TooltipTrigger>
        <TooltipContent className='max-w-[360px]'>
          <div className='flex flex-col gap-1'>
            {target.details.map((line) => (
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
