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
import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'

import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs'

import {
  normalizeOrganizationTaskPanel,
  ORGANIZATION_TASK_PANELS,
  type OrganizationTaskPanel,
} from '../lib'
import { OrganizationMidjourneyTaskList } from './organization-midjourney-task-list'
import {
  OrganizationSection,
  OrganizationSectionEmpty,
} from './organization-section'
import { OrganizationTaskList } from './organization-task-list'

const route = getRouteApi(
  '/_authenticated/organizations/$organizationId/$section'
)

/**
 * The two task tables.
 *
 * An async task and a Midjourney task are separate records with separate
 * endpoints, separate filters and — awkwardly — separate units for the same
 * timestamp, so they are panels rather than one table with a type column.
 */
const PANEL_LABELS: Record<OrganizationTaskPanel, string> = {
  tasks: 'Async tasks',
  midjourney: 'Midjourney',
}

type OrganizationTasksSectionProps = {
  organizationId: number
  /** The caller may read the organization's task records. */
  canView: boolean
  /**
   * The caller reads every member's tasks, not just their own.
   *
   * Without it the backend pins both lists to the caller, so the holder column
   * would repeat one name down the table.
   */
  canViewWideData: boolean
  /** Leaves the page when the backend refuses the read. */
  onForbidden: () => void
}

/**
 * What the organization's keys submitted, and what it cost.
 *
 * The open panel lives in the URL for the same reason the usage tab's does: the
 * panels hold their filters in the URL, and a link that names a filter without
 * naming the panel it belongs to would open a view where nothing appears to be
 * filtered.
 */
export function OrganizationTasksSection(props: OrganizationTasksSectionProps) {
  const { t } = useTranslation()
  const search = route.useSearch()
  const navigate = route.useNavigate()
  const panel = normalizeOrganizationTaskPanel(search.taskTab)

  if (!props.canView) {
    return (
      <OrganizationSectionEmpty
        icon='tasks'
        title={t('Tasks')}
        message={t(
          'Only an organization owner or administrator can read its task records.'
        )}
      />
    )
  }

  const panels = {
    tasks: (
      <OrganizationTaskList
        organizationId={props.organizationId}
        canViewWideData={props.canViewWideData}
        onForbidden={props.onForbidden}
      />
    ),
    midjourney: (
      <OrganizationMidjourneyTaskList
        organizationId={props.organizationId}
        canViewWideData={props.canViewWideData}
        onForbidden={props.onForbidden}
      />
    ),
  } satisfies Record<OrganizationTaskPanel, ReactNode>

  return (
    <OrganizationSection
      icon='tasks'
      title={t('Tasks')}
      description={t(
        'What the organization submitted through its keys, and what each task cost.'
      )}
      actions={
        <Tabs
          value={panel}
          onValueChange={(value) =>
            void navigate({
              search: (previous) => ({
                ...previous,
                taskTab: normalizeOrganizationTaskPanel(value),
              }),
            })
          }
        >
          <TabsList>
            {ORGANIZATION_TASK_PANELS.map((key) => (
              <TabsTrigger key={key} value={key}>
                {t(PANEL_LABELS[key])}
              </TabsTrigger>
            ))}
          </TabsList>
        </Tabs>
      }
      scroll
    >
      {panels[panel]}
    </OrganizationSection>
  )
}
