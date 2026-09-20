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
  getOrganizationTokenActionFlags,
  maskOrganizationTokenKey,
  organizationTokenErrorMessageKey,
  organizationTokenErrorText,
  organizationTokenKeyPreview,
  organizationTokenStatusMeta,
  organizationTokenVisibilityMeta,
  ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS,
  ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS,
} from '../organization-token'

describe('organizationTokenStatusMeta', () => {
  test('names the four states a key can be stored in', () => {
    expect(organizationTokenStatusMeta(1).labelKey).toBe('Enabled')
    expect(organizationTokenStatusMeta(2).labelKey).toBe('Disabled')
    expect(organizationTokenStatusMeta(3).labelKey).toBe('Expired')
    expect(organizationTokenStatusMeta(4).labelKey).toBe('Exhausted')
  })

  test('an unknown or absent status is said to be unknown', () => {
    expect(organizationTokenStatusMeta(9).labelKey).toBe('Unknown status')
    expect(organizationTokenStatusMeta().labelKey).toBe('Unknown status')
    expect(organizationTokenStatusMeta(0).variant).toBe('neutral')
  })

  test('the filter offers exactly the four stored states', () => {
    expect(ORGANIZATION_TOKEN_STATUS_FILTER_OPTIONS).toEqual([
      { value: '1', labelKey: 'Enabled' },
      { value: '2', labelKey: 'Disabled' },
      { value: '3', labelKey: 'Expired' },
      { value: '4', labelKey: 'Exhausted' },
    ])
  })
})

describe('organizationTokenVisibilityMeta', () => {
  test('an absent visibility reads as private, which is what the column stores', () => {
    expect(organizationTokenVisibilityMeta().labelKey).toBe('Private')
    expect(organizationTokenVisibilityMeta('').labelKey).toBe('Private')
    expect(organizationTokenVisibilityMeta('something')).toEqual(
      organizationTokenVisibilityMeta('private')
    )
  })

  test('the two visibilities say who the key is for', () => {
    expect(organizationTokenVisibilityMeta('private').descriptionKey).toBe(
      'Only the responsible member can use this key.'
    )
    expect(organizationTokenVisibilityMeta('public').descriptionKey).toBe(
      'Any active member can use this key.'
    )
  })
})

describe('maskOrganizationTokenKey', () => {
  test('keeps the prefix and both ends of the key', () => {
    expect(maskOrganizationTokenKey('sk-abcdefghijklmnop')).toBe(
      'sk-abcd**********mnop'
    )
  })

  test('adds the prefix a bare key is missing', () => {
    expect(maskOrganizationTokenKey('abcdefghijklmnop')).toBe(
      'sk-abcd**********mnop'
    )
  })

  test('leaves a key too short to mask alone', () => {
    expect(maskOrganizationTokenKey('sk-abcd')).toBe('sk-abcd')
    expect(maskOrganizationTokenKey('sk-abcdefgh')).toBe('sk-abcdefgh')
  })

  test('nothing to mask when there is no key', () => {
    expect(maskOrganizationTokenKey()).toBe('')
    expect(maskOrganizationTokenKey('   ')).toBe('')
  })
})

describe('organizationTokenKeyPreview', () => {
  test("prefers the mask the backend computed", () => {
    expect(
      organizationTokenKeyPreview({
        key: 'sk-abcdefghijklmnop',
        key_preview: 'sk-zzzz**********yyyy',
      })
    ).toBe('sk-zzzz**********yyyy')
  })

  test('masks the key itself when no preview came with it', () => {
    expect(
      organizationTokenKeyPreview({ key: 'sk-abcdefghijklmnop' })
    ).toBe('sk-abcd**********mnop')
    expect(organizationTokenKeyPreview({ key: '' })).toBe('')
  })
})

describe('getOrganizationTokenActionFlags', () => {
  const key = {
    responsible_user_id: 7,
    user_id: 7,
    visibility: 'private',
    key: 'sk-abcdefghijklmnop',
  }
  const manager = {
    canManageAllTokens: true,
    currentUserId: 7,
    readOnly: false,
  }
  const member = { canManageAllTokens: false, currentUserId: 7, readOnly: false }

  test('an organization-wide manager may act on every key', () => {
    expect(getOrganizationTokenActionFlags(key, manager)).toEqual({
      canEdit: true,
      canDelete: true,
      canCopy: true,
    })
    expect(
      getOrganizationTokenActionFlags(
        { ...key, responsible_user_id: 99 },
        manager
      ).canDelete
    ).toBe(true)
  })

  test('a member reaches the keys they are responsible for', () => {
    expect(getOrganizationTokenActionFlags(key, member)).toEqual({
      canEdit: true,
      canDelete: true,
      canCopy: true,
    })
  })

  test('a member may copy a public key but not change it', () => {
    expect(
      getOrganizationTokenActionFlags(
        { ...key, visibility: 'public' },
        member
      )
    ).toEqual({ canEdit: false, canDelete: false, canCopy: true })
  })

  test('a member may not touch a key held by someone else', () => {
    expect(
      getOrganizationTokenActionFlags(
        { ...key, responsible_user_id: 99 },
        member
      )
    ).toEqual({ canEdit: false, canDelete: false, canCopy: false })
  })

  test('a key with no owner belongs to nobody, not to user zero', () => {
    expect(
      getOrganizationTokenActionFlags(
        { ...key, responsible_user_id: 0, user_id: 0 },
        member
      )
    ).toEqual({ canEdit: false, canDelete: false, canCopy: false })
    expect(
      getOrganizationTokenActionFlags(
        { ...key, responsible_user_id: 0, user_id: 0 },
        manager
      )
    ).toEqual({ canEdit: true, canDelete: true, canCopy: true })
  })

  test('falling back to the owner column still counts as responsibility', () => {
    expect(
      getOrganizationTokenActionFlags(
        { ...key, responsible_user_id: 0, user_id: 7 },
        member
      ).canEdit
    ).toBe(true)
  })

  test('read-only keeps copy, which is a read', () => {
    expect(
      getOrganizationTokenActionFlags(key, { ...manager, readOnly: true })
    ).toEqual({ canEdit: false, canDelete: false, canCopy: true })
    expect(
      getOrganizationTokenActionFlags(key, { ...member, readOnly: true })
    ).toEqual({ canEdit: false, canDelete: false, canCopy: true })
  })

  test('there is nothing to copy when the secret is absent', () => {
    expect(getOrganizationTokenActionFlags({ ...key, key: '' }, manager).canCopy).toBe(
      false
    )
  })
})

describe('organizationTokenErrorMessageKey', () => {
  test('explains the three refusals that have their own sentence', () => {
    expect(
      organizationTokenErrorMessageKey({
        response: { data: { code: 'organization_token_enable_forbidden' } },
      })
    ).toBe(
      'This key was disabled by a higher-privilege administrator. Your account cannot enable it.'
    )
    expect(
      organizationTokenErrorMessageKey({
        response: {
          data: { code: 'organization_token_responsible_member_disabled' },
        },
      })
    ).toBe(
      'The responsible member for this key is disabled. Enable the member before enabling the key.'
    )
    expect(
      organizationTokenErrorMessageKey({
        response: {
          data: { code: 'organization_token_responsible_user_disabled' },
        },
      })
    ).toBe(
      'The responsible user for this key is disabled by the platform. Enable the user before enabling the key.'
    )
  })

  test('an unrecognized refusal has no sentence of its own', () => {
    expect(
      organizationTokenErrorMessageKey({
        response: { data: { code: 'organization_operation_blocked' } },
      })
    ).toBeNull()
    expect(organizationTokenErrorMessageKey(undefined)).toBeNull()
    expect(organizationTokenErrorMessageKey({ response: {} })).toBeNull()
  })
})

describe('organizationTokenErrorText', () => {
  test('a mapped code wins over the message the backend sent', () => {
    expect(
      organizationTokenErrorText(
        {
          response: {
            data: {
              code: 'organization_token_enable_forbidden',
              message: 'permission denied',
            },
          },
        },
        'Failed to save organization key'
      )
    ).toBe(
      'This key was disabled by a higher-privilege administrator. Your account cannot enable it.'
    )
  })

  test('an unmapped refusal falls back to its message, then to the caller', () => {
    expect(
      organizationTokenErrorText(
        { response: { data: { message: 'organization token limit reached' } } },
        'Failed to save organization key'
      )
    ).toBe('organization token limit reached')
    expect(
      organizationTokenErrorText(new Error('offline'), 'Failed to save organization key')
    ).toBe('Failed to save organization key')
  })
})

describe('ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS', () => {
  test('the last shortcut is the absence of a deadline', () => {
    expect(ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS.at(-1)).toEqual({
      labelKey: 'Never expire',
      seconds: null,
    })
  })

  test('every shortcut before it names a real span', () => {
    for (const shortcut of ORGANIZATION_TOKEN_EXPIRY_SHORTCUTS.slice(0, -1)) {
      expect(shortcut.seconds).toBeGreaterThan(0)
      expect(shortcut.labelKey).not.toBe('')
    }
  })
})
