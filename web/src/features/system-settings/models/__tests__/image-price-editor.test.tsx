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
import { act, fireEvent, render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import type { ImagePriceTable } from '@/features/pricing/types'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { ImagePriceEditor } from '../image-price-editor'

// 注水方式与 model-pricing-sheet.test.tsx 一致。
function setDisplay(type: CurrencyDisplayType, rate: number): void {
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

const fastRow: ImagePriceTable = {
  rows: [{ tier: 'fast', price: 0.75 }],
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Image price ({{symbol}}/image)': 'Image price ({{symbol}}/image)',
    Quality: 'Quality',
    Fast: 'Fast',
    Standard: 'Standard',
    High: 'High',
  })
})

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

describe('image price editor fixed quality tiers', () => {
  test('renders the three fixed tier rows with no add/remove controls', () => {
    render(<ImagePriceEditor table={fastRow} onChange={vi.fn()} />)

    expect(screen.getByText('Quality')).toBeInTheDocument()
    expect(screen.getByText('Fast')).toBeInTheDocument()
    expect(screen.getByText('Standard')).toBeInTheDocument()
    expect(screen.getByText('High')).toBeInTheDocument()
    // 固定档位:无添加/删除控件
    expect(
      screen.queryByRole('button', { name: 'Add resolution' })
    ).not.toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Delete' })).not.toBeInTheDocument()
    // 每行一个价格输入框
    expect(screen.getAllByRole('textbox')).toHaveLength(3)
  })

  test('empty tier prices emit an empty table (unpriced tiers bill at anchor)', () => {
    const onChange = vi.fn()
    render(<ImagePriceEditor table={{ rows: [] }} onChange={onChange} />)

    fireEvent.change(screen.getAllByRole('textbox')[1], {
      target: { value: '2' },
    })
    expect(onChange).toHaveBeenLastCalledWith({
      rows: [{ tier: 'standard', price: 2 }],
    })

    fireEvent.change(screen.getAllByRole('textbox')[1], {
      target: { value: '' },
    })
    expect(onChange).toHaveBeenLastCalledWith({ rows: [] })
  })
})

describe('image price editor display currency', () => {
  test('CNY mode:USD/张 表按汇率显示为 ¥ 草稿,表头与占位示例同随 ¥', () => {
    setDisplay('CNY', 7.3)
    render(<ImagePriceEditor table={fastRow} onChange={vi.fn()} />)

    // 加载边界:USD/张 0.75 → 草稿字符串 5.475(×7.3 后归整)
    expect(screen.getByDisplayValue('5.475')).toBeInTheDocument()
    expect(screen.getByText('Image price (¥/image)')).toBeInTheDocument()

    // 空档行的占位示例也按显示货币给出(0.02 USD 的本地示例)
    const priceGroup = screen
      .getByDisplayValue('5.475')
      .parentElement as HTMLElement
    expect(within(priceGroup).getByText('¥')).toBeInTheDocument()
    expect(screen.getAllByPlaceholderText('0.146')).toHaveLength(3)
  })

  test('USD mode:原值直通草稿,$ 表头与前缀保持', () => {
    render(<ImagePriceEditor table={fastRow} onChange={vi.fn()} />)

    expect(screen.getByDisplayValue('0.75')).toBeInTheDocument()
    expect(screen.getByText('Image price ($/image)')).toBeInTheDocument()

    const priceGroup = screen
      .getByDisplayValue('0.75')
      .parentElement as HTMLElement
    expect(within(priceGroup).getByText('$')).toBeInTheDocument()
    expect(screen.getAllByPlaceholderText('0.02')).toHaveLength(3)
  })

  test('CNY mode:编辑草稿后 onChange 以 USD/张 发出(÷汇率出口)', () => {
    setDisplay('CNY', 7.3)
    const onChange = vi.fn()
    render(<ImagePriceEditor table={fastRow} onChange={onChange} />)

    // ¥7.3/张 的输入在载荷里回到 1 USD/张
    fireEvent.change(screen.getByDisplayValue('5.475'), {
      target: { value: '7.3' },
    })

    expect(onChange).toHaveBeenLastCalledWith({
      rows: [{ tier: 'fast', price: 1 }],
    })
  })

  test('挂载中汇率变化:草稿 rebase 保留 USD 意图,随后编辑按新汇率发出正确载荷', () => {
    setDisplay('CNY', 7.3)
    const onChange = vi.fn()
    render(<ImagePriceEditor table={fastRow} onChange={onChange} />)

    // CNY 加载:0.75 USD/张 → ¥5.475 草稿
    expect(screen.getByDisplayValue('5.475')).toBeInTheDocument()

    // 切到 USD:草稿回落到 0.75(不被旧汇率污染),表头随 $
    act(() => setDisplay('USD', 1))
    expect(screen.getByDisplayValue('0.75')).toBeInTheDocument()
    expect(screen.queryByDisplayValue('5.475')).not.toBeInTheDocument()
    expect(screen.getByText('Image price ($/image)')).toBeInTheDocument()

    // 翻转后继续编辑:整表按当前汇率折算,未编辑行保持原 USD
    fireEvent.change(screen.getByDisplayValue('0.75'), {
      target: { value: '1.5' },
    })
    expect(onChange).toHaveBeenLastCalledWith({
      rows: [{ tier: 'fast', price: 1.5 }],
    })
  })
})
