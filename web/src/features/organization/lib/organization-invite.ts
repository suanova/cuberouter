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
import type { StatusVariant } from '@/components/status-badge'

/**
 * What an invitation is actually doing, once delivery is taken into account.
 *
 * A pending invitation is not one state but five: the record is written before
 * the mail is sent, so a pending row may be waiting on the mailer, have failed,
 * or have been rejected outright by the recipient's server. The stored `status`
 * alone cannot tell those apart, which is why `delivery_status` is folded in
 * here rather than read separately at each call site.
 */
export type OrganizationInviteDisplayStatus =
  | 'pending'
  | 'sending'
  | 'failed'
  | 'rejected'
  | 'deliveryUnknown'
  | 'accepted'
  | 'expired'
  | 'revoked'
  | 'unknown'

export function organizationInviteDisplayStatus(
  status?: string,
  deliveryStatus?: string
): OrganizationInviteDisplayStatus {
  if (status !== 'pending') {
    return (status as OrganizationInviteDisplayStatus) || 'unknown'
  }
  switch (deliveryStatus) {
    case 'sent':
      return 'pending'
    case 'pending':
      return 'sending'
    case 'rejected':
      return 'rejected'
    case 'unknown':
      return 'deliveryUnknown'
    default:
      // A pending invitation with no successful send is a failed one; the
      // record survives so the address can be retried.
      return 'failed'
  }
}

/**
 * The badge for a display status.
 *
 * `sending` and `deliveryUnknown` are not failures — they are states where the
 * operator should wait or retry — so they are not coloured as errors.
 */
export const ORGANIZATION_INVITE_STATUS_META: Record<
  OrganizationInviteDisplayStatus,
  { labelKey: string; variant: StatusVariant }
> = {
  accepted: { labelKey: 'Accepted', variant: 'success' },
  pending: { labelKey: 'Pending', variant: 'info' },
  sending: { labelKey: 'Sending', variant: 'cyan' },
  failed: { labelKey: 'Email delivery failed', variant: 'danger' },
  rejected: { labelKey: 'Email undeliverable', variant: 'danger' },
  deliveryUnknown: { labelKey: 'Delivery status unknown', variant: 'warning' },
  expired: { labelKey: 'Expired', variant: 'warning' },
  revoked: { labelKey: 'Revoked', variant: 'neutral' },
  unknown: { labelKey: 'Unknown status', variant: 'neutral' },
}

export function organizationInviteStatusMeta(
  status?: string,
  deliveryStatus?: string
): { labelKey: string; variant: StatusVariant } {
  return ORGANIZATION_INVITE_STATUS_META[
    organizationInviteDisplayStatus(status, deliveryStatus)
  ]
}

/**
 * Whether the invitation can be sent again.
 *
 * Only the two states where the mail demonstrably did not (or may not) have
 * arrived: retrying a `sending` invitation would race the in-flight attempt,
 * and a `rejected` address cannot be fixed by mailing it again.
 */
export function organizationInviteCanRetry(
  invite: { status?: string; delivery_status?: string } | null | undefined,
  canCreateInvite: boolean,
  readOnly: boolean
): boolean {
  if (!canCreateInvite || readOnly) return false
  const displayStatus = organizationInviteDisplayStatus(
    invite?.status,
    invite?.delivery_status
  )
  return displayStatus === 'failed' || displayStatus === 'deliveryUnknown'
}

/**
 * The error body `writeOrganizationError` builds for an invitation failure.
 *
 * `delivery_status` only accompanies a delivery failure; the other codes are
 * distinguished by `code` alone.
 */
export type OrganizationInviteErrorBody = {
  code?: string
  delivery_status?: string
  message?: string
}

/** The shape an axios rejection carries; only the server's body is read. */
export type OrganizationInviteApiError = {
  response?: { data?: OrganizationInviteErrorBody }
}

/**
 * The server's body for a rejected invitation request, if there is one.
 *
 * The parameter is `unknown` because that is what a caught error is; the shape
 * above describes what this build reads out of it, not what a caller has to
 * have already proven.
 */
function inviteErrorBody(error: unknown): OrganizationInviteErrorBody | undefined {
  return (error as OrganizationInviteApiError | null | undefined)?.response?.data
}

/**
 * The message key for a delivery failure, or `null` when the failure is not one
 * of the delivery outcomes this build knows how to explain.
 */
export function organizationInviteDeliveryErrorMessageKey(
  error: unknown
): string | null {
  const data = inviteErrorBody(error)
  switch (data?.code) {
    case 'organization_invite_delivery_failed':
      if (data.delivery_status === 'unknown') {
        return 'Delivery status is unknown; you can retry manually later'
      }
      if (data.delivery_status === 'rejected') {
        return 'The email address is undeliverable; revoke the invite and enter another address'
      }
      return 'Email delivery failed; the invite record was retained'
    case 'organization_invite_delivery_in_progress':
      return 'The invitation email is being sent; refresh again shortly'
    case 'organization_invite_recipient_rejected':
      return 'The email address is undeliverable; revoke the invite and enter another address'
    default:
      return null
  }
}

/**
 * What the operator has to do about a failed send, which is not the same for
 * every failure.
 *
 * A delivery failure means the record already exists and the mail did not go
 * out, so the list is stale and has to be re-read. `already_sent` means a live
 * pending invitation is already in the recipient's inbox, and the only thing
 * left to decide is whether to send another one — a question, not an error.
 */
export type OrganizationInviteCreateFailure =
  | { action: 'deliveryFailure'; messageKey: string }
  | { action: 'confirmResend' }
  | { action: 'knownDeliveryFailure'; messageKey: string }
  | { action: 'userAlreadyJoined' }
  | { action: 'generic' }

export function organizationInviteCreateFailureAction(
  error: unknown
): OrganizationInviteCreateFailure {
  const messageKey = organizationInviteDeliveryErrorMessageKey(error)
  const data = inviteErrorBody(error)
  const code = data?.code

  if (code === 'organization_invite_delivery_failed' && messageKey) {
    return { action: 'deliveryFailure', messageKey }
  }
  if (code === 'organization_invite_already_sent') {
    return { action: 'confirmResend' }
  }
  if (messageKey) {
    return { action: 'knownDeliveryFailure', messageKey }
  }
  // The backend answers this one with a bare message and a 409, with no code of
  // its own to match on.
  if ((data?.message ?? '').includes('already joined')) {
    return { action: 'userAlreadyJoined' }
  }
  return { action: 'generic' }
}
