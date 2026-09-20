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

import { parseQuotaFromDollars, quotaUnitsToDollars } from '@/lib/format'

import type { OrganizationTokenRow } from '../../types'
import {
  buildOrganizationTokenStatusPayload,
  isOrganizationTokenHandover,
  ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
  organizationTokenFormSchema,
  transformOrganizationTokenFormToPayload,
  transformOrganizationTokenToFormDefaults,
} from '../organization-token-form'

function token(
  overrides: Partial<OrganizationTokenRow> = {}
): OrganizationTokenRow {
  return {
    id: 1,
    user_id: 7,
    key: 'sk-abcdefghijklmnop',
    key_preview: 'sk-abcd**********mnop',
    status: 1,
    name: 'ci',
    created_time: 1_700_000_000,
    accessed_time: 1_700_000_000,
    expired_time: -1,
    remain_quota: 0,
    unlimited_quota: true,
    model_limits_enabled: false,
    model_limits: '',
    allow_ips: '',
    used_quota: 0,
    group: '',
    cross_group_retry: false,
    visibility: 'private',
    responsible_user_id: 7,
    ...overrides,
  }
}

const MANAGER = { canManageAllTokens: true }
const MEMBER = { canManageAllTokens: false }

describe('transformOrganizationTokenFormToPayload', () => {
  test('a new key starts enabled and never expiring', () => {
    const payload = transformOrganizationTokenFormToPayload(
      ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
      MANAGER
    )

    expect(payload.status).toBe(1)
    expect(payload.expired_time).toBe(-1)
    expect(payload.unlimited_quota).toBe(true)
    expect(payload.remain_quota).toBe(0)
    expect(payload.model_limits_enabled).toBe(false)
    expect(payload.model_limits).toBe('')
  })

  test('the expiry is converted to the seconds the backend stores', () => {
    const expires = new Date('2030-01-02T03:04:05.000Z')
    const payload = transformOrganizationTokenFormToPayload(
      { ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES, expired_time: expires },
      MANAGER
    )

    expect(payload.expired_time).toBe(Math.floor(expires.getTime() / 1000))
  })

  test('a quota is stored in units, and survives the round trip', () => {
    const payload = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        unlimited_quota: false,
        remain_quota_dollars: 25,
      },
      MANAGER
    )

    expect(payload.remain_quota).toBe(parseQuotaFromDollars(25))
    // A missing quota reads as zero here, which the assertion still rejects.
    expect(quotaUnitsToDollars(payload.remain_quota ?? 0)).toBe(25)
  })

  test('an unlimited key keeps no leftover quota figure', () => {
    const payload = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        unlimited_quota: true,
        remain_quota_dollars: 25,
      },
      MANAGER
    )

    expect(payload.remain_quota).toBe(0)
  })

  test('model limits are enabled exactly when some are chosen', () => {
    const payload = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        model_limits: ['gpt-4o', 'claude-fable-5'],
      },
      MANAGER
    )

    expect(payload.model_limits_enabled).toBe(true)
    expect(payload.model_limits).toBe('gpt-4o,claude-fable-5')
  })

  test('cross-group retry belongs to the auto group and nowhere else', () => {
    const auto = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        group: 'auto',
        cross_group_retry: true,
      },
      MANAGER
    )
    const fixed = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        group: 'vip',
        cross_group_retry: true,
      },
      MANAGER
    )

    expect(auto.cross_group_retry).toBe(true)
    expect(fixed.cross_group_retry).toBe(false)
  })

  test('a manager chooses who holds the key and whether it is public', () => {
    const payload = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        visibility: 'public',
        responsible_user_id: 42,
      },
      MANAGER
    )

    expect(payload.visibility).toBe('public')
    expect(payload.responsible_user_id).toBe(42)
  })

  test('a member cannot hand a key to someone else or publish it', () => {
    const payload = transformOrganizationTokenFormToPayload(
      {
        ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
        visibility: 'public',
        responsible_user_id: 42,
      },
      MEMBER
    )

    expect(payload.visibility).toBe('private')
    expect(payload.responsible_user_id).toBeUndefined()
  })

  test('a member editing a key keeps its holder and visibility untouched', () => {
    const stored = token({
      visibility: 'private',
      responsible_user_id: 7,
      name: 'ci',
    })
    const payload = transformOrganizationTokenFormToPayload(
      { ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES, visibility: 'public' },
      { ...MEMBER, editing: stored }
    )

    expect(payload.visibility).toBe('private')
    expect(payload.responsible_user_id).toBe(7)
    expect(payload.status).toBe(stored.status)
  })

  test('a member editing still falls back to the owner column for the holder', () => {
    const stored = token({ responsible_user_id: 0, user_id: 9 })
    const payload = transformOrganizationTokenFormToPayload(
      ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
      { ...MEMBER, editing: stored }
    )

    expect(payload.responsible_user_id).toBe(9)
  })

  test('the name is trimmed', () => {
    const payload = transformOrganizationTokenFormToPayload(
      { ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES, name: '  ci key  ' },
      MANAGER
    )

    expect(payload.name).toBe('ci key')
  })
})

describe('transformOrganizationTokenToFormDefaults', () => {
  test('a key that never expires leaves the picker empty', () => {
    expect(
      transformOrganizationTokenToFormDefaults(token({ expired_time: -1 }))
        .expired_time
    ).toBeUndefined()
  })

  test('a real deadline comes back as a date', () => {
    const expiresAt = 1_800_000_000
    expect(
      transformOrganizationTokenToFormDefaults(
        token({ expired_time: expiresAt })
      ).expired_time
    ).toEqual(new Date(expiresAt * 1000))
  })

  test('an unlimited key reads as no quota rather than a zero balance', () => {
    expect(
      transformOrganizationTokenToFormDefaults(
        token({ unlimited_quota: true, remain_quota: 5_000_000 })
      ).remain_quota_dollars
    ).toBe(0)
  })

  test('a limited key reads back as the amount the operator typed', () => {
    const stored = token({ unlimited_quota: false, remain_quota: 12_500_000 })
    const values = transformOrganizationTokenToFormDefaults(stored)

    expect(values.remain_quota_dollars).toBe(quotaUnitsToDollars(12_500_000))
  })

  test('model limits and the allowlist come back split', () => {
    const values = transformOrganizationTokenToFormDefaults(
      token({ model_limits: 'gpt-4o,claude-fable-5', allow_ips: '10.0.0.1' })
    )

    expect(values.model_limits).toEqual(['gpt-4o', 'claude-fable-5'])
    expect(values.allow_ips).toBe('10.0.0.1')
  })

  test('an absent visibility reads as private, never as public', () => {
    const values = transformOrganizationTokenToFormDefaults(
      token({ visibility: '' })
    )

    expect(values.visibility).toBe('private')
    expect(transformOrganizationTokenToFormDefaults(token({})).visibility).toBe(
      'private'
    )
  })
})

describe('buildOrganizationTokenStatusPayload', () => {
  test('carries the whole key, since a patch replaces what it names', () => {
    const stored = token({
      remain_quota: 5_000_000,
      unlimited_quota: false,
      expired_time: 1_800_000_000,
      model_limits_enabled: true,
      model_limits: 'gpt-4o',
      allow_ips: '10.0.0.1',
      group: 'vip',
      cross_group_retry: true,
      visibility: 'public',
      responsible_user_id: 42,
    })

    expect(buildOrganizationTokenStatusPayload(stored, 2)).toEqual({
      name: 'ci',
      status: 2,
      expired_time: 1_800_000_000,
      remain_quota: 5_000_000,
      unlimited_quota: false,
      model_limits_enabled: true,
      model_limits: 'gpt-4o',
      allow_ips: '10.0.0.1',
      group: 'vip',
      cross_group_retry: true,
      visibility: 'public',
      responsible_user_id: 42,
    })
  })

  test('a null allowlist is sent as the empty string the column defaults to', () => {
    expect(
      buildOrganizationTokenStatusPayload(token({ allow_ips: null }), 1)
        .allow_ips
    ).toBe('')
  })
})

describe('isOrganizationTokenHandover', () => {
  test('a different member is a hand-over', () => {
    expect(isOrganizationTokenHandover(token(), 42)).toBe(true)
  })

  test('the same member, an absent choice or a zero owner is not', () => {
    expect(isOrganizationTokenHandover(token(), 7)).toBe(false)
    expect(isOrganizationTokenHandover(token(), undefined)).toBe(false)
    expect(isOrganizationTokenHandover(token(), 0)).toBe(false)
  })

  test('the owner column decides when the responsible column is empty', () => {
    expect(
      isOrganizationTokenHandover(token({ responsible_user_id: 0, user_id: 9 }), 9)
    ).toBe(false)
  })
})

describe('organizationTokenFormSchema', () => {
  const valid = {
    ...ORGANIZATION_TOKEN_FORM_DEFAULT_VALUES,
    name: 'ci',
  }

  test('a name is required', () => {
    expect(
      organizationTokenFormSchema.safeParse({ ...valid, name: '   ' }).success
    ).toBe(false)
    expect(organizationTokenFormSchema.safeParse(valid).success).toBe(true)
  })

  test('a limited key needs a quota that is not negative', () => {
    expect(
      organizationTokenFormSchema.safeParse({
        ...valid,
        unlimited_quota: false,
        remain_quota_dollars: undefined,
      }).success
    ).toBe(false)
    expect(
      organizationTokenFormSchema.safeParse({
        ...valid,
        unlimited_quota: false,
        remain_quota_dollars: -1,
      }).success
    ).toBe(false)
    expect(
      organizationTokenFormSchema.safeParse({
        ...valid,
        unlimited_quota: false,
        remain_quota_dollars: 0,
      }).success
    ).toBe(true)
  })

  test('an unlimited key needs no quota at all', () => {
    expect(
      organizationTokenFormSchema.safeParse({
        ...valid,
        unlimited_quota: true,
        remain_quota_dollars: undefined,
      }).success
    ).toBe(true)
  })

  test('an expiry in the past is refused, and an absent one is allowed', () => {
    expect(
      organizationTokenFormSchema.safeParse({
        ...valid,
        expired_time: new Date(Date.now() - 1000),
      }).success
    ).toBe(false)
    expect(
      organizationTokenFormSchema.safeParse({
        ...valid,
        expired_time: new Date(Date.now() + 60_000),
      }).success
    ).toBe(true)
    expect(organizationTokenFormSchema.safeParse(valid).success).toBe(true)
  })

  test('a batch is between one and a hundred keys', () => {
    expect(
      organizationTokenFormSchema.safeParse({ ...valid, token_count: 0 }).success
    ).toBe(false)
    expect(
      organizationTokenFormSchema.safeParse({ ...valid, token_count: 101 })
        .success
    ).toBe(false)
    expect(
      organizationTokenFormSchema.safeParse({ ...valid, token_count: 100 })
        .success
    ).toBe(true)
  })
})
