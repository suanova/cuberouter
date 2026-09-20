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

import type { OrganizationDialogType, UserOrganization } from '../types'

type OrganizationsContextType = {
  open: OrganizationDialogType | null
  setOpen: (value: OrganizationDialogType | null) => void
  currentRow: UserOrganization | null
  setCurrentRow: React.Dispatch<React.SetStateAction<UserOrganization | null>>
  refreshTrigger: number
  triggerRefresh: () => void
}

const OrganizationsContext = React.createContext<OrganizationsContextType | null>(
  null
)

export function OrganizationsProvider({
  children,
}: {
  children: React.ReactNode
}) {
  const [open, setOpen] = useDialogState<OrganizationDialogType>(null)
  const [currentRow, setCurrentRow] = useState<UserOrganization | null>(null)
  const [refreshTrigger, setRefreshTrigger] = useState(0)

  const triggerRefresh = () => setRefreshTrigger((previous) => previous + 1)

  return (
    <OrganizationsContext
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
    </OrganizationsContext>
  )
}

// eslint-disable-next-line react-refresh/only-export-components
export const useOrganizations = () => {
  const context = React.useContext(OrganizationsContext)

  if (!context) {
    throw new Error(
      'useOrganizations has to be used within <OrganizationsProvider>'
    )
  }

  return context
}
