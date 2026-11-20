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
import { render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import type { PaymentMethodData } from '../payment-method-dialog'
import {
  deletePaymentMethod,
  insertPaymentMethodTemplate,
  PaymentMethodsVisualEditor,
  savePaymentMethod,
} from '../payment-methods-visual-editor'

const epayAlipay: PaymentMethodData = {
  name: '支付宝',
  type: 'alipay',
  icon: 'SiAlipay',
}

const paymentAsiaAlipayHK: PaymentMethodData = {
  name: 'AlipayHK',
  type: 'paymentasia_alipayhk',
  icon: 'SiAlipay',
}

const paymentAsiaWeChat: PaymentMethodData = {
  name: 'WeChat Pay',
  type: 'paymentasia_wechat',
  icon: 'SiWechat',
}

describe('savePaymentMethod', () => {
  test('replaces only the edited entry and keeps other gateway entries intact', () => {
    const rawValue = JSON.stringify([
      epayAlipay,
      paymentAsiaAlipayHK,
      paymentAsiaWeChat,
    ])

    const result = savePaymentMethod(rawValue, paymentAsiaAlipayHK, {
      name: 'AlipayHK',
      type: 'paymentasia_alipayhk',
      min_topup: '20',
    })

    expect(JSON.parse(result)).toEqual([
      epayAlipay,
      {
        name: 'AlipayHK',
        type: 'paymentasia_alipayhk',
        min_topup: '20',
      },
      paymentAsiaWeChat,
    ])
  })
})

describe('deletePaymentMethod', () => {
  test('removes only the deleted entry and keeps other gateway entries intact', () => {
    const rawValue = JSON.stringify([
      epayAlipay,
      paymentAsiaAlipayHK,
      paymentAsiaWeChat,
    ])

    const result = deletePaymentMethod(rawValue, paymentAsiaAlipayHK)

    expect(JSON.parse(result)).toEqual([epayAlipay, paymentAsiaWeChat])
  })
})

describe('insertPaymentMethodTemplate', () => {
  test('appends the template while keeping existing entries from other gateways', () => {
    const rawValue = JSON.stringify([epayAlipay, paymentAsiaAlipayHK])
    const template: PaymentMethodData = {
      name: 'FPS',
      type: 'paymentasia_fps',
      icon: 'LuLandmark',
    }

    const result = insertPaymentMethodTemplate(rawValue, template)

    expect(JSON.parse(result)).toEqual([
      epayAlipay,
      paymentAsiaAlipayHK,
      template,
    ])
  })

  test('does not insert the template twice', () => {
    const rawValue = JSON.stringify([epayAlipay, paymentAsiaAlipayHK])
    const template: PaymentMethodData = {
      name: 'FPS',
      type: 'paymentasia_fps',
      icon: 'LuLandmark',
    }

    const first = insertPaymentMethodTemplate(rawValue, template)
    const second = insertPaymentMethodTemplate(first, template)

    expect(JSON.parse(second)).toEqual([
      epayAlipay,
      paymentAsiaAlipayHK,
      template,
    ])
  })
})

describe('PaymentMethodsVisualEditor type filtering', () => {
  const value = JSON.stringify([epayAlipay, paymentAsiaAlipayHK])

  test('shows only PaymentAsia entries when the filter allows PaymentAsia', () => {
    render(
      <PaymentMethodsVisualEditor
        value={value}
        onChange={vi.fn()}
        typeFilter={(type) => type.startsWith('paymentasia_')}
      />
    )

    expect(screen.queryAllByText('支付宝')).toHaveLength(0)
    expect(screen.getAllByText('paymentasia_alipayhk').length).toBeGreaterThan(
      0
    )
  })

  test('shows only non-PaymentAsia entries when the filter excludes PaymentAsia', () => {
    render(
      <PaymentMethodsVisualEditor
        value={value}
        onChange={vi.fn()}
        typeFilter={(type) => !type.startsWith('paymentasia_')}
      />
    )

    expect(screen.getAllByText('支付宝').length).toBeGreaterThan(0)
    expect(screen.queryAllByText('paymentasia_alipayhk')).toHaveLength(0)
  })
})
