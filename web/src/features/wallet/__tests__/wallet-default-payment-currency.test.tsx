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
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getOrganizations } from '@/features/organization/api'
import { getSelf, getStatus } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { calculatePaymentAsiaAmount, getTopupInfo } from '../api'
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
    calculatePaymentAsiaAmount: vi.fn(),
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

// PaymentAsia is the first configured method, so it is also the default one:
// the initial quote is produced by that gateway and is charged in HKD.
const topupInfo = {
  enable_online_topup: false,
  enable_stripe_topup: false,
  enable_paymentasia_topup: true,
  paymentasia_min_topup: 50,
  min_topup: 1,
  stripe_min_topup: 1,
  pay_methods: [
    {
      name: 'AlipayHK',
      type: 'paymentasia_alipayhk',
      currency: 'HKD',
      min_topup: 10,
    },
  ],
  amount_options: [],
  discount: {},
  creem_products: [],
  waffo_pay_methods: [],
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

describe('wallet default payment currency', () => {
  test('shows the default gateway currency on the initial quote', async () => {
    vi.mocked(calculatePaymentAsiaAmount).mockResolvedValue({
      success: true,
      data: '54',
    })

    render(
      <QueryClientProvider
        client={
          new QueryClient({ defaultOptions: { queries: { retry: false } } })
        }
      >
        <Wallet />
      </QueryClientProvider>
    )

    await waitFor(() => {
      expect(
        screen.getByText('Amount to pay:').parentElement
      ).toHaveTextContent('HK$54')
    })
  })
})
