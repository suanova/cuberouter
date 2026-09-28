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

import { userDeleteFailureMessage } from '../user-delete-failure'

/**
 * `t` as the helper receives it: resolves the keys it knows and hands anything
 * else back unchanged, which is how react-i18next treats a missing key.
 */
const translate = (key: string) => {
  const catalog: Record<string, string> = {
    Blockers: '阻断项',
    'This operation is blocked. Resolve the listed blockers and try again.':
      '该操作已被阻断。请先解决列出的阻断项后重试。',
  }
  return catalog[key] ?? key
}

/**
 * The shape the axios interceptor throws for a 409 refusal: the response body
 * hangs off `error.response.data`.
 */
function blockedError(body: Record<string, unknown>) {
  return { response: { status: 409, data: body } }
}

describe('user delete failure message', () => {
  test('names the organization and lists the organization blockers', () => {
    const message = userDeleteFailureMessage(
      blockedError({
        success: false,
        code: 'organization_operation_blocked',
        message:
          'organization operation blocked: organization Acme still owns this account as its owner; transfer ownership first',
        blockers: ['active_owner'],
      }),
      translate
    )

    expect(message).toContain('Acme')
    expect(message).toContain('transfer ownership first')
    expect(message).toContain('该操作已被阻断')
    expect(message).toContain('阻断项: active_owner')
  })

  test('lists every blocker when the account holds organization keys', () => {
    const message = userDeleteFailureMessage(
      blockedError({
        code: 'organization_operation_blocked',
        message: 'organization Acme still has keys held by this account',
        blockers: ['organization_keys'],
      }),
      translate
    )

    expect(message).toContain('阻断项: organization_keys')
  })

  test('keeps the server message when the refusal carries no code', () => {
    const message = userDeleteFailureMessage(
      blockedError({ success: false, message: 'database is unreachable' }),
      translate
    )

    expect(message).toBe('database is unreachable')
  })

  test('reads a bare response body as well as an interceptor error', () => {
    const message = userDeleteFailureMessage(
      {
        code: 'organization_operation_blocked',
        message: 'organization Acme is blocked',
        blockers: ['active_owner'],
      },
      translate
    )

    expect(message).toContain('organization Acme is blocked')
    expect(message).toContain('阻断项: active_owner')
  })

  test('returns an empty message when there is nothing to explain', () => {
    expect(
      userDeleteFailureMessage(new Error('Network Error'), translate)
    ).toBe('')
  })

  test('ignores non-string blockers instead of printing them', () => {
    const message = userDeleteFailureMessage(
      blockedError({
        message: 'organization Acme is blocked',
        blockers: ['active_owner', 7, '', null],
      }),
      translate
    )

    expect(message).toBe('organization Acme is blocked 阻断项: active_owner')
  })
})
