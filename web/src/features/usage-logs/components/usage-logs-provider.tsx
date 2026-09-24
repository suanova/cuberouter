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
/* eslint-disable react-refresh/only-export-components */
import { createContext, useContext, useState, type ReactNode } from 'react'

import { ROLE } from '@/lib/roles'
import { useAuthStore } from '@/stores/auth-store'

import type { ChannelAffinityInfo, LogCategory } from '../types'

export type LogsViewScope = 'all' | 'self'
export type LogsViewAccess = 'self' | 'admin' | 'root'

/**
 * Resolves the effective log view tier. Ops and above may read the common
 * log endpoint (OpsAuth), but the drawing (/api/mj) and task (/api/task)
 * read endpoints are still AdminAuth — for those categories ops is pinned
 * to the self tier so "All" never issues a request it cannot authorize.
 */
export function resolveLogsViewAccess(
  role: number,
  viewScope: LogsViewScope,
  logCategory: LogCategory = 'common'
): LogsViewAccess {
  if (viewScope !== 'all' || role < ROLE.OPS) return 'self'
  if (logCategory !== 'common' && role < ROLE.ADMIN) return 'self'
  return role === ROLE.SUPER_ADMIN ? 'root' : 'admin'
}

interface UsageLogsContextValue {
  selectedUserId: number | null
  setSelectedUserId: (userId: number | null) => void
  userInfoDialogOpen: boolean
  setUserInfoDialogOpen: (open: boolean) => void
  affinityTarget: ChannelAffinityInfo | null
  setAffinityTarget: (target: ChannelAffinityInfo | null) => void
  affinityDialogOpen: boolean
  setAffinityDialogOpen: (open: boolean) => void
  sensitiveVisible: boolean
  setSensitiveVisible: (visible: boolean) => void
  viewScope: LogsViewScope
  setViewScope: (scope: LogsViewScope) => void
}

const UsageLogsContext = createContext<UsageLogsContextValue | undefined>(
  undefined
)

export function UsageLogsProvider({ children }: { children: ReactNode }) {
  const [selectedUserId, setSelectedUserId] = useState<number | null>(null)
  const [userInfoDialogOpen, setUserInfoDialogOpen] = useState(false)
  const [affinityTarget, setAffinityTarget] =
    useState<ChannelAffinityInfo | null>(null)
  const [affinityDialogOpen, setAffinityDialogOpen] = useState(false)
  const [sensitiveVisible, setSensitiveVisible] = useState(true)
  const [viewScope, setViewScope] = useState<LogsViewScope>('all')

  return (
    <UsageLogsContext.Provider
      value={{
        selectedUserId,
        setSelectedUserId,
        userInfoDialogOpen,
        setUserInfoDialogOpen,
        affinityTarget,
        setAffinityTarget,
        affinityDialogOpen,
        setAffinityDialogOpen,
        sensitiveVisible,
        setSensitiveVisible,
        viewScope,
        setViewScope,
      }}
    >
      {children}
    </UsageLogsContext.Provider>
  )
}

export function useUsageLogsContext() {
  const context = useContext(UsageLogsContext)
  if (!context) {
    throw new Error('useUsageLogsContext must be used within UsageLogsProvider')
  }
  return context
}

/**
 * Resolves the effective admin scope for usage logs: whether the current
 * user is allowed to view all users' logs (`canManageScope`), and whether
 * their current view preference (`viewScope`) has that scope active
 * (`isAdminView`) for the given `logCategory`. Data fetching and admin-only
 * UI should key off `isAdminView` rather than raw role, so an admin who
 * switches to "only mine" is treated exactly like a regular user for that
 * view, and ops are pinned to self on drawing/task categories.
 */
export function useLogsViewScope(logCategory: LogCategory = 'common') {
  const role = useAuthStore((state) => state.auth.user?.role ?? ROLE.GUEST)
  const { viewScope, setViewScope } = useUsageLogsContext()
  // Whether the "All" tier is reachable for this category: ops and above
  // for common logs (read-only; the log table has no write actions), admin
  // and above for drawing/task (their read endpoints are AdminAuth).
  const canManageScope =
    resolveLogsViewAccess(role, 'all', logCategory) !== 'self'
  const viewAccess = resolveLogsViewAccess(role, viewScope, logCategory)
  const isAdminView = viewAccess !== 'self'
  const isRootView = viewAccess === 'root'

  return {
    canManageScope,
    viewScope,
    setViewScope,
    isAdminView,
    isRootView,
    viewAccess,
  }
}
