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
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getSelf, getStatus } from '@/lib/api'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { calculateAlipayAmount, getAffiliateCode, getTopupInfo } from '../api'
import { Wallet } from '../index'

// 网络边界按真实响应形状给出:站点状态(展示货币 + 汇率)、用户、充值配置与
// 官方支付宝的支付金额。其余(订阅等)在本用例范围外,交由真实请求失败降级。
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
    calculateAlipayAmount: vi.fn(),
  }
})

/** 站点展示货币与美元汇率,对应 /api/status 的返回。 */
function status(quotaDisplayType: 'USD' | 'CNY', usdExchangeRate: number) {
  return {
    quota_display_type: quotaDisplayType,
    usd_exchange_rate: usdExchangeRate,
    quota_per_unit: 500000,
    display_in_currency: true,
    system_name: 'Test',
    logo: '',
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
  enable_alipay_topup: true,
  alipay_min_topup: 1,
  min_topup: 1,
  stripe_min_topup: 1,
  pay_methods: [{ name: 'Alipay', type: 'alipay_official' }],
  amount_options: [],
  discount: {},
  creem_products: [],
  waffo_pay_methods: [],
}

/** 打开确认弹窗:钱包页按最低充值金额初始化,点支付方式即进入确认。 */
async function openConfirmDialog(): Promise<HTMLElement> {
  render(
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <Wallet />
    </QueryClientProvider>
  )

  const alipay = await screen.findByRole('button', { name: 'Alipay' })
  fireEvent.click(alipay)

  return screen.findByRole('alertdialog')
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  vi.mocked(getSelf).mockResolvedValue({ success: true, data: user })
  vi.mocked(getTopupInfo).mockResolvedValue({ success: true, data: topupInfo })
  vi.mocked(getAffiliateCode).mockResolvedValue({ success: true, data: 'AFF1' })
})

describe('wallet confirm dialog amounts', () => {
  test('CNY 站点:充值 1 元时,充值金额与您支付同为 1(不按汇率放大)', async () => {
    vi.mocked(getStatus).mockResolvedValue(status('CNY', 7.3))
    // CNY 模式支付金额即元数:充 1 元 → 实付 1 元
    vi.mocked(calculateAlipayAmount).mockResolvedValue({
      success: true,
      data: '1',
    })

    const dialog = await openConfirmDialog()

    await waitFor(() => {
      expect(within(dialog).getByText('¥1')).toBeInTheDocument()
    })
    expect(within(dialog).getByText('Topup Amount')).toBeInTheDocument()
    expect(within(dialog).queryByText('¥7.3')).not.toBeInTheDocument()

    const youPay = within(dialog).getByText('You Pay').parentElement
    expect(youPay).toHaveTextContent('1')
  })

  test('USD 站点:充值金额按展示货币($)显示,您支付为按汇率换算的本地货币', async () => {
    vi.mocked(getStatus).mockResolvedValue(status('USD', 7.3))
    // USD 模式:充 1 美元 → 实付 7.3 元
    vi.mocked(calculateAlipayAmount).mockResolvedValue({
      success: true,
      data: '7.3',
    })

    const dialog = await openConfirmDialog()

    await waitFor(() => {
      expect(within(dialog).getByText('$1')).toBeInTheDocument()
    })

    const youPay = within(dialog).getByText('You Pay').parentElement
    expect(youPay).toHaveTextContent('7.3')
  })
})
