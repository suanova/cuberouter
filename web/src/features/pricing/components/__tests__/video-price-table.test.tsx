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
import { act, render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
  type CurrencyDisplayType,
} from '@/stores/system-config-store'

import type { VideoPriceTable as VideoPriceTableData } from '../../types'
import { VideoPriceTable } from '../video-price-table'

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
  setDisplay('USD', 1)
})

const table: VideoPriceTableData = {
  rows: [
    { resolution: '1080p', normal_price: 0.75, off_peak_price: 0.5 },
  ],
}

describe('VideoPriceTable currency reactivity', () => {
  test('挂载中展示货币从 CNY 切到 USD 后,表头符号与单价随之刷新', () => {
    setDisplay('CNY', 7.3)
    render(<VideoPriceTable table={table} />)

    // CNY/rate 7.3:表头 ¥,0.75 USD/s → ¥5.475/s(单元格无符号数值列)
    expect(screen.getByText('Video price (¥/s)')).toBeInTheDocument()
    expect(screen.getByText('5.475')).toBeInTheDocument()

    // 同一视图实例内切换展示货币 → 响应式重渲染
    act(() => setDisplay('USD', 1))
    expect(screen.getByText('Video price ($/s)')).toBeInTheDocument()
    expect(screen.getByText('0.75')).toBeInTheDocument()
    expect(screen.queryByText('Video price (¥/s)')).not.toBeInTheDocument()
  })
})
