/*
Copyright (C) 2023-2026 QuantumNous

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
import { act, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, test } from 'vitest'

import type { BillingUsageSchema } from '@/features/pricing/types'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { DynamicPricingBreakdown } from '../dynamic-pricing-breakdown'

function setDisplay(type: CurrencyDisplayType, rate: number) {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: type,
        usdExchangeRate: rate,
        customCurrencyExchangeRate: rate,
      },
    },
  }))
}

beforeEach(() => {
  setDisplay('USD', 1)
})

const usageSchema: BillingUsageSchema = {
  seconds: { type: 'number', unit: 'second' },
}

describe('DynamicPricingBreakdown currency reactivity', () => {
  test('挂载中展示货币从 CNY 切到 USD 后,单价金额符号与换算随之刷新', () => {
    setDisplay('CNY', 7.3)
    render(
      <DynamicPricingBreakdown
        billingExpr='tier("base", u("seconds") * 0.4)'
        usageSchema={usageSchema}
      />
    )

    // 0.4 USD/s × 7.3 = ¥2.92(展示为 4 位小数)
    expect(
      screen.getAllByText((content) => content.includes('¥2.9200')).length
    ).toBeGreaterThan(0)

    act(() => setDisplay('USD', 1))
    expect(
      screen.getAllByText((content) => content.includes('$0.4000')).length
    ).toBeGreaterThan(0)
    expect(
      screen.queryAllByText((content) => content.includes('¥2.9200')).length
    ).toBe(0)
  })
})
