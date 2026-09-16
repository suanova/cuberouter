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
  addImagePriceRowDraft,
  createImagePriceRowDraft,
  rebaseImagePriceDrafts,
  removeImagePriceRowDraft,
  updateImagePriceRowDraft,
  imagePriceDraftsFromTable,
  imagePriceTableFromDrafts,
  type ImagePriceRowDraft,
} from '../image-price-drafts'

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

function resetDisplayCurrency(): void {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
}

function draft(overrides: Partial<ImagePriceRowDraft> = {}): ImagePriceRowDraft {
  return { ...createImagePriceRowDraft(), ...overrides }
}

describe('image price drafts currency boundary (CNY rate 7.3)', () => {
  beforeEach(() => seedDisplayCurrency('CNY', 7.3))

  test('加载:表内 USD/张 按汇率转为显示货币草稿字符串', () => {
    const drafts = imagePriceDraftsFromTable({
      rows: [
        { resolution: '1024x1024', price: 0.75 },
        { resolution: '1328x1328', price: 1.5 },
      ],
    })

    expect(drafts.length).toBe(2)
    const [sq, hd] = drafts
    expect(typeof sq.id).toBe('string')
    expect(sq.resolution).toBe('1024x1024')
    expect(sq.price).toBe('5.475')
    expect(hd.resolution).toBe('1328x1328')
    expect(hd.price).toBe('10.95')
  })

  test('提交:显示货币草稿字符串按汇率转为 USD/张 表载荷', () => {
    const table = imagePriceTableFromDrafts([
      draft({ resolution: '1024x1024', price: '5.475' }),
    ])

    expect(table.rows).toEqual([{ resolution: '1024x1024', price: 0.75 }])
  })

  test('往返:USD/张 表 → 显示草稿 → USD/张 表数值稳定', () => {
    const original = {
      rows: [
        { resolution: '1024x1024', price: 0.75 },
        { resolution: '1328x1328', price: 1.5 },
      ],
    }

    const drafts = imagePriceDraftsFromTable(original)

    expect(imagePriceTableFromDrafts(drafts)).toEqual(original)
  })

  test('空/非法价格草稿仍归零(0 ÷ 汇率 = 0)', () => {
    const table = imagePriceTableFromDrafts([
      draft({ resolution: '4K', price: 'abc' }),
    ])

    expect(table.rows[0].price).toBe(0)
  })
})

describe('image price drafts currency boundary (USD)', () => {
  beforeEach(resetDisplayCurrency)

  test('加载/提交均为恒等,与旧行为一致', () => {
    const drafts = imagePriceDraftsFromTable({
      rows: [{ resolution: '1024x1024', price: 0.75 }],
    })

    expect(drafts[0].price).toBe('0.75')

    const table = imagePriceTableFromDrafts([
      draft({ resolution: '1024x1024', price: '0.75' }),
    ])

    expect(table.rows).toEqual([{ resolution: '1024x1024', price: 0.75 }])
  })
})

describe('image price draft list helpers', () => {
  beforeEach(resetDisplayCurrency)

  test('adding a row and editing values emits a table with the filled row', () => {
    let drafts = addImagePriceRowDraft([])
    expect(drafts.length).toBe(1)
    expect(drafts[0].resolution).toBe('')
    expect(drafts[0].price).toBe('')

    drafts = addImagePriceRowDraft(drafts)
    drafts = updateImagePriceRowDraft(drafts, 0, {
      resolution: '1024x1024',
      price: '0.75',
    })

    const table = imagePriceTableFromDrafts(drafts)
    expect(table).toEqual({
      rows: [{ resolution: '1024x1024', price: 0.75 }],
    })
  })

  test('removing a row drops it from the emitted table', () => {
    let drafts = imagePriceDraftsFromTable({
      rows: [
        { resolution: '1024x1024', price: 0.75 },
        { resolution: '1328x1328', price: 1.5 },
      ],
    })

    drafts = removeImagePriceRowDraft(drafts, 0)

    const table = imagePriceTableFromDrafts(drafts)
    expect(table.rows).toEqual([{ resolution: '1328x1328', price: 1.5 }])
  })

  test('trims resolution text when emitting the table', () => {
    const table = imagePriceTableFromDrafts([
      draft({ resolution: ' 1024x1024 ', price: '0.75' }),
    ])

    expect(table.rows[0].resolution).toBe('1024x1024')
    expect(table.rows[0].price).toBe(0.75)
  })

  test('keeps partially filled rows for backend validation', () => {
    const table = imagePriceTableFromDrafts([
      draft({ resolution: '4K' }),
      draft(),
    ])

    expect(table.rows).toEqual([{ resolution: '4K', price: 0 }])
  })

  test('keeps an explicit zero price as a valid draft value', () => {
    const table = imagePriceTableFromDrafts([
      draft({ resolution: '1024x1024', price: '0' }),
    ])

    expect(table.rows).toEqual([{ resolution: '1024x1024', price: 0 }])
  })
})

describe('rebaseImagePriceDrafts(汇率变化保留底层 USD 意图)', () => {
  test('CNY 7.3 → USD 1:显示草稿回落到 USD 数值', () => {
    const drafts = [
      draft({ resolution: '1024x1024', price: '5.475' }),
      draft({ resolution: '', price: '' }),
    ]

    const rebased = rebaseImagePriceDrafts(drafts, 7.3, 1)

    expect(rebased[0].resolution).toBe('1024x1024')
    expect(rebased[0].price).toBe('0.75')
    // 空行(未录入)原样保留
    expect(rebased[1].price).toBe('')
  })

  test('USD 1 → CNY 7.3:显示草稿按新汇率放大,保持同一美元意图', () => {
    const drafts = [draft({ resolution: '720p', price: '0.75' })]

    const rebased = rebaseImagePriceDrafts(drafts, 1, 7.3)

    expect(rebased[0].price).toBe('5.475')
  })

  test('汇率相等时恒等返回,不重建对象', () => {
    const drafts = [draft({ price: '5.475' })]
    expect(rebaseImagePriceDrafts(drafts, 7.3, 7.3)).toBe(drafts)
  })

  test('不可解析的录入串(空/尾点)原样保留,不打断输入', () => {
    const drafts = [draft({ resolution: '1024x1024', price: '.' })]

    const rebased = rebaseImagePriceDrafts(drafts, 7.3, 1)

    expect(rebased[0].price).toBe('.')
  })
})
