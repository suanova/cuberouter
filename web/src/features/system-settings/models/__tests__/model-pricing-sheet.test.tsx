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
import { fireEvent, render, screen, within } from '@testing-library/react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { ModelPricingEditorPanel } from '../model-pricing-sheet'

vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({ models: [] }),
}))

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

beforeAll(() => {
  i18next.addResourceBundle('en', 'translation', {
    'Price in {{symbol}} per 1M tokens.': 'Price in {{symbol}} per 1M tokens.',
    'Cost in {{symbol}} per request, regardless of tokens used.':
      'Cost in {{symbol}} per request, regardless of tokens used.',
    'per request': 'per request',
  })
})

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

function openPerRequestTab() {
  fireEvent.click(screen.getByRole('tab', { name: 'Per-request' }))
}

describe('model pricing sheet currency copy', () => {
  test('CNY mode: per-request 输入前缀为 ¥、说明文案插值 ¥', () => {
    setDisplay('CNY', 7.3)
    render(<ModelPricingEditorPanel />)

    // 默认 per-token tab:Input price 下方说明随货币(而非写死 USD)
    expect(
      screen.getByText('Price in ¥ per 1M tokens.')
    ).toBeInTheDocument()

    openPerRequestTab()

    const input = screen.getByPlaceholderText('0.073')
    // FormControl 的 slot 会合并到 InputGroup 根节点,input 的父节点即输入框组
    const group = input.parentElement
    expect(group).not.toBeNull()
    const prefix = within(group as HTMLElement).getByText('¥')
    const suffix = within(group as HTMLElement).getByText('per request')
    // 布局契约同 PriceInput:货币符号为输入框组首元素,单位文案为末尾元素
    expect(group?.firstElementChild).toBe(prefix)
    expect(group?.lastElementChild).toBe(suffix)
    expect(
      screen.getByText('Cost in ¥ per request, regardless of tokens used.')
    ).toBeInTheDocument()
  })

  test('USD mode: per-request 输入前缀与说明文案保持 $', () => {
    render(<ModelPricingEditorPanel />)

    openPerRequestTab()

    const input = screen.getByPlaceholderText('0.01')
    const group = input.parentElement
    expect(group).not.toBeNull()
    const prefix = within(group as HTMLElement).getByText('$')
    const suffix = within(group as HTMLElement).getByText('per request')
    expect(group?.firstElementChild).toBe(prefix)
    expect(group?.lastElementChild).toBe(suffix)
    expect(
      screen.getByText('Cost in $ per request, regardless of tokens used.')
    ).toBeInTheDocument()
  })
})
