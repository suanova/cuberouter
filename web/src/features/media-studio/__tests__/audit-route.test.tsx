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
  createMemoryHistory,
  createRootRoute,
  createRoute,
  createRouter,
  Outlet,
  RouterProvider,
} from '@tanstack/react-router'
import { render, screen, waitFor } from '@testing-library/react'
import { afterEach, expect, test, vi } from 'vitest'

import { ROLE } from '@/lib/roles'
import { Route as AuditRoute } from '@/routes/_authenticated/admin/image-output-audit'
import { useAuthStore } from '@/stores/auth-store'

afterEach(() => useAuthStore.getState().auth.reset())

test('navigating to the audit route without root access redirects to 403', async () => {
  vi.spyOn(window, 'scrollTo').mockImplementation(() => undefined)
  useAuthStore
    .getState()
    .auth.setUser({ id: 1, username: 'admin', role: ROLE.ADMIN })
  const root = createRootRoute({ component: Outlet })
  // Initialize the real file route with the metadata normally supplied by the generated tree.
  const audit = AuditRoute.update({
    path: '/admin/image-output-audit',
    getParentRoute: () => root,
  } as never)
  const forbidden = createRoute({
    getParentRoute: () => root,
    path: '/403',
    component: () => <p>Access denied</p>,
  })
  const router = createRouter({
    routeTree: root.addChildren([audit, forbidden]),
    history: createMemoryHistory({
      initialEntries: ['/admin/image-output-audit'],
    }),
  })
  render(<RouterProvider router={router} />)
  await screen.findByText('Access denied')
  await waitFor(() => expect(router.state.location.pathname).toBe('/403'))
  expect(screen.queryByText('Image output audit')).not.toBeInTheDocument()
})
