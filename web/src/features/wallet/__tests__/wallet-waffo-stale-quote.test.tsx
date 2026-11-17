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
import { act, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getOrganizations } from '@/features/organization/api'
import { getSelf, getStatus } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { calculateWaffoAmount, getTopupInfo } from '../api'
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
    calculateWaffoAmount: vi.fn(),
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

// Waffo is the only gateway, so it is the default and its button opens the
// confirmation flow directly.
const topupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: false,
  min_topup: 1,
  stripe_min_topup: 1,
  pay_methods: [],
  amount_options: [],
  discount: {},
  enable_waffo_topup: true,
  waffo_pay_methods: [
    { name: 'Waffo Card', payMethodType: 'card', payMethodName: 'Card' },
  ],
  waffo_min_topup: 1,
  creem_products: [],
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  vi.mocked(getStatus).mockResolvedValue({
    quota_display_type: 'USD',
    usd_exchange_rate: 1,
    quota_per_unit: 500000,
    display_in_currency: true,
    system_name: 'Test',
    logo: '',
  })
  vi.mocked(getSelf).mockResolvedValue({ success: true, data: user })
  vi.mocked(getTopupInfo).mockResolvedValue({ success: true, data: topupInfo })
  vi.mocked(getOrganizations).mockResolvedValue({ success: true, data: [] })
})

describe('wallet Waffo confirm dialog', () => {
  // Selecting a Waffo method awaits its quote before opening the dialog. If
  // the user changes the amount meanwhile, that newer quote wins and the
  // superseded Waffo request must not open the dialog with a stale amount.
  test('stays closed when the Waffo quote was superseded by a newer one', async () => {
    const resolvers: Array<
      (response: { success: boolean; data?: string }) => void
    > = []
    vi.mocked(calculateWaffoAmount).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvers.push(resolve)
        })
    )

    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <Wallet />
      </QueryClientProvider>
    )

    const waffoButton = await screen.findByRole('button', {
      name: 'Waffo Card',
    })
    // The initial quote for the default (Waffo) gateway starts on mount.
    await waitFor(() => expect(resolvers).toHaveLength(1))

    fireEvent.click(waffoButton)
    await waitFor(() => expect(resolvers).toHaveLength(2))

    fireEvent.change(screen.getByLabelText('Custom Amount'), {
      target: { value: '2' },
    })
    await waitFor(() => expect(resolvers).toHaveLength(3))

    // The newer amount-change quote lands first, then the clicked method's
    // quote arrives late and must be discarded.
    await act(async () => {
      resolvers[2]({ success: true, data: '2' })
    })
    await act(async () => {
      resolvers[1]({ success: true, data: '1.9' })
    })

    await waitFor(() => expect(waffoButton).not.toBeDisabled())
    expect(screen.queryByRole('alertdialog')).not.toBeInTheDocument()
  })
})
