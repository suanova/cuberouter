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
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getOrganizations } from '@/features/organization/api'
import type { UserOrganization } from '@/features/organization/types'
import { getSelf, getStatus } from '@/lib/api'
import { useAccountContextStore } from '@/stores/account-context-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { getAffiliateCode, getTopupInfo } from '../api'
import { Wallet } from '../index'

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return {
    ...actual,
    getStatus: vi.fn(),
    getSelf: vi.fn(),
  }
})

vi.mock('@/features/organization/api', async (importOriginal) => {
  const actual =
    await importOriginal<typeof import('@/features/organization/api')>()
  return {
    ...actual,
    getOrganizations: vi.fn(),
  }
})

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    getTopupInfo: vi.fn(),
    getAffiliateCode: vi.fn(),
  }
})

const user = {
  id: 1,
  username: 'alice',
  quota: 0,
  used_quota: 0,
  request_count: 0,
  aff_quota: 0,
  aff_history_quota: 0,
  aff_count: 0,
  group: 'default',
}

const status = {
  quota_display_type: 'USD' as const,
  usd_exchange_rate: 1,
  quota_per_unit: 500000,
  display_in_currency: true,
  system_name: 'Test',
  logo: '',
  affiliate_enabled: false,
}

const topupInfo = {
  enable_online_topup: false,
  enable_stripe_topup: false,
  min_topup: 1,
  stripe_min_topup: 1,
  pay_methods: [],
  amount_options: [],
  discount: {},
  creem_products: [],
  waffo_pay_methods: [],
}

function organization(
  overrides: Partial<UserOrganization> = {}
): UserOrganization {
  return {
    id: 7,
    name: 'Research',
    slug: 'research',
    status: 'active',
    group: 'default',
    quota: 5_000_000,
    used_quota: 2_000_000,
    member_quota_limit: 3_000_000,
    member_used_quota: 1_000_000,
    request_count: 0,
    owner_user_id: 1,
    created_by: 1,
    created_at: 1_700_000_000,
    updated_at: 1_700_000_000,
    role: 'member',
    disable_state: {
      active_sources: [],
      effective_source: '',
      can_self_enable: false,
    },
    can_self_enable: false,
    capabilities: {
      can_view_organization: true,
      can_view_organization_wide_data: false,
      can_view_organization_usage: true,
      can_view_organization_tokens: true,
      can_view_organization_logs: true,
      can_update_organization: false,
      can_disable_organization: false,
      can_enable_organization: false,
      can_manage_members: false,
      can_manage_member_discounts: false,
      can_manage_member_models: false,
      can_manage_member_quotas: false,
      can_transfer_owner: false,
      can_add_members_directly: false,
      can_exit_organization: true,
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
    },
    access_mode: 'workspace',
    ...overrides,
  }
}

function renderWallet() {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <Wallet />
    </QueryClientProvider>
  )
}

beforeEach(() => {
  window.localStorage.clear()
  useAccountContextStore.getState().accountContext.reset()
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  vi.mocked(getSelf).mockResolvedValue({ success: true, data: user })
  vi.mocked(getStatus).mockResolvedValue(status)
  vi.mocked(getTopupInfo).mockResolvedValue({ success: true, data: topupInfo })
  vi.mocked(getAffiliateCode).mockResolvedValue({ success: true, data: 'AFF1' })
  vi.mocked(getOrganizations).mockResolvedValue({
    success: true,
    data: [],
  })
})

describe('wallet organization card', () => {
  test('shows the organization figures when the user belongs to one', async () => {
    vi.mocked(getOrganizations).mockResolvedValue({
      success: true,
      data: [organization()],
    })

    renderWallet()

    expect(await screen.findByText('Organization')).toBeInTheDocument()
    expect(screen.getByText('Organization Quota')).toBeInTheDocument()
    expect(screen.getByText('$6')).toBeInTheDocument()
    expect(screen.getByText('Used')).toBeInTheDocument()
    expect(screen.getByText('$2')).toBeInTheDocument()
  })

  test('hides the organization card when the user belongs to none', async () => {
    renderWallet()

    // Waits for the personal stats so the organization query has also settled.
    expect(await screen.findByText('Personal Balance')).toBeInTheDocument()

    expect(screen.queryByText('Organization')).not.toBeInTheDocument()
    expect(screen.queryByText('Organization Quota')).not.toBeInTheDocument()
  })
})
