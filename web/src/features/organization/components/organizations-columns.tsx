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
import { useTranslation } from 'react-i18next'

import { BadgeCell } from '@/components/data-table'
import { GroupBadge } from '@/components/group-badge'
import { StatusBadge } from '@/components/status-badge'
import { TableId } from '@/components/table-id'
import { formatQuota, formatTimestamp } from '@/lib/format'

import {
  ORGANIZATION_ROLES,
  ORGANIZATION_STATUSES,
  organizationRoleLabelKey,
} from '../constants'
import { useOpenOrganization } from '../hooks/use-open-organization'
import { getOrganizationListActionFlags } from '../lib'
import type { UserOrganization } from '../types'
import { OrganizationsRowActions } from './organizations-row-actions'

export function useOrganizationsColumns(): ColumnDef<UserOrganization>[] {
  const { t } = useTranslation()
  const openOrganization = useOpenOrganization()

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
      header: t('Name'),
      cell: ({ row }) => {
        const organization = row.original
        const description = organization.description?.trim()
        const canOpen = getOrganizationListActionFlags(organization).enter
        return (
          <div className='flex min-w-0 flex-col gap-0.5'>
            {canOpen ? (
              // The name is the way into the organization; the row menu offers
              // the same, and both go through the same context switch.
              <button
                type='button'
                className='truncate text-left font-medium hover:underline focus-visible:underline focus-visible:outline-none'
                onClick={() => void openOrganization(organization)}
              >
                {organization.name}
              </button>
            ) : (
              <span className='truncate font-medium'>{organization.name}</span>
            )}
            {description && (
              <span className='text-muted-foreground truncate text-xs'>
                {description}
              </span>
            )}
          </div>
        )
      },
      enableHiding: false,
      size: 260,
      meta: { mobileTitle: true },
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
      accessorKey: 'role',
      header: t('My Role'),
      cell: ({ row }) => {
        const config =
          ORGANIZATION_ROLES[
            row.original.role as keyof typeof ORGANIZATION_ROLES
          ]
        return (
          <StatusBadge
            label={t(organizationRoleLabelKey(row.original.role))}
            variant={config?.variant}
            copyable={false}
          />
        )
      },
      filterFn: (row, id, value: string[]) =>
        value.includes(String(row.getValue(id))),
      enableSorting: false,
      size: 130,
      meta: { mobileOrder: 20 },
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
      id: 'quota',
      header: t('Quota'),
      cell: ({ row }) => (
        <div className='flex min-w-0 flex-col gap-0.5 text-sm'>
          <span>{formatQuota(row.original.quota)}</span>
          <span className='text-muted-foreground text-xs'>
            {t('Used:')} {formatQuota(row.original.used_quota)}
          </span>
        </div>
      ),
      enableSorting: false,
      size: 200,
      meta: { mobileOrder: 40 },
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
        <OrganizationsRowActions organization={row.original} />
      ),
      enableSorting: false,
      enableHiding: false,
      size: 90,
      meta: { mobileBadge: true, mobileOrder: 5 },
    },
  ]
}
