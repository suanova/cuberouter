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
import { describe, expect, it } from 'vitest'

import {
  getPlatformOrganizationActionFlags,
  platformOrganizationQuotaDelta,
  platformOrganizationRemainingQuota,
  platformOrganizationStatusParam,
} from '../organization-platform'
import {
  platformOrganizationEditSchema,
  transformPlatformOrganizationEditToRequests,
  transformPlatformOrganizationToFormDefaults,
} from '../organization-platform-form'
import type { OrganizationManagementView } from '@/features/organization/types'

function row(
  overrides: Partial<OrganizationManagementView> = {}
): OrganizationManagementView {
  return {
    id: 1,
    name: 'Acme',
    slug: 'acme',
    description: '',
    group: 'default',
    status: 'active',
    quota: 500_000,
    used_quota: 0,
    request_count: 0,
    owner_user_id: 7,
    created_by: 7,
    created_at: 0,
    updated_at: 0,
    dissolved_at: 0,
    owner_username: 'owner',
    owner_display_name: 'Owner',
    owner_email: 'owner@example.com',
    active_member_count: 1,
    disabled_member_count: 0,
    total_member_count: 1,
    enabled_token_count: 1,
    disabled_token_count: 0,
    total_token_count: 1,
    ...overrides,
  }
}

describe('platformOrganizationStatusParam', () => {
  it('joins the chosen statuses the way the endpoint parses them', () => {
    expect(
      platformOrganizationStatusParam(['active', 'disabled', 'dissolved'])
    ).toBe('active,disabled,dissolved')
  })

  it('sends nothing when no status is chosen, which the endpoint reads as all', () => {
    expect(platformOrganizationStatusParam([])).toBe('')
    expect(platformOrganizationStatusParam(undefined)).toBe('')
  })

  it('drops empty selections rather than sending an empty term', () => {
    expect(platformOrganizationStatusParam(['active', ''])).toBe('active')
  })
})

describe('platformOrganizationRemainingQuota', () => {
  it('subtracts what was spent', () => {
    expect(
      platformOrganizationRemainingQuota(
        row({ quota: 500_000, used_quota: 120_000 })
      )
    ).toBe(380_000)
  })

  it('never reports a negative remainder', () => {
    expect(
      platformOrganizationRemainingQuota(
        row({ quota: 100_000, used_quota: 250_000 })
      )
    ).toBe(0)
  })
})

describe('platformOrganizationQuotaDelta', () => {
  it('is the difference against the grant when nothing was spent', () => {
    expect(
      platformOrganizationQuotaDelta(
        row({ quota: 500_000, used_quota: 0 }),
        700_000
      )
    ).toBe(200_000)
  })

  it('accounts for what was already spent', () => {
    // The grant is 1,000,000 and 400,000 of it is spent, leaving 600,000. Moving
    // the remainder to 900,000 means the grant has to rise by 300,000 — not by
    // the 900,000 asked for, which would overshoot by everything spent.
    expect(
      platformOrganizationQuotaDelta(
        row({ quota: 1_000_000, used_quota: 400_000 }),
        900_000
      )
    ).toBe(300_000)
  })

  it('goes negative when the quota is taken back', () => {
    expect(
      platformOrganizationQuotaDelta(
        row({ quota: 500_000, used_quota: 0 }),
        100_000
      )
    ).toBe(-400_000)
  })

  it('is zero when the remainder is left alone', () => {
    expect(
      platformOrganizationQuotaDelta(
        row({ quota: 500_000, used_quota: 200_000 }),
        300_000
      )
    ).toBe(0)
  })

  it('clamps a negative target at zero, which takes back the whole grant', () => {
    expect(
      platformOrganizationQuotaDelta(
        row({ quota: 500_000, used_quota: 0 }),
        -10
      )
    ).toBe(-500_000)
  })
})

describe('getPlatformOrganizationActionFlags', () => {
  it('lets an administrator edit, disable and adjust an active organization', () => {
    const flags = getPlatformOrganizationActionFlags(row(), { isRoot: false })

    expect(flags).toEqual({
      readOnly: false,
      canEdit: true,
      canDisable: true,
      canEnable: false,
      canAdjustQuota: true,
      canDissolve: false,
    })
  })

  it('offers enable instead of disable once the organization is disabled', () => {
    const flags = getPlatformOrganizationActionFlags(
      row({ status: 'disabled' }),
      { isRoot: false }
    )

    expect(flags.canDisable).toBe(false)
    expect(flags.canEnable).toBe(true)
    expect(flags.canEdit).toBe(true)
  })

  it('reserves dissolve for the platform root role', () => {
    expect(
      getPlatformOrganizationActionFlags(row(), { isRoot: true }).canDissolve
    ).toBe(true)
  })

  it('offers nothing at all on a dissolved organization, root included', () => {
    const flags = getPlatformOrganizationActionFlags(
      row({ status: 'dissolved' }),
      { isRoot: true }
    )

    expect(flags).toEqual({
      readOnly: true,
      canEdit: false,
      canDisable: false,
      canEnable: false,
      canAdjustQuota: false,
      canDissolve: false,
    })
  })
})

describe('platformOrganizationEditSchema', () => {
  it('requires a reason, unlike the organization center form', () => {
    const result = platformOrganizationEditSchema.safeParse({
      name: 'Acme',
      description: '',
      group: 'default',
      remain_quota: 0,
      reason: '   ',
    })

    expect(result.success).toBe(false)
  })

  it('requires a group, so an organization is never left without one', () => {
    const result = platformOrganizationEditSchema.safeParse({
      name: 'Acme',
      description: '',
      group: '  ',
      remain_quota: 0,
      reason: 'because',
    })

    expect(result.success).toBe(false)
  })

  it('accepts a whitespace-trimmed name and reason', () => {
    const result = platformOrganizationEditSchema.parse({
      name: '  Acme  ',
      description: '  ',
      group: 'default',
      remain_quota: '1000',
      reason: '  correcting the grant  ',
    })

    expect(result.name).toBe('Acme')
    expect(result.reason).toBe('correcting the grant')
    // The quota arrives as a string from the input and is coerced.
    expect(result.remain_quota).toBe(1000)
  })

  it('refuses a fractional or negative quota', () => {
    const base = {
      name: 'Acme',
      description: '',
      group: 'default',
      reason: 'because',
    }

    expect(
      platformOrganizationEditSchema.safeParse({ ...base, remain_quota: 1.5 })
        .success
    ).toBe(false)
    expect(
      platformOrganizationEditSchema.safeParse({ ...base, remain_quota: -1 })
        .success
    ).toBe(false)
  })
})

describe('transformPlatformOrganizationToFormDefaults', () => {
  it('seeds the form with the remaining quota, not the grant', () => {
    const values = transformPlatformOrganizationToFormDefaults(
      row({ quota: 500_000, used_quota: 120_000, group: '' })
    )

    expect(values.remain_quota).toBe(380_000)
    // An organization without a group is on the default one.
    expect(values.group).toBe('default')
    expect(values.reason).toBe('')
  })

  it('round-trips an untouched quota to no adjustment at all', () => {
    // Whatever the seed is, saving it back unchanged has to mean "nothing
    // changed" — otherwise opening the drawer would post an adjustment for an
    // edit nobody made.
    const organization = row({ quota: 123_457, used_quota: 456 })
    const values = transformPlatformOrganizationToFormDefaults(organization)
    const { quotaDelta } = transformPlatformOrganizationEditToRequests(
      values,
      organization
    )

    expect(quotaDelta).toBeNull()
  })
})

describe('transformPlatformOrganizationEditToRequests', () => {
  it('sends no adjustment when the quota was left alone', () => {
    const organization = row({ quota: 500_000, used_quota: 100_000 })
    const { edit, quotaDelta } = transformPlatformOrganizationEditToRequests(
      {
        name: ' Renamed ',
        description: ' why not ',
        group: 'vip',
        remain_quota: 400_000,
        reason: ' renaming ',
      },
      organization
    )

    expect(edit).toEqual({
      name: 'Renamed',
      description: 'why not',
      group: 'vip',
      reason: 'renaming',
    })
    expect(quotaDelta).toBeNull()
  })

  it('reports the signed adjustment when the quota moved', () => {
    const organization = row({ quota: 500_000, used_quota: 100_000 })
    const { quotaDelta } = transformPlatformOrganizationEditToRequests(
      {
        name: 'Acme',
        description: '',
        group: 'default',
        remain_quota: 700_000,
        reason: 'top-up',
      },
      organization
    )

    expect(quotaDelta).toBe(300_000)
  })
})
