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
import type { QueryClient } from '@tanstack/react-query'

import { switchAccountContext } from '@/lib/account-context'
import { getServerErrorCode } from '@/lib/server-error-message'
import { useAccountContextStore } from '@/stores/account-context-store'

import { getOrganization } from '../api'

/**
 * Make the account context agree with the organization in the URL.
 *
 * A deep link — a bookmark, a pasted URL — arrives with whatever context the
 * browser last held. For an active organization the read endpoints compare that
 * context against the id in the path and answer 403
 * `organization_context_mismatch`, so the link would be unusable until the user
 * visited the organization center and opened it from there.
 *
 * The probe is what decides whether switching is even correct: a disabled or
 * dissolved organization cannot be selected as a context (the server refuses
 * it) and does not need to be, because its read endpoints skip the comparison
 * entirely. Probing in the current context therefore succeeds for exactly the
 * organizations that must not be switched to.
 *
 * Returns `true` when the context was switched.
 */
export async function alignOrganizationAccountContext(
  organizationId: number,
  queryClient: QueryClient
): Promise<boolean> {
  if (!Number.isFinite(organizationId) || organizationId <= 0) return false

  const current = useAccountContextStore.getState().accountContext.current
  if (current?.type === 'organization' && current.id === organizationId) {
    return false
  }

  try {
    // Always the member surface: this probes what the *account context* can
    // read, and the platform admin console answers from the administrator's own
    // account without one. There is nothing for an administrator to align.
    await getOrganization('member', organizationId, { silent: true })
    // Readable as-is: either the caller is on their personal context and has
    // read access, or the organization is in a state that skips the check.
    return false
  } catch (error) {
    if (getServerErrorCode(error) !== 'organization_context_mismatch') {
      // Not an alignment problem — the page reports whatever it is.
      return false
    }
  }

  try {
    await switchAccountContext('organization', organizationId)
  } catch {
    // Best effort. The switch can be refused — the caller was removed between
    // the probe and now, the organization was disabled, and so on. The error
    // interceptor has already reported it, and the page degrades to its own
    // "not available" state on its own; failing the navigation instead would
    // replace that with a router error boundary.
    return false
  }
  // Everything cached was fetched under the previous context, and React Query
  // cannot tell the two apart. Nothing of this page has been fetched yet, so
  // this only refetches the rest of the app.
  await queryClient.invalidateQueries()
  return true
}
