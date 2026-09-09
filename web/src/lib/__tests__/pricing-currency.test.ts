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
import { beforeEach, describe, expect, it } from 'vitest'

import {
  getBillingCurrency,
  localToUsdNumber,
  usdToLocalNumber,
} from '@/lib/currency'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

// 注水以 system-config-store.ts 的真实类型为准(字段名与测试意图不变):
// - store 的 config 不可为 null,重置即把 currency 恢复为默认值;
// - CurrencyConfig 为完整对象(displayInCurrency/quotaPerUnit 等必填),
//   故在 DEFAULT_CURRENCY_CONFIG 之上覆盖被测字段;
// - quotaDisplayType 是 'USD' | 'CNY' | 'TOKENS' | 'CUSTOM' 字面量联合,
//   参数类型用 CurrencyDisplayType(各处调用均为合法字面量)。
function setDisplay(type: CurrencyDisplayType, rate: number, symbol = '¤'): void {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: {
        ...DEFAULT_CURRENCY_CONFIG,
        quotaDisplayType: type,
        usdExchangeRate: rate,
        customCurrencyExchangeRate: rate,
        customCurrencySymbol: symbol,
      },
    },
  }))
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

describe('getBillingCurrency', () => {
  it('CNY 用 usdExchangeRate 与 ¥', () => {
    setDisplay('CNY', 7.3)
    expect(getBillingCurrency()).toEqual({
      kind: 'currency',
      symbol: '¥',
      exchangeRate: 7.3,
    })
  })
  it('USD 回落 $ 且 rate=1', () => {
    setDisplay('USD', 99)
    expect(getBillingCurrency()).toEqual({
      kind: 'currency',
      symbol: '$',
      exchangeRate: 1,
    })
  })
  it('CUSTOM 用自定义符号与汇率', () => {
    setDisplay('CUSTOM', 2.5, '₩')
    expect(getBillingCurrency()).toEqual({
      kind: 'custom',
      symbol: '₩',
      exchangeRate: 2.5,
    })
  })
  it('TOKENS 回落美元语义($, rate=1)', () => {
    setDisplay('TOKENS', 7.3)
    expect(getBillingCurrency()).toEqual({
      kind: 'currency',
      symbol: '$',
      exchangeRate: 1,
    })
  })
})

describe('数值换算', () => {
  it('CNY:usd→local 乘 rate,local→usd 除 rate', () => {
    setDisplay('CNY', 7.3)
    expect(usdToLocalNumber(2.5)).toBeCloseTo(18.25, 6)
    expect(localToUsdNumber(18.25)).toBeCloseTo(2.5, 6)
  })
  it('USD:恒等', () => {
    setDisplay('USD', 1)
    expect(usdToLocalNumber(2.5)).toBe(2.5)
    expect(localToUsdNumber(2.5)).toBe(2.5)
  })
  it('非法/零汇率按 1 处理', () => {
    setDisplay('CUSTOM', 0)
    expect(localToUsdNumber(3)).toBe(3)
  })
})
