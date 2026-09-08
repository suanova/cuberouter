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
  addVideoPriceRowDraft,
  createVideoPriceRowDraft,
  removeVideoPriceRowDraft,
  updateVideoPriceRowDraft,
  videoPriceDraftsFromTable,
  videoPriceTableFromDrafts,
  type VideoPriceRowDraft,
} from '../video-price-drafts'

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

function draft(overrides: Partial<VideoPriceRowDraft> = {}): VideoPriceRowDraft {
  return { ...createVideoPriceRowDraft(), ...overrides }
}

describe('video price drafts currency boundary (CNY rate 7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  test('加载:表内 USD/s 按汇率转为显示货币草稿字符串', () => {
    const drafts = videoPriceDraftsFromTable({
      rows: [
        { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
        { resolution: '720p', normal_price: 0.625, off_peak_price: 0.3125 },
      ],
    })

    expect(drafts.length).toBe(2)
    const [fullHd, hd] = drafts
    expect(typeof fullHd.id).toBe('string')
    expect(fullHd.resolution).toBe('1080p')
    expect(fullHd.normalPrice).toBe('5.475')
    expect(fullHd.offPeakPrice).toBe('2.7375')
    expect(hd.resolution).toBe('720p')
    expect(hd.normalPrice).toBe('4.5625')
    expect(hd.offPeakPrice).toBe('2.28125')
  })

  test('提交:显示货币草稿字符串按汇率转为 USD/s 表载荷', () => {
    const table = videoPriceTableFromDrafts([
      draft({
        resolution: '1080p',
        normalPrice: '5.475',
        offPeakPrice: '2.7375',
      }),
    ])

    expect(table.rows).toEqual([
      { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
    ])
  })

  test('往返:USD/s 表 → 显示草稿 → USD/s 表数值稳定', () => {
    const original = {
      rows: [
        { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
        { resolution: '720p', normal_price: 0.625, off_peak_price: 0.3125 },
      ],
    }

    const drafts = videoPriceDraftsFromTable(original)

    expect(videoPriceTableFromDrafts(drafts)).toEqual(original)
  })

  test('空/非法价格草稿仍归零(0 ÷ 汇率 = 0)', () => {
    const table = videoPriceTableFromDrafts([
      draft({ resolution: '4K', normalPrice: '', offPeakPrice: 'abc' }),
    ])

    expect(table.rows[0].normal_price).toBe(0)
    expect(table.rows[0].off_peak_price).toBe(0)
  })
})

describe('video price drafts currency boundary (USD)', () => {
  beforeEach(resetDisplayCurrency)

  test('加载/提交均为恒等,与旧行为一致', () => {
    const drafts = videoPriceDraftsFromTable({
      rows: [
        { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
      ],
    })

    expect(drafts[0].normalPrice).toBe('0.75')
    expect(drafts[0].offPeakPrice).toBe('0.375')

    const table = videoPriceTableFromDrafts([
      draft({ resolution: '1080p', normalPrice: '0.75', offPeakPrice: '0.375' }),
    ])

    expect(table.rows).toEqual([
      { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
    ])
  })
})

describe('video price draft list helpers', () => {
  beforeEach(resetDisplayCurrency)

  test('adding a row and editing values emits a table with the filled row', () => {
    let drafts = addVideoPriceRowDraft([])
    expect(drafts.length).toBe(1)
    expect(drafts[0].resolution).toBe('')
    expect(drafts[0].normalPrice).toBe('')
    expect(drafts[0].offPeakPrice).toBe('')

    drafts = addVideoPriceRowDraft(drafts)
    drafts = updateVideoPriceRowDraft(drafts, 0, {
      resolution: '1080p',
      normalPrice: '0.75',
      offPeakPrice: '0.375',
    })

    const table = videoPriceTableFromDrafts(drafts)
    expect(table).toEqual({
      rows: [
        { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
      ],
    })
  })

  test('removing a row drops it from the emitted table', () => {
    let drafts = videoPriceDraftsFromTable({
      rows: [
        { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
        { resolution: '720p', normal_price: 0.625, off_peak_price: 0.3125 },
      ],
    })

    drafts = removeVideoPriceRowDraft(drafts, 0)

    const table = videoPriceTableFromDrafts(drafts)
    expect(table.rows).toEqual([
      { resolution: '720p', normal_price: 0.625, off_peak_price: 0.3125 },
    ])
  })

  test('trims resolution text when emitting the table', () => {
    const table = videoPriceTableFromDrafts([
      draft({ resolution: ' 1080p ', normalPrice: '0.75', offPeakPrice: '0.5' }),
    ])

    expect(table.rows[0].resolution).toBe('1080p')
    expect(table.rows[0].normal_price).toBe(0.75)
  })

  test('keeps partially filled rows for backend validation', () => {
    const table = videoPriceTableFromDrafts([
      draft({ resolution: '4K' }),
      draft(),
    ])

    expect(table.rows).toEqual([
      { resolution: '4K', normal_price: 0, off_peak_price: 0 },
    ])
  })

  test('keeps an explicit zero off-peak price as a valid draft value', () => {
    const table = videoPriceTableFromDrafts([
      draft({ resolution: '1080p', normalPrice: '0.75', offPeakPrice: '0' }),
    ])

    expect(table.rows).toEqual([
      { resolution: '1080p', normal_price: 0.75, off_peak_price: 0 },
    ])
  })
})
