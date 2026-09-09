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

import {
  formatOffPeakHour,
  formatVideoPrice,
  formatVideoPriceMoney,
  getOffPeakWindowLabel,
} from '../video-price'

// 注水 store 的 currency,与 pricing-currency.test.ts 同款(字段以真实类型为准)。
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

describe('formatVideoPrice', () => {
  test('renders an already-converted display value verbatim without trailing zeros', () => {
    expect(formatVideoPrice(0.75)).toBe('0.75')
    expect(formatVideoPrice(0.375)).toBe('0.375')
    expect(formatVideoPrice(0.625)).toBe('0.625')
    expect(formatVideoPrice(0.3125)).toBe('0.3125')
    expect(formatVideoPrice(1)).toBe('1')
    expect(formatVideoPrice(0)).toBe('0')
  })

  test('falls back to a placeholder for non-finite values', () => {
    expect(formatVideoPrice(Number.NaN)).toBe('—')
    expect(formatVideoPrice(Number.POSITIVE_INFINITY)).toBe('—')
  })
})

describe('formatVideoPriceMoney(USD/s 随站点展示货币)', () => {
  test('USD 显示模式:数值即存储的 USD/s 原值,不被汇率缩放', () => {
    expect(formatVideoPriceMoney(0.75)).toBe('$0.75')
    expect(formatVideoPriceMoney(0.1027)).toBe('$0.1027')
    expect(formatVideoPriceMoney(0.375)).toBe('$0.375')
  })

  test('CNY/7.3:USD/s 0.75 → 本地 ¥5.475(≈¥5.48/s 数量级),符号随货币', () => {
    seedDisplayCurrency('CNY', 7.3)
    expect(formatVideoPriceMoney(0.75)).toBe('¥5.475')
    expect(formatVideoPriceMoney(1)).toBe('¥7.3')
  })

  test('CNY 存量往返:legacy ¥0.75/s 迁移为 USD/s 后再展示回到 ¥0.75', () => {
    seedDisplayCurrency('CNY', 7.3)
    expect(formatVideoPriceMoney(0.75 / 7.3)).toBe('¥0.75')
  })

  test('小值保留:0.1027 USD/s 不会被格式化成 0', () => {
    expect(formatVideoPriceMoney(0.1027)).toBe('$0.1027')
    seedDisplayCurrency('CNY', 7.3)
    // 0.1027 × 7.3 = 0.74971,digitsSmall 6 保留
    expect(formatVideoPriceMoney(0.1027)).toBe('¥0.74971')
  })

  test('showSymbol:false 只出换算后的数值(表头已含符号与 /s)', () => {
    expect(formatVideoPriceMoney(0.75, { showSymbol: false })).toBe('0.75')
    seedDisplayCurrency('CNY', 7.3)
    expect(formatVideoPriceMoney(0.75, { showSymbol: false })).toBe('5.475')
  })

  test('TOKENS 展示模式回落美元语义($/rate 1)', () => {
    seedDisplayCurrency('TOKENS', 7.3)
    expect(formatVideoPriceMoney(0.75)).toBe('$0.75')
  })

  test('CUSTOM 展示模式用自定义符号与汇率', () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CUSTOM',
          customCurrencySymbol: '₩',
          customCurrencyExchangeRate: 1300,
        },
      },
    }))
    expect(formatVideoPriceMoney(0.75)).toBe('₩ 975')
  })

  test('非有限值/缺省值回退占位符', () => {
    expect(formatVideoPriceMoney(Number.NaN)).toBe('—')
    expect(formatVideoPriceMoney(null)).toBe('—')
    expect(formatVideoPriceMoney(undefined)).toBe('—')
  })
})

describe('formatOffPeakHour', () => {
  test('formats hours with zero padding', () => {
    expect(formatOffPeakHour(22)).toBe('22:00')
    expect(formatOffPeakHour(8)).toBe('08:00')
    expect(formatOffPeakHour(0)).toBe('00:00')
    expect(formatOffPeakHour(23)).toBe('23:00')
  })

  test('rejects out-of-range or fractional hours', () => {
    expect(formatOffPeakHour(-1)).toBe('—')
    expect(formatOffPeakHour(24)).toBe('—')
    expect(formatOffPeakHour(12.5)).toBe('—')
    expect(formatOffPeakHour(Number.NaN)).toBe('—')
  })
})

describe('getOffPeakWindowLabel', () => {
  test('flags a window crossing midnight', () => {
    expect(
      getOffPeakWindowLabel({
        start_hour: 22,
        end_hour: 8,
        timezone: 'Asia/Shanghai',
      })
    ).toEqual({ start: '22:00', end: '08:00', crossesMidnight: true })
  })

  test('does not flag a same-day window', () => {
    expect(
      getOffPeakWindowLabel({
        start_hour: 9,
        end_hour: 17,
        timezone: 'Asia/Shanghai',
      })
    ).toEqual({ start: '09:00', end: '17:00', crossesMidnight: false })
  })

  test('equal start and end hours mean off-peak is disabled and no label is shown', () => {
    const label = getOffPeakWindowLabel({
      start_hour: 22,
      end_hour: 22,
      timezone: 'Asia/Shanghai',
    })
    expect(label).toBe(null)
  })

  test('returns null when the window is missing or invalid', () => {
    expect(getOffPeakWindowLabel(undefined)).toBe(null)
    expect(
      getOffPeakWindowLabel({
        start_hour: 24,
        end_hour: 8,
        timezone: 'Asia/Shanghai',
      })
    ).toBe(null)
  })
})
