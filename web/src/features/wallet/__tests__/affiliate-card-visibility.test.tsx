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

import { getSelf, getStatus } from '@/lib/api'
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

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    getTopupInfo: vi.fn(),
    getAffiliateCode: vi.fn(),
  }
})

/** /api/status 返回的形状:站点状态里携带推荐计划开关。 */
function status(affiliateEnabled: boolean) {
  return {
    quota_display_type: 'CNY' as const,
    usd_exchange_rate: 7.3,
    quota_per_unit: 500000,
    display_in_currency: true,
    system_name: 'Test',
    logo: '',
    affiliate_enabled: affiliateEnabled,
  }
}

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
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  vi.mocked(getSelf).mockResolvedValue({ success: true, data: user })
  vi.mocked(getTopupInfo).mockResolvedValue({ success: true, data: topupInfo })
  vi.mocked(getAffiliateCode).mockResolvedValue({ success: true, data: 'AFF1' })
})

describe('wallet referral program card', () => {
  test('站点未配置邀请奖励时隐藏推荐计划卡片', async () => {
    vi.mocked(getStatus).mockResolvedValue(status(false))

    renderWallet()

    // 站点货币为 CNY:金额渲染出 ¥ 说明 status 已生效,此时再断言卡片不存在
    await screen.findAllByText(/¥/)

    expect(screen.queryByText('Referral Program')).not.toBeInTheDocument()
    expect(getAffiliateCode).not.toHaveBeenCalled()
  })

  test('站点配置了邀请奖励时展示卡片与邀请链接', async () => {
    vi.mocked(getStatus).mockResolvedValue(status(true))

    renderWallet()

    expect(await screen.findByText('Referral Program')).toBeInTheDocument()
    expect(await screen.findByDisplayValue(/\/sign-up\?aff=AFF1$/)).toBeInTheDocument()
    expect(getAffiliateCode).toHaveBeenCalledTimes(1)
  })
})
