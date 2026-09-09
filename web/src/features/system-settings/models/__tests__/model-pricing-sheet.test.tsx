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
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { createRef } from 'react'
import i18next from 'i18next'
import { beforeAll, beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import {
  ModelPricingEditorPanel,
  type ModelPricingEditorPanelHandle,
} from '../model-pricing-sheet'

vi.mock('@/features/pricing/hooks/use-pricing-data', () => ({
  usePricingData: () => ({ models: [] }),
}))

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

function openPerRequestTab(): void {
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
    // FormControl 的 slot 会合并到 InputGroup 根节点,input 的父节点即输入框组;
    // 只断言可见的货币前缀与单位文案,不锁 DOM 层级
    const group = input.parentElement
    expect(group).not.toBeNull()
    expect(within(group as HTMLElement).getByText('¥')).toBeInTheDocument()
    expect(
      within(group as HTMLElement).getByText('per request')
    ).toBeInTheDocument()
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
    expect(within(group as HTMLElement).getByText('$')).toBeInTheDocument()
    expect(
      within(group as HTMLElement).getByText('per request')
    ).toBeInTheDocument()
    expect(
      screen.getByText('Cost in $ per request, regardless of tokens used.')
    ).toBeInTheDocument()
  })
})

describe('model pricing sheet draft exchange-rate rebase', () => {
  test('CNY 打开存量 per-request 价:汇率切到 USD 后草稿 rebase,提交仍为原 USD 价', async () => {
    setDisplay('CNY', 7.3)
    const ref = createRef<ModelPricingEditorPanelHandle>()
    render(
      <ModelPricingEditorPanel
        ref={ref}
        editData={{ name: 'gpt-video', price: '2.5' }}
      />
    )
    openPerRequestTab()

    // 加载边界:2.5 USD × 7.3 → ¥18.25 显示草稿
    const priceInput = screen.getByPlaceholderText('0.073')
    await waitFor(() => expect(priceInput).toHaveValue('18.25'))

    // 挂载中汇率切到 USD:草稿 rebase 回 2.5(不丢、不按旧汇率写回)
    act(() => setDisplay('USD', 1))
    await waitFor(() => expect(priceInput).toHaveValue('2.5'))

    const data = await act(async () => ref.current?.commitDraft())
    expect(data?.price).toBe('2.5')
  })
})
