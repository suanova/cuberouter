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
import {
  Activity,
  Box,
  Building2,
  ChartColumn,
  CreditCard,
  FileText,
  FlaskConical,
  Key,
  LayoutDashboard,
  ListTodo,
  Megaphone,
  MessageSquare,
  PlugZap,
  Puzzle,
  Radio,
  ScrollText,
  ServerCog,
  Settings,
  Ticket,
  User,
  Users,
  Wallet,
} from 'lucide-react'
import { useTranslation } from 'react-i18next'

import type { NavItem, SidebarData } from '@/components/layout/types'
import { getOrganizations } from '@/features/organization/api'
import { ROLE } from '@/lib/roles'
import { getAccountContextCacheKey } from '@/stores/account-context-store'

/**
 * Root navigation groups for the application sidebar.
 *
 * These are shown when the URL does not match any nested sidebar view
 * registered in `layout/lib/sidebar-view-registry.ts`.
 */
export function useSidebarData(): SidebarData {
  const { t } = useTranslation()
  // The organization center is a management surface. A plain member manages
  // their organization keys from the personal API Keys page and has no use for
  // it, so the entry is offered only when at least one organization is theirs
  // to manage. Kept hidden until the answer is known, so a member never sees it
  // appear and disappear.
  const { data: organizations = [], isFetched: organizationsLoaded } = useQuery(
    {
      queryKey: ['organizations', 'sidebar', getAccountContextCacheKey()],
      queryFn: async () => {
        const result = await getOrganizations()
        return result.success ? (result.data ?? []) : []
      },
      staleTime: 5 * 60 * 1000,
    }
  )
  const showOrganizations =
    organizationsLoaded &&
    organizations.some(
      (organization) =>
        organization.role === 'owner' || organization.role === 'admin'
    )

  return {
    navGroups: [
      {
        id: 'chat',
        title: t('Chat'),
        items: [
          {
            title: t('Playground'),
            url: '/playground',
            icon: FlaskConical,
          },
          {
            title: t('Chat'),
            icon: MessageSquare,
            type: 'chat-presets',
          },
        ],
      },
      {
        id: 'general',
        title: t('General'),
        items: [
          {
            title: t('Overview'),
            url: '/dashboard/overview',
            icon: Activity,
            requiredRole: ROLE.ADMIN,
          },
          {
            title: t('Dashboard'),
            url: '/dashboard/models',
            icon: LayoutDashboard,
            requiredRole: ROLE.ADMIN,
          },
          {
            title: t('API Keys'),
            url: '/keys',
            icon: Key,
          },
          ...(showOrganizations
            ? [
                {
                  title: t('Organizations'),
                  url: '/organizations',
                  icon: Building2,
                } satisfies NavItem,
              ]
            : []),
          {
            title: t('Usage'),
            url: '/usage',
            icon: ChartColumn,
          },
          {
            title: t('Usage Logs'),
            url: '/usage-logs/common',
            icon: FileText,
          },
          {
            title: t('Task Logs'),
            url: '/usage-logs/task',
            activeUrls: ['/usage-logs/drawing'],
            configUrls: ['/usage-logs/drawing', '/usage-logs/task'],
            icon: ListTodo,
          },
        ],
      },
      {
        id: 'personal',
        title: t('Personal'),
        items: [
          {
            title: t('Wallet'),
            url: '/wallet',
            icon: Wallet,
          },
          {
            title: t('Profile'),
            url: '/profile',
            icon: User,
          },
        ],
      },
      {
        id: 'admin',
        title: t('Admin'),
        items: [
          {
            title: t('Channels'),
            url: '/channels',
            icon: Radio,
          },
          {
            title: t('Model Metadata'),
            url: '/models/metadata',
            icon: Box,
          },
          {
            title: t('Plugins'),
            url: '/plugins',
            icon: Puzzle,
          },
          {
            title: t('Users'),
            url: '/users',
            icon: Users,
          },
          {
            // Named for what it does rather than for what it lists: the
            // personal organization center is already called "Organizations",
            // and the two are different pages answering different questions.
            title: t('Organization Management'),
            url: '/admin/organizations',
            icon: Building2,
          },
          {
            // What the organizations did, rather than which ones exist, so it
            // is its own entry: an administrator looking for one member's
            // history is not looking at the list.
            title: t('Organization Audit Log'),
            url: '/admin/organization-audit-logs',
            icon: ScrollText,
          },
          {
            title: t('Redemption Codes'),
            url: '/redemption-codes',
            icon: Ticket,
          },
          {
            title: t('Campaigns'),
            url: '/campaigns',
            icon: Megaphone,
          },
          {
            title: t('Subscriptions'),
            url: '/subscriptions',
            icon: CreditCard,
          },
          {
            title: t('System Info'),
            url: '/system-info',
            icon: ServerCog,
            requiredRole: ROLE.SUPER_ADMIN,
          },
          {
            title: t('Task Plugins'),
            url: '/task-plugins',
            icon: PlugZap,
            requiredRole: ROLE.SUPER_ADMIN,
          },
          {
            title: t('System Settings'),
            url: '/system-settings/site',
            activeUrls: ['/system-settings'],
            icon: Settings,
          },
        ],
      },
      {
        id: 'ops',
        title: t('Ops'),
        items: [
          {
            title: t('Ops Campaigns'),
            url: '/ops/campaign',
            icon: Megaphone,
            requiredRole: ROLE.OPS,
          },
          {
            title: t('Users'),
            url: '/users',
            icon: Users,
            requiredRole: ROLE.OPS,
          },
          {
            title: t('Invite History'),
            url: '/ops/invite-history',
            icon: Users,
            requiredRole: ROLE.OPS,
          },
        ],
      },
    ],
  }
}
