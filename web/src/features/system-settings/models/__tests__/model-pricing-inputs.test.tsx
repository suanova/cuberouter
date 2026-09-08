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
import { render, screen } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { PriceInput, PriceLane } from '../model-pricing-inputs'

// 注水方式与 lib/__tests__/pricing-currency.test.ts 一致:在
// DEFAULT_CURRENCY_CONFIG 之上覆盖被测字段,重置即恢复默认。
function setDisplay(type: CurrencyDisplayType, rate: number, symbol = '¤') {
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

const noop = () => {}

function expectSymbolAroundInput(symbol: string, unit: string, value: string) {
  const prefix = screen.getByText(symbol)
  const suffix = screen.getByText(`${symbol}/${unit}`)
  const input = screen.getByDisplayValue(value)
  const group = input.closest('[data-slot="input-group"]')
  // 输入控件的布局契约:货币符号是输入框前的首元素,单位后缀是末尾元素
  expect(group?.firstElementChild).toBe(prefix)
  expect(group?.lastElementChild).toBe(suffix)
}

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Price in {{symbol}} per 1M tokens.': 'Price in {{symbol}} per 1M tokens.',
  })
})

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

describe('PriceInput currency symbol and unit', () => {
  test('CNY mode shows ¥ before the input and a ¥/1M suffix', () => {
    setDisplay('CNY', 7.3)
    render(<PriceInput value='18.25' onChange={noop} />)

    expectSymbolAroundInput('¥', '1M', '18.25')
  })

  test('USD mode shows $ before the input and a $/1M suffix', () => {
    setDisplay('USD', 1)
    render(<PriceInput value='0.3' onChange={noop} />)

    expectSymbolAroundInput('$', '1M', '0.3')
  })

  test('CUSTOM mode uses the configured currency symbol', () => {
    setDisplay('CUSTOM', 2.5, '₩')
    render(<PriceInput value='0.75' onChange={noop} />)

    expectSymbolAroundInput('₩', '1M', '0.75')
  })

  test('unit prop overrides the suffix unit while the prefix symbol stays', () => {
    setDisplay('USD', 1)
    render(<PriceInput value='0.3' unit='1K' onChange={noop} />)

    expectSymbolAroundInput('$', '1K', '0.3')
  })
})

describe('PriceLane hint text', () => {
  test('enabled lane hint interpolates the display currency symbol', () => {
    setDisplay('CNY', 7.3)
    render(
      <PriceLane
        title='Audio input price'
        description='Token price for audio input.'
        placeholder='0.5'
        value='18.25'
        enabled
        onEnabledChange={noop}
        onChange={noop}
      />
    )

    expect(screen.getByText('Price in ¥ per 1M tokens.')).toBeInTheDocument()
    expect(screen.getByText('¥/1M')).toBeInTheDocument()
  })
})
