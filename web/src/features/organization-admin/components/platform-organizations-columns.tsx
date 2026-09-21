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
import { Link } from '@tanstack/react-router'
import type { ColumnDef } from '@tanstack/react-table'
import { useTranslation } from 'react-i18next'

import { BadgeCell } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'
import { ORGANIZATION_STATUSES } from '@/features/organization/constants'
import type { OrganizationManagementView } from '@/features/organization/types'
import { formatNumber, formatQuota, formatTimestamp } from '@/lib/format'

import { PLATFORM_ORGANIZATION_DEFAULT_TAB } from '../lib'
import { PlatformOrganizationsRowActions } from './platform-organizations-row-actions'

/**
 * The platform list, as an administrator reads it.
 *
 * It carries the columns the organization center's own list does not: who owns
 * the organization and how much of its membership and key count is switched
 * off. Those are the questions a platform administrator opens this page with —
 * the organization's own members already know who they are.
 */
export function usePlatformOrganizationsColumns(options: {
  isRoot: boolean
}): ColumnDef<OrganizationManagementView>[] {
  const { t } = useTranslation()

  return [
    {
      accessorKey: 'id',
      header: t('ID'),
      cell: ({ row }) => (
        <TableId value={row.original.id} className='w-[60px] text-sm' />
      ),
      size: 80,
      meta: { mobileOrder: 10 },
    },
    {
      accessorKey: 'name',
      header: t('Organization'),
      cell: ({ row }) => (
        <div className='flex min-w-0 flex-col gap-0.5'>
          {/* The name is the way into the organization, the same as the row
              menu's View. Nothing is switched here: an administrator reads an
              organization through their own identity, never as a member. */}
          <Link
            to='/admin/organizations/$organizationId/$section'
            params={{
              organizationId: String(row.original.id),
              section: PLATFORM_ORGANIZATION_DEFAULT_TAB,
            }}
            className='truncate font-medium hover:underline focus-visible:underline focus-visible:outline-none'
          >
            {row.original.name}
          </Link>
          <span className='text-muted-foreground truncate text-xs'>
            {row.original.slug}
          </span>
        </div>
      ),
      enableHiding: false,
      size: 240,
      meta: { mobileTitle: true },
    },
    {
      accessorKey: 'description',
      header: t('Description'),
      cell: ({ row }) => {
        const description = row.original.description?.trim()
        if (!description) {
          return <span className='text-muted-foreground text-sm'>-</span>
        }
        return (
          <Tooltip>
            <TooltipTrigger
              render={
                <span className='block max-w-[220px] cursor-help truncate text-sm' />
              }
            >
              {description}
            </TooltipTrigger>
            <TooltipContent className='max-w-sm whitespace-pre-wrap'>
              {description}
            </TooltipContent>
          </Tooltip>
        )
      },
      enableSorting: false,
      size: 240,
    },
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const config =
          ORGANIZATION_STATUSES[
            row.original.status as keyof typeof ORGANIZATION_STATUSES
          ]
        if (!config) return null

        return (
          <StatusBadge
            label={t(config.labelKey)}
            variant={config.variant}
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value: string[]) =>
        value.includes(String(row.getValue(id))),
      enableSorting: false,
      size: 120,
      meta: { mobileBadge: true },
    },
    {
      id: 'quota',
      header: t('Quota'),
      cell: ({ row }) => {
        const organization = row.original
        const remaining = Math.max(
          organization.quota - organization.used_quota,
          0
        )
        return (
          <div className='flex min-w-0 flex-col gap-0.5 text-sm'>
            <span className='whitespace-nowrap'>
              {formatQuota(organization.used_quota)} /{' '}
              {formatQuota(organization.quota)}
            </span>
            <span className='text-muted-foreground text-xs whitespace-nowrap'>
              {t('Remaining:')} {formatQuota(remaining)}
            </span>
          </div>
        )
      },
      enableSorting: false,
      size: 200,
      meta: { mobileOrder: 40 },
    },
    {
      accessorKey: 'group',
      header: t('Group'),
      cell: ({ row }) => (
        <BadgeCell>
          <GroupBadge group={row.original.group} />
        </BadgeCell>
      ),
      enableSorting: false,
      size: 140,
      meta: { mobileOrder: 30 },
    },
    {
      id: 'owner',
      header: t('Owner'),
      cell: ({ row }) => (
        <div className='flex min-w-0 flex-col gap-0.5'>
          <span className='truncate text-sm'>
            {row.original.owner_display_name ||
              row.original.owner_username ||
              '-'}
          </span>
          <span className='text-muted-foreground truncate text-xs'>
            {row.original.owner_email || '-'}
          </span>
        </div>
      ),
      enableSorting: false,
      size: 210,
    },
    {
      id: 'members',
      header: t('Members'),
      cell: ({ row }) => (
        <CountsCell
          active={row.original.active_member_count}
          disabled={row.original.disabled_member_count}
          total={row.original.total_member_count}
        />
      ),
      enableSorting: false,
      size: 170,
    },
    {
      id: 'tokens',
      header: t('API Keys'),
      cell: ({ row }) => (
        <CountsCell
          active={row.original.enabled_token_count}
          disabled={row.original.disabled_token_count}
          total={row.original.total_token_count}
        />
      ),
      enableSorting: false,
      size: 170,
    },
    {
      accessorKey: 'request_count',
      header: t('Requests'),
      cell: ({ row }) => (
        <span className='text-sm'>
          {formatNumber(row.original.request_count)}
        </span>
      ),
      enableSorting: false,
      size: 120,
    },
    {
      accessorKey: 'created_at',
      header: t('Created'),
      cell: ({ row }) => (
        <span className='text-muted-foreground text-sm'>
          {formatTimestamp(row.original.created_at)}
        </span>
      ),
      enableSorting: false,
      size: 180,
      meta: { mobileOrder: 50 },
    },
    {
      id: 'actions',
      cell: ({ row }) => (
        <PlatformOrganizationsRowActions
          organization={row.original}
          isRoot={options.isRoot}
        />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 90,
      meta: { mobileBadge: true, mobileOrder: 5 },
    },
  ]
}

/**
 * How much of a count is switched off, out of how much there is.
 *
 * One column per resource rather than a badge per number: an administrator
 * scanning the list is looking for the rows that stand out, and three numbers
 * side by side in a fixed order are read faster than three coloured tags whose
 * order has to be learned.
 */
// eslint-disable-next-line react-refresh/only-export-components
function CountsCell(props: {
  active: number
  disabled: number
  total: number
}) {
  const { t } = useTranslation()

  return (
    <div className='flex min-w-0 flex-col gap-0.5 text-sm'>
      <span className='whitespace-nowrap'>
        {props.active} / {props.disabled}
      </span>
      <span className='text-muted-foreground text-xs whitespace-nowrap'>
        {t('Total:')} {props.total}
      </span>
    </div>
  )
}
