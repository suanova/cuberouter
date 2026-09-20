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
import React, { type ReactNode } from 'react'

import type { NavigateFn } from '@/hooks/use-table-url-state'

import type { OrganizationDetailSearch } from '../lib'
import {
  DEFAULT_ORGANIZATION_SURFACE,
  type OrganizationSurface,
} from '../lib/organization-surface'

type OrganizationPageContextValue = {
  /** Which API surface this page reads its organization through. */
  surface: OrganizationSurface
  /** The page's search params, whichever route owns them. */
  search: OrganizationDetailSearch
  /** Replaces the search params of the section currently rendered. */
  navigate: NavigateFn
}

const OrganizationPageContext =
  React.createContext<OrganizationPageContextValue | null>(null)

/**
 * Tells everything below it which organization page it is on.
 *
 * One set of sections renders an organization — the overview, the members, the
 * API keys, the logs, the tasks, the usage and the audit trail. The member
 * center and the platform admin console both show those same sections, and
 * differ in exactly two ways: the API prefix they read from, and the route that
 * owns their search params. Both are supplied here rather than passed down
 * through every section, because a section that had to name its own route could
 * not be rendered from anywhere else.
 *
 * `search` and `navigate` are handed in already resolved because only the route
 * module knows how to resolve them; see the two detail routes.
 */
export function OrganizationPageProvider(props: {
  surface: OrganizationSurface
  search: OrganizationDetailSearch
  navigate: NavigateFn
  children: ReactNode
}) {
  const value: OrganizationPageContextValue = {
    surface: props.surface,
    search: props.search,
    navigate: props.navigate,
  }

  return (
    <OrganizationPageContext value={value}>
      {props.children}
    </OrganizationPageContext>
  )
}

function useOrganizationPageContext(): OrganizationPageContextValue {
  const context = React.useContext(OrganizationPageContext)
  if (!context) {
    throw new Error(
      'The organization detail page hooks have to be used within <OrganizationPageProvider>'
    )
  }
  return context
}

/**
 * The API surface to read this organization through.
 *
 * Every call into `../api` that names an organization takes this as its first
 * argument, so which organization the caller is looking at and how they are
 * allowed to reach it stay two separate decisions.
 *
 * Outside a detail page there is nothing to read, and the answer is the member
 * surface: the organization center's own list and its dialogs are member-side,
 * and they render on a route that has no organization in it. The admin console
 * says which surface it is on explicitly.
 */
// eslint-disable-next-line react-refresh/only-export-components
export function useOrganizationSurface(): OrganizationSurface {
  const context = React.useContext(OrganizationPageContext)
  return context?.surface ?? DEFAULT_ORGANIZATION_SURFACE
}

/** The search params and the way to replace them, for URL-backed tables. */
// eslint-disable-next-line react-refresh/only-export-components
export function useOrganizationSectionRoute(): {
  search: OrganizationDetailSearch
  navigate: NavigateFn
} {
  const { search, navigate } = useOrganizationPageContext()
  return { search, navigate }
}
