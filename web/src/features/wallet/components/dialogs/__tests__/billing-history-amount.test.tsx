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
import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import { getUserBillingHistory } from '@/features/wallet/api'
import type { TopupRecord } from '@/features/wallet/types'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { BillingHistoryDialog } from '../billing-history-dialog'

vi.mock('@/features/wallet/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/features/wallet/api')>()
  return {
    ...actual,
    getUserBillingHistory: vi.fn(),
    getAllBillingHistory: vi.fn(),
  }
})

/** 一条 CNY 支付宝订单:充 1 元、实付 1 元(Amount 存的是元)。 */
const cnyOrder: TopupRecord = {
  id: 1,
  user_id: 1,
  amount: 1,
  money: 1,
  trade_no: 'ALIUSR1NO1758000000',
  payment_method: 'alipay',
  create_time: 1758000000,
  complete_time: 1758000060,
  status: 'success',
}

/** 记录卡片里的字段值:按字段名找到 Label,再取它相邻的值节点。 */
function fieldValue(label: string): string {
  const labelElement = screen.getByText(label)
  return labelElement.nextElementSibling?.textContent ?? ''
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
  vi.mocked(getUserBillingHistory).mockResolvedValue({
    success: true,
    data: { items: [cnyOrder], total: 1 },
  })
})

describe('billing history record amounts', () => {
  test('CNY 站点:1 元订单的金额与支付同为 1,不按汇率放大', async () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CNY',
          usdExchangeRate: 7.3,
        },
      },
    }))

    render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('ALIUSR1NO1758000000')).toBeInTheDocument()
    })

    expect(fieldValue('Amount')).toBe('¥1')
    expect(fieldValue('Payment')).toBe('1')
  })

  test('USD 站点:金额按美元存储并显示为 $,支付为实付金额', async () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'USD',
          usdExchangeRate: 7.3,
        },
      },
    }))

    render(<BillingHistoryDialog open onOpenChange={vi.fn()} />)

    await waitFor(() => {
      expect(screen.getByText('ALIUSR1NO1758000000')).toBeInTheDocument()
    })

    expect(fieldValue('Amount')).toBe('$1')
    expect(fieldValue('Payment')).toBe('1')
  })
})
