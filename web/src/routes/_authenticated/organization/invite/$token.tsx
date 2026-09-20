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
import { createFileRoute } from '@tanstack/react-router'

import { OrganizationInviteLanding } from '@/features/organization'

/**
 * Where an emailed invitation link lands.
 *
 * The path is the one the backend puts in the mail
 * (`service/organization_invite.go`), so it is fixed, not a naming choice — and
 * it is deliberately not under `/organizations`, which is the member workspace
 * and requires a membership the recipient does not have yet.
 *
 * Sitting inside the authenticated layout is what makes the signed-out case
 * work: the layout's guard sends the recipient to sign in and returns them to
 * this URL, which is the same detour the page would have had to make itself.
 */
export const Route = createFileRoute('/_authenticated/organization/invite/$token')(
  {
    component: OrganizationInviteLanding,
  }
)
