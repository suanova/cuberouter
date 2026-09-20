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

import type { OrganizationCapabilities } from '@/lib/account-context'

import type { OrganizationMember } from '../../types'
import {
  buildOrganizationMemberRoleUpdatePayload,
  getOrganizationDetailPath,
  getOrganizationListActionFlags,
  getOrganizationMemberActionFlags,
  getOrganizationReadOnlyState,
  getOrganizationTabs,
  getOrganizationTokenBatchDeletePlan,
  getOrganizationTransferMemberOptions,
  isOrganizationMemberDemotion,
  normalizeOrganizationTabKey,
} from '../organization-center'

/** All-false base: each test opts into exactly the capabilities it exercises. */
function capabilities(
  overrides: Partial<OrganizationCapabilities> = {}
): OrganizationCapabilities {
  return {
    can_view_organization: false,
    can_view_organization_wide_data: false,
    can_view_organization_usage: false,
    can_view_members_limited: false,
    can_view_organization_tokens: false,
    can_view_organization_logs: false,
    can_update_organization: false,
    can_disable_organization: false,
    can_enable_organization: false,
    can_manage_members: false,
    can_transfer_owner: false,
    can_add_members_directly: false,
    can_exit_organization: false,
    can_view_invites: false,
    can_create_invites: false,
    can_revoke_invites: false,
    can_manage_all_tokens: false,
    can_modify_organization_group: false,
    can_dissolve_organization: false,
    can_view_audit: false,
    can_view_organization_billing_summary: false,
    show_return_organization_center: false,
    show_return_personal_center: false,
    ...overrides,
  }
}

/** An owner of an active organization: every capability the backend grants. */
function ownerCapabilities(
  overrides: Partial<OrganizationCapabilities> = {}
): OrganizationCapabilities {
  return capabilities({
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
    can_exit_organization: true,
    can_view_invites: true,
    can_create_invites: true,
    can_revoke_invites: true,
    can_manage_all_tokens: true,
    can_modify_organization_group: true,
    can_dissolve_organization: true,
    can_view_audit: true,
    can_view_organization_billing_summary: true,
    show_return_organization_center: true,
    show_return_personal_center: true,
    ...overrides,
  })
}

describe('normalizeOrganizationTabKey', () => {
  test('maps the legacy section names onto their canonical tabs', () => {
    // Old bookmarks and links must keep working.
    expect(normalizeOrganizationTabKey('invites')).toBe('invitations')
    expect(normalizeOrganizationTabKey('billing')).toBe('usage')
    expect(normalizeOrganizationTabKey('audit')).toBe('audit-logs')
  })

  test('takes the last segment of a path', () => {
    expect(normalizeOrganizationTabKey('/organizations/12/members')).toBe(
      'members'
    )
    expect(normalizeOrganizationTabKey('organizations/12/audit')).toBe(
      'audit-logs'
    )
  })

  test('falls back to the overview for anything unrecognized', () => {
    // The section comes from the URL, so an unknown value must not 404.
    expect(normalizeOrganizationTabKey('nope')).toBe('overview')
    expect(normalizeOrganizationTabKey(undefined)).toBe('overview')
    expect(normalizeOrganizationTabKey('')).toBe('overview')
  })
})

describe('getOrganizationDetailPath', () => {
  test('builds a detail path and normalizes the section', () => {
    expect(getOrganizationDetailPath(12)).toBe('/organizations/12/overview')
    expect(getOrganizationDetailPath(12, 'billing')).toBe(
      '/organizations/12/usage'
    )
  })
})

describe('getOrganizationReadOnlyState', () => {
  test('a dissolved organization can never be written to', () => {
    expect(
      getOrganizationReadOnlyState({
        capabilities: ownerCapabilities(),
        status: 'dissolved',
      })
    ).toEqual({ readOnly: true, reason: 'dissolved' })
  })

  test('a disabled organization is read-only and says so', () => {
    // Disabled and dissolved are distinguished because the recovery differs.
    expect(
      getOrganizationReadOnlyState({
        capabilities: ownerCapabilities(),
        status: 'disabled',
      })
    ).toEqual({ readOnly: true, reason: 'disabled' })
  })

  test('the read_only access mode makes the organization read-only', () => {
    expect(
      getOrganizationReadOnlyState({
        capabilities: ownerCapabilities(),
        status: 'active',
        accessMode: 'read_only',
      })
    ).toEqual({ readOnly: true, reason: 'read_only' })
  })

  test('status is matched case-insensitively', () => {
    expect(
      getOrganizationReadOnlyState({
        capabilities: ownerCapabilities(),
        status: 'Disabled',
      }).readOnly
    ).toBe(true)
  })

  test('an active organization with write access is writable', () => {
    expect(
      getOrganizationReadOnlyState({
        capabilities: ownerCapabilities(),
        status: 'active',
        accessMode: 'workspace',
      })
    ).toEqual({ readOnly: false, reason: null })
  })
})

describe('getOrganizationTabs', () => {
  test('an owner sees every section', () => {
    const keys = getOrganizationTabs({
      capabilities: ownerCapabilities(),
      status: 'active',
    }).map((tab) => tab.key)

    expect(keys).toEqual([
      'overview',
      'members',
      'invitations',
      'tokens',
      'logs',
      'usage',
      'tasks',
      'audit-logs',
      'settings',
    ])
  })

  test('a member without capabilities sees only the overview', () => {
    // Nothing is shown disabled: the backend would reject the data requests.
    expect(
      getOrganizationTabs({
        capabilities: capabilities({ can_view_organization: true }),
        status: 'active',
      }).map((tab) => tab.key)
    ).toEqual(['overview'])
  })

  test('a limited viewer gets the members tab but flagged as narrowed', () => {
    const tabs = getOrganizationTabs({
      capabilities: capabilities({
        can_view_organization: true,
        can_view_members_limited: true,
      }),
      status: 'active',
    })
    const members = tabs.find((tab) => tab.key === 'members')

    expect(members?.limitedView).toBe(true)
    expect(tabs.some((tab) => tab.key === 'settings')).toBe(false)
  })

  test('wide-data access removes the narrowed flag from the log sections', () => {
    const tabs = getOrganizationTabs({
      capabilities: capabilities({
        can_view_organization: true,
        can_view_organization_wide_data: true,
        can_view_organization_logs: true,
        can_view_organization_usage: true,
      }),
      status: 'active',
    })
    for (const key of ['logs', 'usage', 'tasks']) {
      expect(tabs.find((tab) => tab.key === key)?.limitedView).toBe(false)
    }
  })

  test('a read-only organization loses its settings section', () => {
    const tabs = getOrganizationTabs({
      capabilities: ownerCapabilities(),
      status: 'disabled',
    })

    expect(tabs.some((tab) => tab.key === 'settings')).toBe(false)
    expect(tabs.every((tab) => tab.readOnly)).toBe(true)
  })

  test('a partial administrator can still reach settings to hand over', () => {
    const tabs = getOrganizationTabs({
      capabilities: capabilities({
        can_view_organization: true,
        can_transfer_owner: true,
        can_manage_members: true,
      }),
      status: 'active',
    })

    expect(tabs.some((tab) => tab.key === 'settings')).toBe(true)
  })

  test('an organization can be reached by a platform administrator alone', () => {
    // access_mode 'admin' means no membership at all; the sections must still
    // render for whoever the backend granted capabilities to.
    const tabs = getOrganizationTabs({
      capabilities: capabilities({
        can_view_organization: true,
        can_view_organization_logs: true,
        can_view_audit: true,
        can_dissolve_organization: true,
      }),
      status: 'active',
      accessMode: 'admin',
    })

    // `tasks` rides on the log capability, not on usage.
    expect(tabs.map((tab) => tab.key)).toEqual([
      'overview',
      'logs',
      'tasks',
      'audit-logs',
      'settings',
    ])
  })
})

describe('getOrganizationListActionFlags', () => {
  test('row actions follow the capability set', () => {
    expect(
      getOrganizationListActionFlags({
        capabilities: capabilities({
          can_view_organization: true,
          can_disable_organization: true,
        }),
      })
    ).toEqual({
      enter: true,
      edit: false,
      disable: true,
      enable: false,
      dissolve: false,
    })
  })
})

describe('member actions', () => {
  test('an owner may manage admins and members but never the owner row', () => {
    const granted = { canManageMembers: true }
    expect(
      getOrganizationMemberActionFlags({
        ...granted,
        actorRole: 'owner',
        targetRole: 'admin',
      }).canChangeRole
    ).toBe(true)
    expect(
      getOrganizationMemberActionFlags({
        ...granted,
        actorRole: 'owner',
        targetRole: 'owner',
      }).canChangeRole
    ).toBe(false)
  })

  test('an admin may not manage another admin', () => {
    const flags = getOrganizationMemberActionFlags({
      canManageMembers: true,
      actorRole: 'admin',
      targetRole: 'admin',
    })
    expect(flags).toEqual({
      canChangeRole: false,
      canChangeStatus: false,
      canRemove: false,
    })
  })

  test('the caller cannot act on their own row, even as owner', () => {
    // Demoting or removing yourself is a distinct, deliberately narrowed flow.
    expect(
      getOrganizationMemberActionFlags({
        canManageMembers: true,
        actorRole: 'owner',
        targetRole: 'member',
        isCurrentUser: true,
      }).canRemove
    ).toBe(false)
  })

  test('a read-only organization blocks every member action', () => {
    expect(
      getOrganizationMemberActionFlags({
        canManageMembers: true,
        actorRole: 'owner',
        targetRole: 'member',
        readOnly: true,
      }).canChangeRole
    ).toBe(false)
  })

  test('roles are compared case-insensitively', () => {
    expect(
      getOrganizationMemberActionFlags({
        canManageMembers: true,
        actorRole: 'Owner',
        targetRole: 'Member',
      }).canRemove
    ).toBe(true)
  })
})

describe('role changes', () => {
  test('demoting an admin to member requires a hand-over target', () => {
    expect(isOrganizationMemberDemotion({ role: 'admin' }, 'member')).toBe(true)
    expect(isOrganizationMemberDemotion({ role: 'member' }, 'member')).toBe(false)
    expect(isOrganizationMemberDemotion({ role: 'admin' }, 'admin')).toBe(false)

    expect(
      buildOrganizationMemberRoleUpdatePayload({ role: 'admin' }, 'member', {
        transferToUserId: 42,
        reason: '  stepping down  ',
      })
    ).toEqual({ role: 'member', transfer_to_user_id: 42, reason: 'stepping down' })
  })

  test('a plain role change carries no transfer or reason fields', () => {
    // Sending them for a normal promotion would confuse the audit entry.
    expect(
      buildOrganizationMemberRoleUpdatePayload({ role: 'member' }, 'admin', {
        transferToUserId: 42,
        reason: 'promotion',
      })
    ).toEqual({ role: 'admin' })
  })
})

describe('getOrganizationTransferMemberOptions', () => {
  const members: Array<Partial<OrganizationMember> & { user_status?: number; username?: string }> = [
    {
      user_id: 1,
      role: 'owner',
      status: 'active',
      user_status: 1,
      username: 'a',
    },
    {
      user_id: 2,
      role: 'admin',
      status: 'active',
      user_status: 1,
      username: 'b',
    },
    {
      user_id: 3,
      role: 'member',
      status: 'active',
      user_status: 1,
      username: 'c',
    },
    {
      user_id: 4,
      role: 'admin',
      status: 'exited',
      user_status: 1,
      username: 'd',
    },
    {
      user_id: 5,
      role: 'admin',
      status: 'active',
      user_status: 2,
      username: 'e',
    },
  ]

  test('offers only active owners and admins', () => {
    // A member cannot own the organization, and a disabled account cannot sign in.
    expect(
      getOrganizationTransferMemberOptions(members, 1).map((o) => o.value)
    ).toEqual([2])
  })

  test('excludes the outgoing owner', () => {
    expect(
      getOrganizationTransferMemberOptions(members, 1).some((o) => o.value === 1)
    ).toBe(false)
  })

  test('falls back through username, display name and email for the label', () => {
    expect(
      getOrganizationTransferMemberOptions(
        [
          { user_id: 9, role: 'admin', status: 'active', user_status: 1, email: 'z@b.c' },
          {
            user_id: 10,
            role: 'admin',
            status: 'active',
            user_status: 1,
            display_name: 'Ten',
          },
        ],
        1
      )
    ).toEqual([
      { label: 'z@b.c', value: 9 },
      { label: 'Ten', value: 10 },
    ])
  })

  test('an empty member list yields no options', () => {
    expect(getOrganizationTransferMemberOptions()).toEqual([])
  })
})

describe('getOrganizationTokenBatchDeletePlan', () => {
  const tokens = [{ id: 1 }, { id: 2 }, { id: 3 }]

  test('withholds the whole request when any key is out of scope', () => {
    // Batch deletion is all-or-nothing, so the UI asks the user to deselect.
    const plan = getOrganizationTokenBatchDeletePlan(
      tokens,
      (token) => token.id !== 2
    )
    expect(plan).toEqual({
      selectedCount: 3,
      unauthorizedCount: 1,
      request: null,
    })
  })

  test('builds the request when every key is authorized', () => {
    expect(getOrganizationTokenBatchDeletePlan(tokens, () => true)).toEqual({
      selectedCount: 3,
      unauthorizedCount: 0,
      request: { ids: [1, 2, 3] },
    })
  })

  test('an empty selection produces no request', () => {
    expect(getOrganizationTokenBatchDeletePlan([], () => true).request).toBeNull()
  })
})
