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

import type { OrganizationCapabilities } from '@/lib/account-context'

import type { OrganizationMemberRow } from '@/features/organization/types'

import {
  getPlatformOrganizationDetailActions,
  getPlatformOrganizationDetailPath,
  getPlatformOrganizationOwnerOptions,
  getPlatformOrganizationTabs,
  isPlatformOrganizationReadOnly,
  normalizePlatformOrganizationTabKey,
  platformOrganizationOwnerTransferSchema,
  transformPlatformOrganizationOwnerTransfer,
} from '../organization-platform-detail'

/**
 * The capability set the backend resolves for a platform administrator, minus
 * what each test turns off. Taken from `capabilitiesForPlatformRole` in
 * service/organization_policy.go.
 */
function capabilities(
  overrides: Partial<OrganizationCapabilities> = {}
): OrganizationCapabilities {
  return {
    can_view_organization: true,
    can_view_organization_wide_data: true,
    can_view_organization_usage: true,
    can_view_organization_tokens: true,
    can_view_organization_logs: true,
    can_update_organization: true,
    can_disable_organization: true,
    can_enable_organization: true,
    can_manage_members: true,
    can_transfer_owner: true,
    can_add_members_directly: true,
    can_exit_organization: false,
    can_view_invites: false,
    can_create_invites: false,
    can_revoke_invites: false,
    can_manage_all_tokens: true,
    can_modify_organization_group: true,
    can_dissolve_organization: true,
    can_view_audit: true,
    can_view_organization_billing_summary: true,
    show_return_organization_center: false,
    show_return_personal_center: false,
    ...overrides,
  }
}

function member(
  overrides: Partial<OrganizationMemberRow> = {}
): OrganizationMemberRow {
  return {
    id: 1,
    organization_id: 5,
    user_id: 11,
    role: 'member',
    status: 'active',
    user_status: 1,
    username: 'ada',
    display_name: 'Ada',
    email: 'ada@example.com',
    ...overrides,
  }
}

describe('normalizePlatformOrganizationTabKey', () => {
  it('keeps the keys the platform page defines', () => {
    for (const key of ['overview', 'members', 'tokens', 'owner-repair']) {
      expect(normalizePlatformOrganizationTabKey(key)).toBe(key)
    }
  })

  it('resolves the aliases older links used', () => {
    expect(normalizePlatformOrganizationTabKey('billing')).toBe('usage')
    expect(normalizePlatformOrganizationTabKey('audit')).toBe('audit-logs')
    expect(normalizePlatformOrganizationTabKey('owner')).toBe('owner-repair')
  })

  it('reads the last segment of a nested path', () => {
    expect(normalizePlatformOrganizationTabKey('/admin/organizations/5/tokens')).toBe(
      'tokens'
    )
  })

  it('falls back to the overview rather than 404-ing', () => {
    expect(normalizePlatformOrganizationTabKey('nonsense')).toBe('overview')
    expect(normalizePlatformOrganizationTabKey(undefined)).toBe('overview')
    expect(normalizePlatformOrganizationTabKey('')).toBe('overview')
  })

  it('does not adopt the organization center’s own section names', () => {
    // Invitations and settings are member business; the platform page has no
    // such tab, so naming one resolves to the overview rather than to a tab
    // that does not exist here.
    expect(normalizePlatformOrganizationTabKey('invitations')).toBe('overview')
    expect(normalizePlatformOrganizationTabKey('settings')).toBe('overview')
  })
})

describe('getPlatformOrganizationDetailPath', () => {
  it('builds the canonical path, defaulting to the overview', () => {
    expect(getPlatformOrganizationDetailPath(5)).toBe(
      '/admin/organizations/5/overview'
    )
    expect(getPlatformOrganizationDetailPath(5, 'members')).toBe(
      '/admin/organizations/5/members'
    )
  })

  it('normalizes whatever it is handed', () => {
    expect(getPlatformOrganizationDetailPath('5', 'billing')).toBe(
      '/admin/organizations/5/usage'
    )
    expect(getPlatformOrganizationDetailPath(5, 'nonsense')).toBe(
      '/admin/organizations/5/overview'
    )
  })
})

describe('isPlatformOrganizationReadOnly', () => {
  it('is read-only only once dissolved', () => {
    expect(isPlatformOrganizationReadOnly('dissolved')).toBe(true)
    expect(isPlatformOrganizationReadOnly('DISSOLVED')).toBe(true)
    expect(isPlatformOrganizationReadOnly('active')).toBe(false)
    // The organization center treats this as read-only — its members wait for
    // support — but an administrator is that support, so disabling must not
    // take away the button that undoes it.
    expect(isPlatformOrganizationReadOnly('disabled')).toBe(false)
    expect(isPlatformOrganizationReadOnly(undefined)).toBe(false)
  })
})

describe('getPlatformOrganizationDetailActions', () => {
  it('offers the writes the backend granted on an active organization', () => {
    const actions = getPlatformOrganizationDetailActions({
      capabilities: capabilities(),
      status: 'active',
    })

    expect(actions).toEqual({
      readOnly: false,
      canEdit: true,
      canDisable: true,
      canEnable: false,
      canAdjustQuota: true,
      canDissolve: true,
      canTransferOwner: true,
    })
  })

  it('offers enable, and not disable, on a disabled organization', () => {
    const actions = getPlatformOrganizationDetailActions({
      capabilities: capabilities(),
      status: 'disabled',
    })

    expect(actions.canEnable).toBe(true)
    expect(actions.canDisable).toBe(false)
    expect(actions.canEdit).toBe(true)
  })

  it('offers nothing but reading once dissolved', () => {
    const actions = getPlatformOrganizationDetailActions({
      capabilities: capabilities(),
      status: 'dissolved',
    })

    expect(actions.readOnly).toBe(true)
    expect(actions.canEdit).toBe(false)
    expect(actions.canEnable).toBe(false)
    expect(actions.canDisable).toBe(false)
    expect(actions.canAdjustQuota).toBe(false)
    expect(actions.canDissolve).toBe(false)
    expect(actions.canTransferOwner).toBe(false)
  })

  it('follows the capability set rather than the role', () => {
    // An ordinary platform administrator holds no dissolve or transfer
    // capability; the page must not offer what the backend would refuse.
    const actions = getPlatformOrganizationDetailActions({
      capabilities: capabilities({
        can_dissolve_organization: false,
        can_transfer_owner: false,
        can_update_organization: false,
      }),
      status: 'active',
    })

    expect(actions.canEdit).toBe(false)
    expect(actions.canAdjustQuota).toBe(false)
    expect(actions.canDissolve).toBe(false)
    expect(actions.canTransferOwner).toBe(false)
    // Still able to switch the organization off, which the capability grants.
    expect(actions.canDisable).toBe(true)
  })
})

describe('getPlatformOrganizationTabs', () => {
  it('lists every section a granted capability can read, in order', () => {
    const tabs = getPlatformOrganizationTabs({
      capabilities: capabilities(),
      status: 'active',
    })

    expect(tabs.map((tab) => tab.key)).toEqual([
      'overview',
      'members',
      'tokens',
      'logs',
      'tasks',
      'usage',
      'audit-logs',
      'owner-repair',
    ])
    expect(tabs.every((tab) => !tab.readOnly)).toBe(true)
    expect(tabs[7].labelKey).toBe('Transfer Ownership')
  })

  it('drops the sections the caller may not read', () => {
    const tabs = getPlatformOrganizationTabs({
      capabilities: capabilities({
        can_view_organization_tokens: false,
        can_view_organization_logs: false,
        can_view_audit: false,
        can_view_organization_usage: false,
      }),
      status: 'active',
    })

    // Which reads are granted says nothing about owner repair — that tab is
    // gated on the transfer capability alone.
    expect(tabs.map((tab) => tab.key)).toEqual([
      'overview',
      'members',
      'owner-repair',
    ])
  })

  it('answers the members tab for a caller who may only read wide data', () => {
    const tabs = getPlatformOrganizationTabs({
      capabilities: capabilities({
        can_manage_members: false,
        can_view_organization_wide_data: true,
      }),
      status: 'active',
    })

    expect(tabs.map((tab) => tab.key)).toContain('members')
  })

  it('offers no owner repair once dissolved, and marks the rest read-only', () => {
    const tabs = getPlatformOrganizationTabs({
      capabilities: capabilities(),
      status: 'dissolved',
    })

    expect(tabs.map((tab) => tab.key)).not.toContain('owner-repair')
    expect(tabs.every((tab) => tab.readOnly)).toBe(true)
  })

  it('offers owner repair while the organization is disabled', () => {
    // An owner who is gone is just as gone while the organization's traffic is
    // switched off, and that is when the repair is most often needed.
    const tabs = getPlatformOrganizationTabs({
      capabilities: capabilities(),
      status: 'disabled',
    })

    expect(tabs.map((tab) => tab.key)).toContain('owner-repair')
  })

  it('offers no owner repair without the capability', () => {
    const tabs = getPlatformOrganizationTabs({
      capabilities: capabilities({ can_transfer_owner: false }),
      status: 'active',
    })

    expect(tabs.map((tab) => tab.key)).not.toContain('owner-repair')
  })
})

describe('platformOrganizationOwnerTransferSchema', () => {
  it('requires a chosen member and a reason', () => {
    expect(platformOrganizationOwnerTransferSchema.safeParse({}).success).toBe(
      false
    )
    expect(
      platformOrganizationOwnerTransferSchema.safeParse({
        owner_user_id: 0,
        reason: 'Owner left the company',
      }).success
    ).toBe(false)
    expect(
      platformOrganizationOwnerTransferSchema.safeParse({
        owner_user_id: 11,
        reason: '   ',
      }).success
    ).toBe(false)
  })

  it('accepts a member id that arrives as a string', () => {
    const parsed = platformOrganizationOwnerTransferSchema.parse({
      owner_user_id: '11',
      reason: '  Owner left the company  ',
    })

    expect(transformPlatformOrganizationOwnerTransfer(parsed)).toEqual({
      owner_user_id: 11,
      reason: 'Owner left the company',
    })
  })
})

describe('getPlatformOrganizationOwnerOptions', () => {
  it('offers every active member, not only the administrators', () => {
    // The backend's owner repair locks the target as an active membership and
    // requires nothing else, so narrowing this to owners and administrators —
    // as the member-side transfer picker does — would hide valid targets.
    const options = getPlatformOrganizationOwnerOptions(
      [
        member({ user_id: 11, role: 'member' }),
        member({ user_id: 12, role: 'admin' }),
        member({ user_id: 13, role: 'owner' }),
      ],
      0
    )

    expect(options.map((option) => option.value)).toEqual([11, 12, 13])
  })

  it('leaves out the current owner', () => {
    const options = getPlatformOrganizationOwnerOptions(
      [member({ user_id: 11 }), member({ user_id: 12 })],
      11
    )

    expect(options.map((option) => option.value)).toEqual([12])
  })

  it('leaves out members who could not sign in to act as owner', () => {
    const options = getPlatformOrganizationOwnerOptions(
      [
        member({ user_id: 11, status: 'disabled' }),
        member({ user_id: 12, user_status: 2 }),
        member({ user_id: 13, status: 'exited' }),
        member({ user_id: 14 }),
      ],
      0
    )

    expect(options.map((option) => option.value)).toEqual([14])
  })

  it('names a member by whatever the roster holds', () => {
    const options = getPlatformOrganizationOwnerOptions(
      [
        member({ user_id: 11, display_name: 'Ada', username: 'ada' }),
        member({ user_id: 12, display_name: '', username: 'grace' }),
        member({
          user_id: 13,
          display_name: '',
          username: '',
          email: 'linus@example.com',
        }),
        member({ user_id: 14, display_name: '', username: '', email: '' }),
      ],
      0
    )

    expect(options.map((option) => option.label)).toEqual([
      'Ada',
      'grace',
      'linus@example.com',
      '14',
    ])
  })

  it('survives an empty roster', () => {
    expect(getPlatformOrganizationOwnerOptions()).toEqual([])
  })
})
