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
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import {
  getPriceDetail,
  getPriceSummary,
  type ModelPricingSnapshot,
} from '../model-pricing-snapshots'

// 注水方式与 model-pricing-core.test.ts 一致(store 真实类型,currency 不可为 null)。
function seedDisplayCurrency(type: CurrencyDisplayType, rate: number) {
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

function resetDisplayCurrency() {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
}

// 模拟 i18next:未翻译 key 原样返回,插值占位符按 options 替换。
const t = (key: string, options?: Record<string, string>) =>
  key.replaceAll(/\{\{(\w+)\}\}/g, (match, name: string) =>
    options && name in options ? options[name] : match
  )

const perRequestRow: ModelPricingSnapshot = {
  name: 'mj_imagine',
  price: '0.01',
  billingMode: 'per-request',
  hasConflict: false,
}

const perTokenRow: ModelPricingSnapshot = {
  name: 'gpt-4o',
  ratio: '1.25',
  completionRatio: '2',
  cacheRatio: '0.3',
  billingMode: 'per-token',
  hasConflict: false,
}

const ratioOnlyRow: ModelPricingSnapshot = {
  name: 'gpt-4o-mini',
  ratio: '1.25',
  billingMode: 'per-token',
  hasConflict: false,
}

const videoRow: ModelPricingSnapshot = {
  name: 'kling',
  billingMode: 'video-per-second',
  hasConflict: false,
}

beforeEach(resetDisplayCurrency)

describe('getPriceSummary 金额随显示货币 (CNY/7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  it('per-request 摘要显示本地价:¥0.073 / request', () => {
    expect(getPriceSummary(perRequestRow, t)).toBe('¥0.073 / request')
  })

  it('per-token 主价摘要显示本地价:Input ¥18.25 · 2 extras', () => {
    expect(getPriceSummary(perTokenRow, t)).toBe('Input ¥18.25 · 2 extras')
  })

  it('per-token 无额外 lane 时省略 extras 段', () => {
    expect(getPriceSummary(ratioOnlyRow, t)).toBe('Input ¥18.25')
  })

  it('视频模型摘要仍为模式文案', () => {
    expect(getPriceSummary(videoRow, t)).toBe('Video per second')
  })

  it('无价格时保留 Unset price', () => {
    expect(getPriceSummary({ ...perTokenRow, ratio: '' }, t)).toBe(
      'Unset price'
    )
  })
})

describe('getPriceSummary 金额随显示货币 (USD)', () => {
  it('per-request 摘要为 $0.01 / request', () => {
    expect(getPriceSummary(perRequestRow, t)).toBe('$0.01 / request')
  })

  it('per-token 主价摘要为 Input $2.5 · 2 extras', () => {
    expect(getPriceSummary(perTokenRow, t)).toBe('Input $2.5 · 2 extras')
  })
})

describe('getPriceDetail 金额随显示货币 (CNY/7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  it('派生 lane 摘要显示本地价:Output ¥36.5 · Cache ¥5.475', () => {
    expect(getPriceDetail(perTokenRow, t)).toBe(
      'Output ¥36.5 · Cache ¥5.475'
    )
  })

  it('per-request 细节为模式文案 Fixed request price', () => {
    expect(getPriceDetail(perRequestRow, t)).toBe('Fixed request price')
  })

  it('视频模型细节单位符号随货币:Video price (¥/s)', () => {
    expect(getPriceDetail(videoRow, t)).toBe('Video price (¥/s)')
  })

  it('无额外 lane 时保留 Base input price only', () => {
    expect(getPriceDetail(ratioOnlyRow, t)).toBe('Base input price only')
  })
})

describe('getPriceDetail 金额随显示货币 (USD)', () => {
  it('派生 lane 摘要为 Output $5 · Cache $0.75', () => {
    expect(getPriceDetail(perTokenRow, t)).toBe('Output $5 · Cache $0.75')
  })

  it('视频模型细节单位符号随货币:Video price ($/s)', () => {
    expect(getPriceDetail(videoRow, t)).toBe('Video price ($/s)')
  })
})

describe('getPriceSummary/getPriceDetail 小额精度 (CNY/7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  it('per-request 小数价保留 digitsSmall 精度:¥0.00146', () => {
    expect(
      getPriceSummary({ ...perRequestRow, price: '0.0002' }, t)
    ).toBe('¥0.00146 / request')
  })
})
