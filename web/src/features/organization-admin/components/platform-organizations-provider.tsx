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
import React, { useState } from 'react'

import useDialogState from '@/hooks/use-dialog'

import type { OrganizationManagementView } from '@/features/organization/types'

/** The platform list's own dialogs. There is no create: an administrator does
 * not open an organization on another user's behalf — that is what the member
 * surface is for. */
export type PlatformOrganizationDialogType = 'edit' | 'status' | 'dissolve'

type PlatformOrganizationsContextType = {
  open: PlatformOrganizationDialogType | null
  setOpen: (value: PlatformOrganizationDialogType | null) => void
  currentRow: OrganizationManagementView | null
  setCurrentRow: React.Dispatch<
    React.SetStateAction<OrganizationManagementView | null>
  >
  refreshTrigger: number
  triggerRefresh: () => void
}

const PlatformOrganizationsContext =
  React.createContext<PlatformOrganizationsContextType | null>(null)

export function PlatformOrganizationsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<PlatformOrganizationDialogType>(null)
  const [currentRow, setCurrentRow] =
    useState<OrganizationManagementView | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((previous) => previous + 1)

  return (
    <PlatformOrganizationsContext
      value={{
        open,
        setOpen,
        currentRow,
        setCurrentRow,
        refreshTrigger,
        triggerRefresh,
      }}
    >
      {children}
    </PlatformOrganizationsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export function usePlatformOrganizations() {
  const context = React.useContext(PlatformOrganizationsContext)

  if (!context) {
    throw new Error(
      'usePlatformOrganizations has to be used within <PlatformOrganizationsProvider>'
    )
  }

  return context
}
