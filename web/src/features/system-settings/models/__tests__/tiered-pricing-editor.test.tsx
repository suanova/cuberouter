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
import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { TieredPricingEditor } from '../tiered-pricing-editor'

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

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

const baseProps = {
  billingExpr: '',
  requestRuleExpr: '',
  onBillingExprChange: vi.fn(),
  onRequestRuleExprChange: vi.fn(),
}

function renderEditor(props: Partial<typeof baseProps> = {}) {
  return render(<TieredPricingEditor {...baseProps} {...props} />)
}

/** 第一个 spinbutton 是 Input price 的录入框(默认 tier 无条件行,缓存/媒体价在后)。 */
function inputPriceInput() {
  return screen.getAllByRole('spinbutton')[0]
}

describe('tiered editor currency boundary (CNY rate 7.3)', () => {
  beforeEach(() => setDisplay('CNY', 7.3))

  test('加载:表达式里的美元单价系数在输入框按汇率展示', () => {
    renderEditor({
      billingExpr: 'tier("base", p * 2.5 + c * 0)',
    })
    expect(inputPriceInput()).toHaveValue(18.25)
  })

  test('录入:填入本地货币价后生成的表达式仍是美元数', async () => {
    const onBillingExprChange = vi.fn()
    renderEditor({
      billingExpr: 'tier("base", p * 0 + c * 0)',
      onBillingExprChange,
    })
    fireEvent.change(inputPriceInput(), { target: { value: '18.25' } })

    await waitFor(() =>
      expect(onBillingExprChange).toHaveBeenLastCalledWith(
        'tier("base", p * 2.5 + c * 0)'
      )
    )
  })

  test('Token prices 单位徽标随展示货币符号', () => {
    renderEditor()
    expect(screen.getByText('¥/1M tokens')).toBeInTheDocument()
  })
})

describe('tiered editor currency boundary (USD 恒等)', () => {
  test('加载与录入数值原样透传,表达式不变', async () => {
    const onBillingExprChange = vi.fn()
    renderEditor({
      billingExpr: 'tier("base", p * 0.4 + c * 0)',
      onBillingExprChange,
    })
    expect(inputPriceInput()).toHaveValue(0.4)

    fireEvent.change(inputPriceInput(), { target: { value: '0.5' } })
    await waitFor(() =>
      expect(onBillingExprChange).toHaveBeenLastCalledWith(
        'tier("base", p * 0.5 + c * 0)'
      )
    )
  })

  test('Token prices 单位徽标保持 $', () => {
    renderEditor()
    expect(screen.getByText('$/1M tokens')).toBeInTheDocument()
  })
})
