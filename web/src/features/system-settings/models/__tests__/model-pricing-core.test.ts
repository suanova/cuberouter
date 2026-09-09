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
import { beforeEach, describe, expect, it, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import {
  EMPTY_LANE_ENABLED,
  EMPTY_LANE_PRICES,
  basePriceToRatio,
  buildPricingSubmitData,
  buildPreviewRows,
  createInitialLaneState,
  displayPriceToUsd,
  getInitialPricingMode,
  usdPriceToDisplay,
  type ModelPricingFormValues,
} from '../model-pricing-core'

// 注水方式与 pricing-currency.test.ts 一致(store 真实类型,currency 不可为 null)。
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

function resetDisplayCurrency(): void {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
}

const emptyValues: ModelPricingFormValues = {
  name: 'viduq3-pro',
  price: '',
  ratio: '',
  cacheRatio: '',
  createCacheRatio: '',
  completionRatio: '',
  imageRatio: '',
  audioRatio: '',
  audioCompletionRatio: '',
}

const videoTable = {
  rows: [
    { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
    { resolution: '720p', normal_price: 0.625, off_peak_price: 0.3125 },
  ],
}

describe('getInitialPricingMode', () => {
  test('starts in video-per-second mode when the model has a video price table', () => {
    const mode = getInitialPricingMode({
      ...emptyValues,
      billingMode: 'video-per-second',
      videoPrices: videoTable,
    })
    expect(mode).toBe('video-per-second')
  })

  test('starts in video-per-second mode when a table exists without an explicit mode', () => {
    const mode = getInitialPricingMode({
      ...emptyValues,
      videoPrices: videoTable,
    })
    expect(mode).toBe('video-per-second')
  })

  test('keeps the existing tiered_expr, per-request and per-token detection', () => {
    expect(getInitialPricingMode({ ...emptyValues, billingMode: 'tiered_expr' })).toBe('tiered_expr')
    expect(getInitialPricingMode({ ...emptyValues, price: '0.01' })).toBe('per-request')
    expect(getInitialPricingMode(emptyValues)).toBe('per-token')
    expect(getInitialPricingMode(null)).toBe('per-token')
  })
})

describe('buildPricingSubmitData', () => {
  test('video-per-second payload carries the model video price table', () => {
    const data = buildPricingSubmitData(emptyValues, 'video-per-second', {
      billingExpr: '',
      requestRuleExpr: '',
      videoPrices: videoTable,
    })

    expect(data.billingMode).toBe('video-per-second')
    expect(data.videoPrices).toEqual(videoTable)
    expect(data.billingExpr).toBe(undefined)
  })

  test('tiered_expr payload keeps the expression fields only', () => {
    const data = buildPricingSubmitData(emptyValues, 'tiered_expr', {
      billingExpr: 'tier("base", p * 2)',
      requestRuleExpr: '',
    })

    expect(data.billingMode).toBe('tiered_expr')
    expect(data.billingExpr).toBe('tier("base", p * 2)')
    expect(data.videoPrices).toBe(undefined)
  })

  test('per-request payload carries the fixed price', () => {
    const data = buildPricingSubmitData(
      { ...emptyValues, price: '0.01' },
      'per-request',
      { billingExpr: '', requestRuleExpr: '' }
    )

    expect(data.billingMode).toBe('per-request')
    expect(data.price).toBe('0.01')
    expect(data.videoPrices).toBe(undefined)
  })
})

describe('buildPreviewRows video branch', () => {
  test('lists the configured resolutions for video-per-second mode', () => {
    const rows = buildPreviewRows(
      emptyValues,
      'video-per-second',
      '',
      '',
      '',
      EMPTY_LANE_PRICES,
      EMPTY_LANE_ENABLED,
      (key) => key,
      videoTable
    )

    expect(rows).toEqual([
      { key: 'mode', label: 'Mode', value: 'Video per second' },
      { key: 'videoRows', label: 'Resolution', value: '1080p, 720p' },
    ])
  })

  test('shows empty state when no rows are configured', () => {
    const rows = buildPreviewRows(
      emptyValues,
      'video-per-second',
      '',
      '',
      '',
      EMPTY_LANE_PRICES,
      EMPTY_LANE_ENABLED,
      (key) => key,
      { rows: [] }
    )

    expect(rows[1].value).toBe('Empty')
  })
})

describe('editor currency boundary (CNY rate 7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  it('加载:ratio 1.25 的输入价显示为本地货币 18.25($2.5/1M × 7.3)', () => {
    const state = createInitialLaneState({ name: 'gpt-4o', ratio: '1.25' })
    expect(state.promptPrice).toBe('18.25')
  })
  it('加载:lane 价 = 倍率 × 本地主价(只在基准处换算一次,不重复乘汇率)', () => {
    const state = createInitialLaneState({
      name: 'gpt-4o',
      ratio: '1.25',
      completionRatio: '2',
      audioRatio: '0.5',
      audioCompletionRatio: '2',
    })
    expect(state.prices.completion).toBe('36.5')
    expect(state.prices.audioInput).toBe('9.125')
    expect(state.prices.audioOutput).toBe('18.25')
  })
  it('提交:per-request 本地价 18.25 转回 2.5 美元载荷', () => {
    const data = buildPricingSubmitData(
      { name: 'mj_imagine', price: '18.25' },
      'per-request',
      { billingExpr: '', requestRuleExpr: '' }
    )
    expect(data.price).toBe('2.5')
  })
  it('usdPriceToDisplay / displayPriceToUsd 往返稳定', () => {
    expect(usdPriceToDisplay(2.5)).toBe('18.25')
    expect(displayPriceToUsd(18.25)).toBeCloseTo(2.5, 9)
  })
})

describe('editor currency boundary (USD)', () => {
  beforeEach(resetDisplayCurrency)

  it('USD 显示模式下与旧行为一致(恒等)', () => {
    expect(usdPriceToDisplay(2.5)).toBe('2.5')
    expect(displayPriceToUsd(2.5)).toBe(2.5)
    const state = createInitialLaneState({ name: 'gpt-4o', ratio: '1.25' })
    expect(state.promptPrice).toBe('2.5')
  })
})

describe('buildPreviewRows 金额前缀随显示货币', () => {
  it('CNY/7.3 下 per-token 预览金额行带 ¥ 前缀', () => {
    seedDisplayCurrency('CNY', 7.3)
    const rows = buildPreviewRows(
      emptyValues,
      'per-token',
      '',
      '',
      '18.25',
      { ...EMPTY_LANE_PRICES, completion: '36.5' },
      { ...EMPTY_LANE_ENABLED, completion: true },
      (key) => key
    )

    expect(rows.find((row) => row.key === 'inputPrice')?.value).toBe(
      '¥18.25'
    )
    expect(rows.find((row) => row.key === 'completion')?.value).toBe('¥36.5')
    expect(rows.find((row) => row.key === 'cache')?.value).toBe('Empty')
  })

  it('USD 下 per-token 预览金额行仍为 $ 前缀', () => {
    resetDisplayCurrency()
    const rows = buildPreviewRows(
      emptyValues,
      'per-token',
      '',
      '',
      '2.5',
      { ...EMPTY_LANE_PRICES, completion: '5' },
      { ...EMPTY_LANE_ENABLED, completion: true },
      (key) => key
    )

    expect(rows.find((row) => row.key === 'inputPrice')?.value).toBe('$2.5')
    expect(rows.find((row) => row.key === 'completion')?.value).toBe('$5')
  })
})

describe('editor currency boundary — 主价变更期基准 ratio 推导(汇率无关)', () => {
  it('CNY/7.3 与 USD 两态下推导的落库 ratio 相同($2.5 主价 ÷ $2 基准 = 1.25)', () => {
    const rates: Array<[CurrencyDisplayType, number, string]> = [
      ['CNY', 7.3, '18.25'],
      ['USD', 1, '2.5'],
    ]
    const ratios = rates.map(([type, rate, price]) => {
      seedDisplayCurrency(type, rate)
      return basePriceToRatio(price)
    })
    expect(ratios).toEqual(['1.25', '1.25'])
  })

  it('空输入推导为空串(表单 ratio 空值语义)', () => {
    resetDisplayCurrency()
    expect(basePriceToRatio('')).toBe('')
  })
})
