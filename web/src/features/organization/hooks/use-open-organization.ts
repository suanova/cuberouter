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
import { useCallback } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ERROR_MESSAGES } from '../constants'
import { isOrganizationEnterable } from '../lib'
import type { UserOrganization } from '../types'
import { useEnterOrganization } from './use-enter-organization'

/** The fields deciding how an organization is opened. */
type OpenableOrganization = Pick<
  UserOrganization,
  'id' | 'status' | 'access_mode'
>

/**
 * Opens an organization from a list surface, reporting a refused context switch.
 *
 * `useEnterOrganization` is the mechanism: select the organization as the account
 * context when it can be one, then navigate. What it does not do is tell the user
 * when the switch is refused — and that has to be reported *before* the
 * navigation, because the destination would otherwise render nothing but a context
 * mismatch. Every surface that opens an organization needs exactly that, so it
 * lives here instead of at each call site.
 */
export function useOpenOrganization() {
  const { t } = useTranslation()
  const enterOrganization = useEnterOrganization()

  return useCallback(
    async (organization: OpenableOrganization) => {
      try {
        await enterOrganization(
          organization.id,
          isOrganizationEnterable(organization)
        )
      } catch (error) {
        toast.error(
          error instanceof Error && error.message
            ? error.message
            : t(ERROR_MESSAGES.UNEXPECTED)
        )
      }
    },
    [enterOrganization, t]
  )
}
