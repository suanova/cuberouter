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
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, test } from 'vitest'

import { useAccountContextStore } from '@/stores/account-context-store'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { OrganizationStatsCard } from '../components/organization-stats-card'
import type { WalletOrganization } from '../types'

const research: WalletOrganization = {
  id: 7,
  name: 'Research',
  quota: 5_000_000,
  member_quota_limit: 3_000_000,
  member_used_quota: 1_000_000,
}

const platform: WalletOrganization = {
  id: 9,
  name: 'Platform',
  quota: 1_000_000,
  member_quota_limit: 0,
  member_used_quota: 500_000,
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  useAccountContextStore.getState().accountContext.reset()
})

describe('wallet organization statistics', () => {
  test('renders nothing when the user belongs to no organization', () => {
    const { container } = render(<OrganizationStatsCard organizations={[]} />)

    expect(container).toBeEmptyDOMElement()
  })

  test("shows the signed-in member's quota limit and usage", () => {
    render(<OrganizationStatsCard organizations={[research]} />)

    expect(screen.getByText('Organization')).toBeInTheDocument()
    expect(screen.getByText('Organization Quota')).toBeInTheDocument()
    expect(screen.getByText('$6')).toBeInTheDocument()
    expect(screen.getByText('Used')).toBeInTheDocument()
    expect(screen.getByText('$2')).toBeInTheDocument()
    expect(screen.queryByText('$10')).not.toBeInTheDocument()
    expect(screen.queryByText('$4')).not.toBeInTheDocument()
    expect(
      screen.getByRole('combobox', { name: 'Select organization' })
    ).toBeInTheDocument()
  })

  test("falls back to the organization's total quota when the member has no limit", () => {
    render(<OrganizationStatsCard organizations={[platform]} />)

    expect(screen.getByText('$2')).toBeInTheDocument()
    expect(screen.getByText('$1')).toBeInTheDocument()
    expect(screen.queryByText('$1.50')).not.toBeInTheDocument()
  })

  test('defaults to the organization in the active account context', () => {
    useAccountContextStore.getState().accountContext.setCurrent({
      type: 'organization',
      id: platform.id,
      name: platform.name,
    })

    render(<OrganizationStatsCard organizations={[research, platform]} />)

    expect(screen.getByText('$2')).toBeInTheDocument()
    expect(screen.getByText('$1')).toBeInTheDocument()
    expect(screen.queryByText('$6')).not.toBeInTheDocument()
  })

  test('switching the organization updates both figures', async () => {
    render(<OrganizationStatsCard organizations={[research, platform]} />)

    expect(screen.getByText('$6')).toBeInTheDocument()

    await userEvent.click(
      screen.getByRole('combobox', { name: 'Select organization' })
    )
    await userEvent.click(
      await screen.findByRole('option', { name: 'Platform' })
    )

    expect(screen.getByText('$2')).toBeInTheDocument()
    expect(screen.getByText('$1')).toBeInTheDocument()
    expect(screen.queryByText('$6')).not.toBeInTheDocument()
  })
})
