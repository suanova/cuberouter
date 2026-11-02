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

import type { UsageLog } from '../../data/schema'
import { getLogBillingAccount, getLogResponsibleUserName } from '../format'

function logRow(overrides: Partial<UsageLog> = {}): UsageLog {
  return {
    id: 1,
    user_id: 42,
    created_at: 1_700_000_000,
    type: 2,
    content: '',
    username: 'alice',
    token_name: 'key-1',
    model_name: 'gpt-4o',
    quota: 1_000,
    prompt_tokens: 10,
    completion_tokens: 5,
    use_time: 1,
    is_stream: false,
    channel: 1,
    channel_name: '',
    token_id: 1,
    group: 'default',
    ip: '',
    other: '',
    request_id: '',
    upstream_request_id: '',
    scope_type: '',
    scope_id: 0,
    billing_account_type: '',
    billing_account_id: 0,
    organization_id: 0,
    organization_name: '',
    actor_user_id: 0,
    creator_user_id: 0,
    creator_name: '',
    responsible_user_id: 0,
    responsible_name: '',
    responsible_username: '',
    responsible_display_name: '',
    ...overrides,
  }
}

describe('log billing account', () => {
  test('treats a legacy empty billing type as the caller personal account', () => {
    expect(getLogBillingAccount(logRow())).toEqual({
      type: 'personal',
      name: '',
      id: 42,
    })
  })

  test('returns the organization name and id for an organization charge', () => {
    expect(
      getLogBillingAccount(
        logRow({
          billing_account_type: 'organization',
          billing_account_id: 9,
          organization_id: 7,
          organization_name: 'Acme',
        })
      )
    ).toEqual({ type: 'organization', name: 'Acme', id: 7 })
  })

  test('falls back to the organization id when the name is missing', () => {
    expect(
      getLogBillingAccount(
        logRow({
          billing_account_type: 'organization',
          organization_id: 7,
        })
      )
    ).toEqual({ type: 'organization', name: '', id: 7 })
  })
})

describe('log responsible user', () => {
  test('prefers the hydrated display name over the snapshot name', () => {
    expect(
      getLogResponsibleUserName(
        logRow({
          responsible_display_name: 'Alice Zhang',
          responsible_name: 'alice',
          responsible_username: 'alice-login',
          username: 'caller',
        })
      )
    ).toBe('Alice Zhang')
  })

  test('falls back to the snapshot name, then the caller', () => {
    expect(
      getLogResponsibleUserName(logRow({ responsible_name: 'alice' }))
    ).toBe('alice')
    expect(getLogResponsibleUserName(logRow())).toBe('alice')
  })
})
