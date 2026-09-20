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
import { getServerErrorMessageKey } from '@/lib/server-error-message'

import { organizationRoleLabelKey } from '../constants'
import type { OrganizationInvitePublicView } from '../types'
import type { Translate } from './organization-audit'
import {
  organizationInviteErrorBody,
  type OrganizationInviteErrorBody,
} from './organization-invite'

/**
 * One line of the invitation preview.
 *
 * Both strings are already translated, because the only value that needs a
 * lookup is the role; handing the caller finished strings keeps the mapping out
 * of the component.
 */
export interface OrganizationInviteLandingRow {
  key: string
  label: string
  value: string
}

/**
 * The invitation as the recipient sees it before deciding.
 *
 * The inviter's display name falls back to the username because an account
 * without one is ordinary. A missing current email reads as a dash rather than
 * blank: it means the server could not name the signed-in account, which is
 * worth showing when the reader is about to be told the addresses do not match.
 */
export function organizationInviteLandingRows(
  invite: OrganizationInvitePublicView,
  translate: Translate
): OrganizationInviteLandingRow[] {
  return [
    {
      key: 'organization',
      label: translate('Organization'),
      value: invite.organization_name,
    },
    {
      key: 'role',
      label: translate('Role'),
      value: translate(organizationRoleLabelKey(invite.role)),
    },
    {
      key: 'inviter',
      label: translate('Invited by'),
      value: invite.inviter_display_name || invite.inviter_username || '-',
    },
    {
      key: 'targetEmail',
      label: translate('Invitation email'),
      value: invite.target_email || '-',
    },
    {
      key: 'currentEmail',
      label: translate('Signed in as'),
      value: invite.current_user_email || '-',
    },
  ]
}

/**
 * Whether accepting is possible.
 *
 * Both halves are the server's verdict rather than the page's reading of them:
 * an invitation that is no longer pending cannot be accepted at all, and the
 * accept endpoint re-checks the address, so a mismatched viewer would only get
 * a refusal back. The button stays visible and is disabled instead of hidden,
 * so the recipient can see that a way in exists and why it is closed.
 */
export function organizationInviteCanAccept(
  invite: { status?: string; email_matched?: boolean } | null | undefined
): boolean {
  return invite?.status === 'pending' && invite.email_matched === true
}

/**
 * The refusal, whichever way the server sent it.
 *
 * A rejection carries it under `response.data`; a body that says
 * `success: false` *is* the refusal. Accepting answers with a rejection in
 * practice, but reading both costs one line and spares the caller from having to
 * know which one it got.
 */
function inviteRefusal(error: unknown): OrganizationInviteErrorBody {
  const body = organizationInviteErrorBody(error)
  if (body) return body
  if (error && typeof error === 'object') {
    return error as OrganizationInviteErrorBody
  }
  return {}
}

/** An address mismatch has no code of its own; the server sends this message. */
const ORGANIZATION_INVITE_EMAIL_MISMATCH =
  'This invitation was sent to a different email address. Sign in with that address to accept it.'

const ORGANIZATION_INVITE_ALREADY_JOINED =
  'You are already a member of this organization.'

/**
 * What to tell the recipient when the invitation could not be read.
 *
 * Only the codes the backend names are explained, by the shared table; anything
 * else — a mistyped link, a record that is gone — reads as the caller's
 * fallback, because the server's own text for those is a database error rather
 * than something the recipient can act on.
 */
export function organizationInviteLandingErrorMessage(
  error: unknown,
  fallback: string
): string {
  return getServerErrorMessageKey(error) ?? fallback
}

/**
 * What to tell the recipient when accepting failed.
 *
 * Most refusals are codes, which the shared table already explains, but two of
 * them arrive as bare messages with no code at all — the address mismatch and an
 * account that is already a member — and both would otherwise reach the
 * recipient as the server's internal English. Anything unrecognised falls back
 * rather than being shown raw.
 */
export function organizationInviteAcceptErrorMessage(
  error: unknown,
  fallback: string
): string {
  const messageKey = getServerErrorMessageKey(error)
  if (messageKey) return messageKey

  const message = inviteRefusal(error).message ?? ''
  if (message.includes('invite email mismatch')) {
    return ORGANIZATION_INVITE_EMAIL_MISMATCH
  }
  if (message.includes('already joined')) {
    return ORGANIZATION_INVITE_ALREADY_JOINED
  }
  return fallback
}
