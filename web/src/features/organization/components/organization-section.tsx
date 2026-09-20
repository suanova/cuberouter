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
import {
  BarChart3,
  LayoutDashboard,
  ListChecks,
  KeyRound,
  MailPlus,
  RefreshCw,
  ScrollText,
  Settings2,
  ShieldCheck,
  UsersRound,
  type LucideIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from '@/components/ui/empty'
import { cn } from '@/lib/utils'

import type { OrganizationDetailTabKey } from '../constants'

/** One icon per section, so the header looks the same wherever it appears. */
const ORGANIZATION_SECTION_ICONS: Record<
  OrganizationDetailTabKey,
  LucideIcon
> = {
  overview: LayoutDashboard,
  members: UsersRound,
  invitations: MailPlus,
  tokens: KeyRound,
  logs: ScrollText,
  usage: BarChart3,
  tasks: ListChecks,
  'audit-logs': ShieldCheck,
  settings: Settings2,
}

type OrganizationSectionProps = {
  icon: OrganizationDetailTabKey
  title: string
  /** One line explaining what the section holds; omitted when obvious. */
  description?: string
  /** Row count, shown beside the title once it is known. */
  count?: number
  /** Buttons rendered at the end of the header row. */
  actions?: ReactNode
  /**
   * Let the body scroll on its own. Tables manage their own scrolling, so this
   * is only for the sections that render a plain form or a chart stack.
   */
  scroll?: boolean
  children: ReactNode
}

/**
 * The frame every organization section sits in: an icon, the section name and
 * whatever actions the caller is allowed to take.
 *
 * The header is deliberately outside the body — a table keeps its own toolbar
 * and pagination, and those belong to the table, not to the section.
 */
export function OrganizationSection(props: OrganizationSectionProps) {
  const Icon = ORGANIZATION_SECTION_ICONS[props.icon]

  return (
    <div className='flex h-full min-h-0 flex-col gap-3'>
      <div className='flex shrink-0 flex-wrap items-start justify-between gap-3'>
        <div className='flex min-w-0 items-start gap-3'>
          <div className='bg-primary/10 text-primary flex h-10 w-10 shrink-0 items-center justify-center rounded-xl'>
            <Icon className='h-5 w-5' />
          </div>
          <div className='min-w-0'>
            <div className='flex flex-wrap items-center gap-2'>
              <span className='font-medium'>{props.title}</span>
              {typeof props.count === 'number' && (
                <Badge variant='secondary'>{props.count}</Badge>
              )}
            </div>
            {props.description && (
              <p className='text-muted-foreground mt-0.5 max-w-2xl text-sm'>
                {props.description}
              </p>
            )}
          </div>
        </div>
        {props.actions && (
          <div className='flex flex-wrap items-center gap-2'>
            {props.actions}
          </div>
        )}
      </div>

      <div
        className={cn(
          'min-h-0 flex-1',
          props.scroll && 'overflow-auto'
        )}
      >
        {props.children}
      </div>
    </div>
  )
}

/**
 * Shown in place of a section the caller cannot read.
 *
 * The tab is only rendered at all when the backend's capability set allows it,
 * so reaching this means the capability was lost while the page was open — the
 * wording therefore explains the requirement rather than the failure.
 */
export function OrganizationSectionEmpty(props: {
  icon: OrganizationDetailTabKey
  title: string
  message: string
}) {
  const Icon = ORGANIZATION_SECTION_ICONS[props.icon]

  return (
    <Empty className='border'>
      <EmptyHeader>
        <EmptyMedia variant='icon'>
          <Icon />
        </EmptyMedia>
        <EmptyTitle>{props.title}</EmptyTitle>
        <EmptyDescription>{props.message}</EmptyDescription>
      </EmptyHeader>
    </Empty>
  )
}

/** Reloads just this section, leaving the rest of the page alone. */
export function OrganizationSectionRefresh(props: {
  onClick: () => void
  isFetching?: boolean
}) {
  const { t } = useTranslation()

  return (
    <Button
      variant='outline'
      size='sm'
      onClick={props.onClick}
      disabled={props.isFetching}
    >
      <RefreshCw className={cn('h-4 w-4', props.isFetching && 'animate-spin')} />
      {t('Refresh')}
    </Button>
  )
}
