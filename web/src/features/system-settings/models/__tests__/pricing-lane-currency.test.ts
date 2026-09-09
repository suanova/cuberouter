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
import { beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { laneLocalToUsdNumber, laneUsdToLocalNumber } from '../pricing-lane-currency'

// 注水方式与 model-pricing-core.test.ts 一致(store 真实类型,currency 不可为 null)。
function seedDisplayCurrency(type: CurrencyDisplayType, rate: number): void {
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
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

describe('lane 输入换算边界(CNY rate 7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  test('展示:美元状态 ×rate 归整为输入框数值', () => {
    expect(laneUsdToLocalNumber(2.5)).toBe(18.25)
    expect(laneUsdToLocalNumber(0.4)).toBe(2.92)
    expect(laneUsdToLocalNumber(0)).toBe(0)
  })

  test('录入解析 ÷rate 得回美元状态(整除汇率时精确)', () => {
    expect(laneLocalToUsdNumber(18.25)).toBe(2.5)
    expect(laneLocalToUsdNumber(2.92)).toBe(0.4)
    expect(laneLocalToUsdNumber(0)).toBe(0)
  })

  test('录入 → 展示往返稳定(含不整除输入,尾噪声被展示归整吸收)', () => {
    for (const local of [18.3, 18.31, 0.21, 0.03, 1.3, 12345.67, 100]) {
      expect(laneUsdToLocalNumber(laneLocalToUsdNumber(local))).toBe(local)
    }
  })

  test('录入解析不做位数截断,保证展示往返在任意汇率下精确', () => {
    // 12 位小数截断会让 18.3 ÷7.3 的结果再 ×7.3 偏离 2.5e-12,超出展示归整容差,
    // 输入框会显示 18.299999999996 —— 因此解析侧必须保留原始双精度。
    const usd = laneLocalToUsdNumber(18.3)
    expect(usd).toBe(18.3 / 7.3)
    expect(laneUsdToLocalNumber(usd)).toBe(18.3)
  })
})

describe('lane 输入换算边界(CUSTOM rate 0.85)', () => {
  beforeEach(() => seedDisplayCurrency('CUSTOM', 0.85))

  test('任意非 1 汇率同样展示 ×rate / 录入 ÷rate', () => {
    expect(laneUsdToLocalNumber(0.5)).toBe(0.425)
    expect(laneLocalToUsdNumber(0.425)).toBe(0.5)
    expect(laneUsdToLocalNumber(laneLocalToUsdNumber(1.03))).toBe(1.03)
  })
})

describe('lane 输入换算边界(USD/TOKENS rate=1 恒等)', () => {
  test('USD 模式下数值原样透传,不做格式化截断', () => {
    expect(laneUsdToLocalNumber(2.5)).toBe(2.5)
    expect(laneLocalToUsdNumber(2.5)).toBe(2.5)
    // 长小数不因展示格式化被截断(与旧行为一致)
    expect(laneUsdToLocalNumber(0.123456789012345)).toBe(0.123456789012345)
    expect(laneLocalToUsdNumber(0.123456789012345)).toBe(0.123456789012345)
  })

  test('TOKENS 展示模式按计费语义回落美元,同样恒等', () => {
    seedDisplayCurrency('TOKENS', 7.3)
    expect(laneUsdToLocalNumber(2.5)).toBe(2.5)
    expect(laneLocalToUsdNumber(2.5)).toBe(2.5)
  })
})
