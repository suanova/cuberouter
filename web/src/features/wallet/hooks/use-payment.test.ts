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
import { act, renderHook } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import { calculateAmount, calculatePaymentAsiaAmount } from '../api'
import { PAYMENT_TYPES } from '../constants'
import type { AmountRequest, AmountResponse } from '../types'
import { requestPaymentAmount, usePayment } from './use-payment'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    calculateAmount: vi.fn(),
    calculatePaymentAsiaAmount: vi.fn(),
  }
})

describe('payment amount routing', () => {
  test('uses the dedicated Waffo amount calculator', async () => {
    const calls: string[] = []
    let waffoRequest: AmountRequest | undefined
    const amount = await requestPaymentAmount(120, PAYMENT_TYPES.WAFFO, {
      regular: async () => {
        calls.push('regular')
        return { success: true, data: '1' }
      },
      stripe: async () => {
        calls.push('stripe')
        return { success: true, data: '2' }
      },
      alipay: async (request) => {
        calls.push(`alipay:${request.amount}`)
        return { success: true, data: '3' }
      },
      waffo: async (request) => {
        calls.push(`waffo:${request.amount}`)
        waffoRequest = request
        return { success: true, data: '18.75' }
      },
      waffoPancake: async () => {
        calls.push('pancake')
        return { success: true, data: '4' }
      },
      paymentAsia: async () => {
        calls.push('paymentAsia')
        return { success: true, data: '5' }
      },
    })

    expect(amount).toBe(18.75)
    expect(calls).toEqual(['waffo:120'])
    expect(waffoRequest).toEqual({ amount: 120 })
  })

  test('uses the dedicated official Alipay amount calculator', async () => {
    const calls: string[] = []
    const amount = await requestPaymentAmount(
      50,
      PAYMENT_TYPES.ALIPAY_OFFICIAL,
      {
        regular: async () => {
          calls.push('regular')
          return { success: true, data: '1' }
        },
        stripe: async () => {
          calls.push('stripe')
          return { success: true, data: '2' }
        },
        alipay: async (request) => {
          calls.push(`alipay:${request.amount}`)
          return { success: true, data: '365.00' }
        },
        waffo: async () => {
          calls.push('waffo')
          return { success: true, data: '4' }
        },
        waffoPancake: async () => {
          calls.push('pancake')
          return { success: true, data: '4' }
        },
        paymentAsia: async () => {
          calls.push('paymentAsia')
          return { success: true, data: '5' }
        },
      }
    )

    expect(amount).toBe(365)
    expect(calls).toEqual(['alipay:50'])
  })

  test('uses the dedicated PaymentAsia amount calculator', async () => {
    const calls: string[] = []
    let paymentAsiaRequest: AmountRequest | undefined
    const amount = await requestPaymentAmount(100, 'paymentasia_alipayhk', {
      regular: async () => {
        calls.push('regular')
        return { success: true, data: '1' }
      },
      stripe: async () => {
        calls.push('stripe')
        return { success: true, data: '2' }
      },
      alipay: async () => {
        calls.push('alipay')
        return { success: true, data: '3' }
      },
      waffo: async () => {
        calls.push('waffo')
        return { success: true, data: '4' }
      },
      waffoPancake: async () => {
        calls.push('pancake')
        return { success: true, data: '4' }
      },
      paymentAsia: async (request) => {
        calls.push(`paymentAsia:${request.amount}`)
        paymentAsiaRequest = request
        return { success: true, data: '108.00' }
      },
    })

    expect(amount).toBe(108)
    expect(calls).toEqual(['paymentAsia:100'])
    // The backend quotes per channel: Octopus is formatted with one decimal.
    expect(paymentAsiaRequest).toEqual({
      amount: 100,
      payment_method: 'paymentasia_alipayhk',
    })
  })
})

describe('stale amount requests', () => {
  test('keeps the newest quote when an older request resolves later', async () => {
    let resolveRegular!: (response: AmountResponse) => void
    let resolvePaymentAsia!: (response: AmountResponse) => void
    vi.mocked(calculateAmount).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolveRegular = resolve
        })
    )
    vi.mocked(calculatePaymentAsiaAmount).mockImplementation(
      () =>
        new Promise((resolve) => {
          resolvePaymentAsia = resolve
        })
    )

    const { result } = renderHook(() => usePayment())

    let regularRequest!: Promise<number | null>
    act(() => {
      regularRequest = result.current.calculatePaymentAmount(
        100,
        PAYMENT_TYPES.ALIPAY
      )
    })
    let paymentAsiaRequest!: Promise<number | null>
    act(() => {
      paymentAsiaRequest = result.current.calculatePaymentAmount(
        100,
        'paymentasia_alipayhk'
      )
    })

    await act(async () => {
      resolvePaymentAsia({ success: true, data: '108.00' })
    })
    await expect(paymentAsiaRequest).resolves.toBe(108)
    expect(result.current.amount).toBe(108)

    await act(async () => {
      resolveRegular({ success: true, data: '365.00' })
    })
    await expect(regularRequest).resolves.toBeNull()
    expect(result.current.amount).toBe(108)
    expect(result.current.calculating).toBe(false)
  })
})
