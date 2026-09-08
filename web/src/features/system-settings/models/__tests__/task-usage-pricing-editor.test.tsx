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

import type { BillingUsageSchema } from '@/features/pricing/types'
import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import { TaskUsagePricingEditor } from '../task-usage-pricing-editor'

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

const usageSchema: BillingUsageSchema = {
  seconds: { type: 'number', unit: 'second' },
}

type RenderEditorOverrides = Partial<{
  billingExpr: string
  requestRuleExpr: string
  onBillingExprChange: (next: string) => void
}>

function renderEditor(overrides: RenderEditorOverrides = {}) {
  const onBillingExprChange = overrides.onBillingExprChange ?? (() => undefined)
  const utils = render(
    <TaskUsagePricingEditor
      billingExpr={overrides.billingExpr ?? ''}
      requestRuleExpr={overrides.requestRuleExpr ?? ''}
      usageSchema={usageSchema}
      onBillingExprChange={onBillingExprChange}
      onRequestRuleExprChange={() => undefined}
    />
  )
  return { ...utils, onBillingExprChange }
}

/** 单数字字段(无枚举)视图下第一个 spinbutton 是 usage price 输入框。 */
function unitPriceInput() {
  return screen.getAllByRole('spinbutton')[0]
}

describe('task usage editor currency boundary (CNY rate 7.3)', () => {
  beforeEach(() => setDisplay('CNY', 7.3))

  test('加载:表达式里的美元单价在输入框与预览公式中按汇率展示', () => {
    renderEditor({ billingExpr: 'tier("base", u("seconds") * 0.4)' })

    expect(unitPriceInput()).toHaveValue(2.92)
    // 示例 5s × $0.4/s = $2 → ¥14.6;行摘要单位价 ¥2.92/s
    const formula = screen.getByText((content) => content.includes('= ¥14.6'))
    expect(formula).toHaveTextContent('¥2.92')
  })

  test('录入:填入本地货币价后触发生成的表达式仍是美元数', async () => {
    const onBillingExprChange = vi.fn()
    renderEditor({
      billingExpr: 'tier("base", u("seconds") * 0)',
      onBillingExprChange,
    })

    fireEvent.change(unitPriceInput(), { target: { value: '5.84' } })

    await waitFor(() =>
      expect(onBillingExprChange).toHaveBeenLastCalledWith(
        'tier("base", u("seconds") * 0.8)'
      )
    )
  })

  test('帮助文案插值当前展示货币符号', () => {
    renderEditor()
    expect(
      screen.getByText(/Visual inputs and previews show ¥;/)
    ).toBeInTheDocument()
  })

  test('矩阵单元格:填本地货币价后生成的表达式仍是美元数', async () => {
    const enumSchema: BillingUsageSchema = {
      seconds: { type: 'number', unit: 'second' },
      mode: { enum: ['std', 'pro'] },
    }
    const onBillingExprChange = vi.fn()
    render(
      <TaskUsagePricingEditor
        billingExpr=''
        requestRuleExpr=''
        usageSchema={enumSchema}
        onBillingExprChange={onBillingExprChange}
        onRequestRuleExprChange={() => undefined}
      />
    )

    const stdCell = screen.getByRole('spinbutton', { name: 'seconds: std' })
    fireEvent.change(stdCell, { target: { value: '2.92' } })

    await waitFor(() =>
      expect(onBillingExprChange).toHaveBeenLastCalledWith(
        'u("mode") == "std" ? tier("std", u("seconds") * 0.4) : tier("pro", u("seconds") * 0)'
      )
    )
  })
})

describe('task usage editor currency boundary (USD 恒等)', () => {
  test('加载与录入数值原样透传,表达式不变', async () => {
    const onBillingExprChange = vi.fn()
    renderEditor({
      billingExpr: 'tier("base", u("seconds") * 0.4)',
      onBillingExprChange,
    })
    expect(unitPriceInput()).toHaveValue(0.4)

    fireEvent.change(unitPriceInput(), { target: { value: '0.5' } })
    await waitFor(() =>
      expect(onBillingExprChange).toHaveBeenLastCalledWith(
        'tier("base", u("seconds") * 0.5)'
      )
    )
  })

  test('帮助文案保持 $ 符号', () => {
    renderEditor()
    expect(
      screen.getByText(/Visual inputs and previews show \$;/)
    ).toBeInTheDocument()
  })
})
