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
import { fireEvent, render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import type { VideoPriceTable } from '@/features/pricing/types'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { VideoPriceEditor } from '../video-price-editor'

// 注水方式与 model-pricing-sheet.test.tsx 一致。
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

const fullHdRow: VideoPriceTable = {
  rows: [{ resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 }],
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Video price ({{symbol}}/s)': 'Video price ({{symbol}}/s)',
    'Off-peak price ({{symbol}}/s)': 'Off-peak price ({{symbol}}/s)',
  })
})

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

describe('video price editor display currency', () => {
  test('CNY mode:USD/s 表按汇率显示为 ¥ 草稿,表头与占位示例同随 ¥', () => {
    setDisplay('CNY', 7.3)
    render(<VideoPriceEditor table={fullHdRow} onChange={vi.fn()} />)

    // 加载边界:USD/s 0.75/0.375 → 草稿字符串 5.475/2.7375(×7.3 后归整)
    expect(screen.getByDisplayValue('5.475')).toBeInTheDocument()
    expect(screen.getByDisplayValue('2.7375')).toBeInTheDocument()
    expect(screen.getByText('Video price (¥/s)')).toBeInTheDocument()
    expect(screen.getByText('Off-peak price (¥/s)')).toBeInTheDocument()

    // 空行的占位示例也按显示货币给出(0.75/0.375 USD 的本地示例)
    const normalGroup = screen
      .getByDisplayValue('5.475')
      .parentElement as HTMLElement
    expect(normalGroup.firstElementChild?.textContent).toBe('¥')
    expect(screen.getByPlaceholderText('5.475')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('2.7375')).toBeInTheDocument()
  })

  test('USD mode:原值直通草稿,$ 表头与前缀保持', () => {
    render(<VideoPriceEditor table={fullHdRow} onChange={vi.fn()} />)

    expect(screen.getByDisplayValue('0.75')).toBeInTheDocument()
    expect(screen.getByDisplayValue('0.375')).toBeInTheDocument()
    expect(screen.getByText('Video price ($/s)')).toBeInTheDocument()
    expect(screen.getByText('Off-peak price ($/s)')).toBeInTheDocument()

    const normalGroup = screen
      .getByDisplayValue('0.75')
      .parentElement as HTMLElement
    expect(normalGroup.firstElementChild?.textContent).toBe('$')
    expect(screen.getByPlaceholderText('0.75')).toBeInTheDocument()
  })

  test('CNY mode:编辑草稿后 onChange 以 USD/s 发出(÷汇率出口)', () => {
    setDisplay('CNY', 7.3)
    const onChange = vi.fn()
    render(<VideoPriceEditor table={fullHdRow} onChange={onChange} />)

    // ¥7.3/s 的输入在载荷里回到 1 USD/s
    fireEvent.change(screen.getByDisplayValue('5.475'), {
      target: { value: '7.3' },
    })

    expect(onChange).toHaveBeenLastCalledWith({
      rows: [{ resolution: '1080p', normal_price: 1, off_peak_price: 0.375 }],
    })
  })

  test('add/remove resolution:发出的表只含非空行', () => {
    const onChange = vi.fn()
    render(<VideoPriceEditor table={fullHdRow} onChange={onChange} />)

    fireEvent.click(screen.getByRole('button', { name: 'Add resolution' }))
    // 新空行仍可编辑但被过滤出载荷
    expect(onChange).toHaveBeenLastCalledWith({
      rows: [
        { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.375 },
      ],
    })

    fireEvent.click(screen.getAllByRole('button', { name: 'Delete' })[0])
    expect(onChange).toHaveBeenLastCalledWith({ rows: [] })
  })
})
