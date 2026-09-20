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
import { describe, expect, test } from 'vitest'

import {
  organizationInviteCanRetry,
  organizationInviteCreateFailureAction,
  organizationInviteDeliveryErrorMessageKey,
  organizationInviteDisplayStatus,
  organizationInviteStatusMeta,
} from '../organization-invite'

describe('organizationInviteDisplayStatus', () => {
  test('a pending invite is described by how its delivery went', () => {
    expect(organizationInviteDisplayStatus('pending', 'sent')).toBe('pending')
    expect(organizationInviteDisplayStatus('pending', 'pending')).toBe('sending')
    expect(organizationInviteDisplayStatus('pending', 'rejected')).toBe(
      'rejected'
    )
    expect(organizationInviteDisplayStatus('pending', 'unknown')).toBe(
      'deliveryUnknown'
    )
    expect(organizationInviteDisplayStatus('pending', 'failed')).toBe('failed')
  })

  test('a pending invite with no delivery outcome at all counts as failed', () => {
    expect(organizationInviteDisplayStatus('pending')).toBe('failed')
    expect(organizationInviteDisplayStatus('pending', '')).toBe('failed')
  })

  test('a settled invite ignores delivery, which only describes the send', () => {
    expect(organizationInviteDisplayStatus('accepted', 'sent')).toBe('accepted')
    expect(organizationInviteDisplayStatus('expired', 'failed')).toBe('expired')
    expect(organizationInviteDisplayStatus('revoked')).toBe('revoked')
  })

  test('an absent status is unknown rather than a failure', () => {
    expect(organizationInviteDisplayStatus()).toBe('unknown')
  })
})

describe('organizationInviteStatusMeta', () => {
  test('only the states the recipient cannot act on are errors', () => {
    expect(
      organizationInviteStatusMeta('pending', 'sent').variant
    ).toBe('info')
    expect(organizationInviteStatusMeta('pending', 'pending').variant).toBe(
      'cyan'
    )
    expect(organizationInviteStatusMeta('pending', 'unknown').variant).toBe(
      'warning'
    )
    expect(organizationInviteStatusMeta('pending', 'rejected').variant).toBe(
      'danger'
    )
    expect(organizationInviteStatusMeta('pending', 'failed').variant).toBe(
      'danger'
    )
  })

  test('every display status has a label', () => {
    for (const status of [
      'pending',
      'accepted',
      'expired',
      'revoked',
    ] as const) {
      expect(organizationInviteStatusMeta(status).labelKey).not.toBe('')
    }
    expect(organizationInviteStatusMeta(undefined).labelKey).toBe(
      'Unknown status'
    )
  })
})

describe('organizationInviteCanRetry', () => {
  test('retries the states where the mail demonstrably did not arrive', () => {
    expect(
      organizationInviteCanRetry(
        { status: 'pending', delivery_status: 'failed' },
        true,
        false
      )
    ).toBe(true)
    expect(
      organizationInviteCanRetry(
        { status: 'pending', delivery_status: 'unknown' },
        true,
        false
      )
    ).toBe(true)
  })

  test('does not retry an in-flight send, which a retry would race', () => {
    expect(
      organizationInviteCanRetry(
        { status: 'pending', delivery_status: 'pending' },
        true,
        false
      )
    ).toBe(false)
  })

  test('does not retry a rejected address or a settled invitation', () => {
    expect(
      organizationInviteCanRetry(
        { status: 'pending', delivery_status: 'rejected' },
        true,
        false
      )
    ).toBe(false)
    expect(
      organizationInviteCanRetry({ status: 'accepted', delivery_status: 'sent' }, true, false)
    ).toBe(false)
  })

  test('requires the create capability and a writable organization', () => {
    const failed = { status: 'pending', delivery_status: 'failed' }
    expect(organizationInviteCanRetry(failed, false, false)).toBe(false)
    expect(organizationInviteCanRetry(failed, true, true)).toBe(false)
    expect(organizationInviteCanRetry(null, true, false)).toBe(false)
  })
})

describe('organizationInviteDeliveryErrorMessageKey', () => {
  test('a delivery failure is explained by the delivery status it carries', () => {
    const code = 'organization_invite_delivery_failed'
    expect(
      organizationInviteDeliveryErrorMessageKey({
        response: { data: { code, delivery_status: 'rejected' } },
      })
    ).toBe(
      'The email address is undeliverable; revoke the invite and enter another address'
    )
    expect(
      organizationInviteDeliveryErrorMessageKey({
        response: { data: { code, delivery_status: 'unknown' } },
      })
    ).toBe('Delivery status is unknown; you can retry manually later')
    expect(
      organizationInviteDeliveryErrorMessageKey({
        response: { data: { code, delivery_status: 'failed' } },
      })
    ).toBe('Email delivery failed; the invite record was retained')
  })

  test('an unrecognized failure has no explanation to offer', () => {
    expect(
      organizationInviteDeliveryErrorMessageKey({
        response: { data: { code: 'organization_operation_blocked' } },
      })
    ).toBeNull()
    expect(organizationInviteDeliveryErrorMessageKey(undefined)).toBeNull()
  })
})

describe('organizationInviteCreateFailureAction', () => {
  test('a delivery failure means the record was written, so the list is stale', () => {
    expect(
      organizationInviteCreateFailureAction({
        response: {
          data: {
            code: 'organization_invite_delivery_failed',
            delivery_status: 'failed',
          },
        },
      })
    ).toEqual({
      action: 'deliveryFailure',
      messageKey: 'Email delivery failed; the invite record was retained',
    })
  })

  test('a live pending invite is a question, not an error', () => {
    expect(
      organizationInviteCreateFailureAction({
        response: { data: { code: 'organization_invite_already_sent' } },
      })
    ).toEqual({ action: 'confirmResend' })
  })

  test('an in-flight send is reported without re-reading the list', () => {
    expect(
      organizationInviteCreateFailureAction({
        response: {
          data: { code: 'organization_invite_delivery_in_progress' },
        },
      })
    ).toEqual({
      action: 'knownDeliveryFailure',
      messageKey: 'The invitation email is being sent; refresh again shortly',
    })
  })

  test('the already-joined refusal is matched on its message, since it has no code', () => {
    expect(
      organizationInviteCreateFailureAction({
        response: { data: { message: 'user already joined organization' } },
      })
    ).toEqual({ action: 'userAlreadyJoined' })
  })

  test('anything else is left to the generic message', () => {
    expect(organizationInviteCreateFailureAction(undefined)).toEqual({
      action: 'generic',
    })
  })
})
