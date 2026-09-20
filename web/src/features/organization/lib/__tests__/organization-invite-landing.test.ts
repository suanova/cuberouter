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

import type { OrganizationInvitePublicView } from '../../types'
import {
  organizationInviteAcceptErrorMessage,
  organizationInviteCanAccept,
  organizationInviteLandingErrorMessage,
  organizationInviteLandingRows,
} from '../organization-invite-landing'

/**
 * `t` as the helper receives it: a lookup that resolves the keys it knows and
 * hands anything else back unchanged, which is how react-i18next treats a
 * missing key.
 */
const translate = (key: string) => {
  const catalog: Record<string, string> = {
    Owner: '所有者',
    Member: '成员',
  }
  return catalog[key] ?? key
}

function invite(
  overrides: Partial<OrganizationInvitePublicView> = {}
): OrganizationInvitePublicView {
  return {
    id: 7,
    organization_id: 42,
    organization_name: 'Acme Research',
    organization_slug: 'acme-research',
    role: 'member',
    target_email: 'invited@example.com',
    status: 'pending',
    inviter_display_name: 'Dana',
    inviter_username: 'dana',
    current_user_email: 'invited@example.com',
    email_matched: true,
    ...overrides,
  }
}

describe('organizationInviteLandingRows', () => {
  test('the preview names the organization, the role and both addresses', () => {
    const rows = organizationInviteLandingRows(invite(), translate)

    expect(rows.map((row) => row.key)).toEqual([
      'organization',
      'role',
      'inviter',
      'targetEmail',
      'currentEmail',
    ])
    expect(rows[0].label).toBe('Organization')
    expect(rows[0].value).toBe('Acme Research')
    expect(rows[2].value).toBe('Dana')
    expect(rows[3].value).toBe('invited@example.com')
    expect(rows[4].value).toBe('invited@example.com')
  })

  test('the role is shown in the reader’s language', () => {
    const rows = organizationInviteLandingRows(invite({ role: 'owner' }), translate)

    expect(rows[1].value).toBe('所有者')
  })

  test('an inviter without a display name falls back to the username', () => {
    const rows = organizationInviteLandingRows(
      invite({ inviter_display_name: '' }),
      translate
    )

    expect(rows[2].value).toBe('dana')
  })

  test('an inviter the server could not name reads as a dash, not as blank', () => {
    const rows = organizationInviteLandingRows(
      invite({ inviter_display_name: '', inviter_username: undefined }),
      translate
    )

    expect(rows[2].value).toBe('-')
  })

  test('a viewer the server could not identify has no current email', () => {
    const rows = organizationInviteLandingRows(
      invite({ current_user_email: undefined }),
      translate
    )

    expect(rows[4].value).toBe('-')
  })
})

describe('organizationInviteCanAccept', () => {
  test('a pending invitation addressed to the signed-in account can be accepted', () => {
    expect(organizationInviteCanAccept(invite())).toBe(true)
  })

  test('an invitation that is no longer pending cannot be accepted', () => {
    expect(organizationInviteCanAccept(invite({ status: 'accepted' }))).toBe(false)
    expect(organizationInviteCanAccept(invite({ status: 'revoked' }))).toBe(false)
    expect(organizationInviteCanAccept(invite({ status: 'expired' }))).toBe(false)
  })

  test('a pending invitation for another address cannot be accepted', () => {
    expect(organizationInviteCanAccept(invite({ email_matched: false }))).toBe(false)
  })

  test('the absence of an invitation cannot be accepted', () => {
    expect(organizationInviteCanAccept(null)).toBe(false)
    expect(organizationInviteCanAccept(undefined)).toBe(false)
  })
})

describe('organizationInviteLandingErrorMessage', () => {
  test('a coded refusal is explained rather than shown raw', () => {
    // The backend marks an expired or already-accepted invitation with this
    // code, and its own message for it is internal wording.
    const message = organizationInviteLandingErrorMessage(
      {
        response: {
          status: 400,
          data: {
            code: 'organization_invite_unavailable',
            message: 'invite is not pending',
          },
        },
      },
      'Failed to load the invitation'
    )

    expect(message).toBe(
      'This invitation is no longer available. Ask an administrator for a new one.'
    )
  })

  test('an uncoded failure reads as the fallback, not as a database error', () => {
    // A mistyped link answers 404 `record not found`; showing that to a
    // recipient would be telling them about the schema.
    const message = organizationInviteLandingErrorMessage(
      { response: { status: 404, data: { message: 'record not found' } } },
      'Failed to load the invitation'
    )

    expect(message).toBe('Failed to load the invitation')
  })

  test('a failure with no response at all reads as the fallback', () => {
    expect(
      organizationInviteLandingErrorMessage(
        new Error('Network Error'),
        'Failed to load the invitation'
      )
    ).toBe('Failed to load the invitation')
  })
})

describe('organizationInviteAcceptErrorMessage', () => {
  test('a coded refusal is explained rather than shown raw', () => {
    const message = organizationInviteAcceptErrorMessage(
      {
        response: {
          status: 403,
          data: {
            code: 'organization_disabled',
            message: 'organization disabled',
          },
        },
      },
      'Failed to accept the invitation'
    )

    expect(message).toBe(
      'This organization is disabled. Contact a platform administrator.'
    )
  })

  test('an address mismatch is explained, since the server sends no code for it', () => {
    const message = organizationInviteAcceptErrorMessage(
      {
        response: {
          status: 400,
          data: { message: 'invite email mismatch' },
        },
      },
      'Failed to accept the invitation'
    )

    expect(message).toContain('different email address')
  })

  test('already being a member is explained, since the server sends no code for it', () => {
    const message = organizationInviteAcceptErrorMessage(
      {
        response: {
          status: 409,
          data: { message: 'user already joined organization' },
        },
      },
      'Failed to accept the invitation'
    )

    expect(message).toBe('You are already a member of this organization.')
  })

  test('a refusal delivered as a 2xx body is explained like a rejection', () => {
    // Defensive: accepting answers with a rejection in practice, but a body
    // that says `success: false` is a refusal too, and its code is what decides
    // the message.
    const message = organizationInviteAcceptErrorMessage(
      {
        success: false,
        code: 'organization_dissolved',
        message: 'organization dissolved',
      },
      'Failed to accept the invitation'
    )

    expect(message).toBe(
      'This organization has been dissolved and can no longer be used.'
    )
  })

  test('a 2xx refusal with no code at all reads as the fallback', () => {
    expect(
      organizationInviteAcceptErrorMessage(
        { success: false, message: 'invite is not pending' },
        'Failed to accept the invitation'
      )
    ).toBe('Failed to accept the invitation')
  })

  test('an unrecognised failure reads as the fallback rather than as the server’s wording', () => {
    expect(
      organizationInviteAcceptErrorMessage(
        { response: { status: 400, data: { message: 'invalid invite acceptance request' } } },
        'Failed to accept the invitation'
      )
    ).toBe('Failed to accept the invitation')
  })
})
