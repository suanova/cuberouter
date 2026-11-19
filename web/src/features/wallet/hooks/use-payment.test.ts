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
import {
  requestPaymentAmount,
  usePayment,
  type PaymentAmountCalculators,
} from './use-payment'

vi.mock('../api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../api')>()
  return {
    ...actual,
    calculateAmount: vi.fn(),
    calculatePaymentAsiaAmount: vi.fn(),
  }
})

/** Every calculator resolves to an unused value unless the test overrides it. */
function amountCalculators(
  overrides: Partial<PaymentAmountCalculators> = {}
): PaymentAmountCalculators {
  return {
    regular: async () => ({ success: true, data: '1' }),
    stripe: async () => ({ success: true, data: '2' }),
    alipay: async () => ({ success: true, data: '3' }),
    waffo: async () => ({ success: true, data: '4' }),
    waffoPancake: async () => ({ success: true, data: '4' }),
    paymentAsia: async () => ({ success: true, data: '5' }),
    ...overrides,
  }
}

/** 报价接口在启用手续费时额外返回的字段；data 仍是用户实付 base。 */
const feeQuote: AmountResponse = {
  success: true,
  data: '100.00',
  fee: '1.6000',
  fee_rate: 0.016,
  fee_fixed: 0,
  net_amount: '98.4000',
  net_quota: 49_200_000,
}

const expectedFeeInfo = {
  fee: 1.6,
  feeRate: 0.016,
  feeFixed: 0,
  netAmount: 98.4,
  netQuota: 49_200_000,
}

describe('payment amount routing', () => {
  test('uses the dedicated Waffo amount calculator', async () => {
    const calls: string[] = []
    let waffoRequest: AmountRequest | undefined
    const result = await requestPaymentAmount(
      120,
      PAYMENT_TYPES.WAFFO,
      amountCalculators({
        waffo: async (request) => {
          calls.push(`waffo:${request.amount}`)
          waffoRequest = request
          return { success: true, data: '18.75' }
        },
      })
    )

    expect(result).toEqual({ amount: 18.75 })
    expect(calls).toEqual(['waffo:120'])
    expect(waffoRequest).toEqual({ amount: 120 })
  })

  test('uses the dedicated official Alipay amount calculator', async () => {
    const calls: string[] = []
    const result = await requestPaymentAmount(
      50,
      PAYMENT_TYPES.ALIPAY_OFFICIAL,
      amountCalculators({
        alipay: async (request) => {
          calls.push(`alipay:${request.amount}`)
          return { success: true, data: '365.00' }
        },
      })
    )

    expect(result).toEqual({ amount: 365 })
    expect(calls).toEqual(['alipay:50'])
  })

  test('uses the dedicated PaymentAsia amount calculator', async () => {
    const calls: string[] = []
    let paymentAsiaRequest: AmountRequest | undefined
    const result = await requestPaymentAmount(
      100,
      'paymentasia_alipayhk',
      amountCalculators({
        paymentAsia: async (request) => {
          calls.push(`paymentAsia:${request.amount}`)
          paymentAsiaRequest = request
          return { success: true, data: '108.00' }
        },
      })
    )

    expect(result).toEqual({ amount: 108 })
    expect(calls).toEqual(['paymentAsia:100'])
    // The backend quotes per channel: Octopus is formatted with one decimal.
    expect(paymentAsiaRequest).toEqual({
      amount: 100,
      payment_method: 'paymentasia_alipayhk',
    })
  })
})

describe('PaymentAsia service fee quotes', () => {
  test('parses the service fee returned with the quote', async () => {
    const result = await requestPaymentAmount(
      100,
      'paymentasia_alipayhk',
      amountCalculators({
        paymentAsia: async () => feeQuote,
      })
    )

    expect(result).toEqual({ amount: 100, feeInfo: expectedFeeInfo })
  })

  test('omits fee info for gateways without a service fee', async () => {
    const result = await requestPaymentAmount(
      50,
      PAYMENT_TYPES.ALIPAY_OFFICIAL,
      amountCalculators({
        alipay: async () => ({ success: true, data: '365.00' }),
      })
    )

    expect(result).toEqual({ amount: 365 })
    expect(result.feeInfo).toBeUndefined()
  })

  test('omits fee info when the fee payload is partial or malformed', async () => {
    const payloads: AmountResponse[] = [
      // net_amount missing
      { success: true, data: '100.00', fee: '1.6000' },
      // fee missing
      { success: true, data: '100.00', net_amount: '98.4000' },
      // non-numeric fee
      {
        success: true,
        data: '100.00',
        fee: 'oops',
        net_amount: '98.4000',
      },
      // non-numeric net amount
      {
        success: true,
        data: '100.00',
        fee: '1.6000',
        net_amount: 'oops',
      },
    ]

    for (const payload of payloads) {
      const result = await requestPaymentAmount(
        100,
        'paymentasia_alipayhk',
        amountCalculators({ paymentAsia: async () => payload })
      )
      expect(result).toEqual({ amount: 100 })
    }
  })

  test('returns the backend rejection message when the quote fails', async () => {
    const result = await requestPaymentAmount(
      100,
      'paymentasia_alipayhk',
      amountCalculators({
        paymentAsia: async () => ({
          message: 'error',
          data: '充值金额过低，无法覆盖手续费',
        }),
      })
    )

    expect(result).toEqual({
      amount: 0,
      error: '充值金额过低，无法覆盖手续费',
    })
  })
})

describe('service fee state in the payment hook', () => {
  test('publishes the newest fee quote and clears it for a fee-free gateway', async () => {
    vi.mocked(calculateAmount).mockResolvedValue({
      success: true,
      data: '365.00',
    })
    vi.mocked(calculatePaymentAsiaAmount).mockResolvedValue(feeQuote)

    const { result } = renderHook(() => usePayment())
    expect(result.current.paymentFeeInfo).toBeNull()

    let paymentAsiaAmount = 0
    await act(async () => {
      const quote = await result.current.calculatePaymentAmount(
        100,
        'paymentasia_alipayhk'
      )
      paymentAsiaAmount = quote?.amount ?? 0
    })
    expect(paymentAsiaAmount).toBe(100)
    expect(result.current.paymentFeeInfo).toEqual(expectedFeeInfo)

    await act(async () => {
      await result.current.calculatePaymentAmount(50, PAYMENT_TYPES.ALIPAY)
    })
    expect(result.current.paymentFeeInfo).toBeNull()
  })

  test('clears the previous fee when a later quote fails', async () => {
    vi.mocked(calculatePaymentAsiaAmount)
      .mockResolvedValueOnce(feeQuote)
      .mockRejectedValueOnce(new Error('quote failed'))

    const { result } = renderHook(() => usePayment())

    await act(async () => {
      await result.current.calculatePaymentAmount(100, 'paymentasia_alipayhk')
    })
    expect(result.current.paymentFeeInfo).toEqual(expectedFeeInfo)

    let failedAmount = -1
    let failedError = ''
    await act(async () => {
      const quote = await result.current.calculatePaymentAmount(
        100,
        'paymentasia_alipayhk'
      )
      failedAmount = quote?.amount ?? -1
      failedError = quote?.error ?? ''
    })
    expect(failedAmount).toBe(0)
    expect(failedError).not.toBe('')
    expect(result.current.paymentFeeInfo).toBeNull()
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

    let regularRequest!: Promise<{ amount: number; error?: string } | null>
    act(() => {
      regularRequest = result.current.calculatePaymentAmount(
        100,
        PAYMENT_TYPES.ALIPAY
      )
    })
    let paymentAsiaRequest!: Promise<{ amount: number; error?: string } | null>
    act(() => {
      paymentAsiaRequest = result.current.calculatePaymentAmount(
        100,
        'paymentasia_alipayhk'
      )
    })

    await act(async () => {
      resolvePaymentAsia({ ...feeQuote, data: '108.00' })
    })
    await expect(paymentAsiaRequest).resolves.toMatchObject({ amount: 108 })
    expect(result.current.amount).toBe(108)
    expect(result.current.paymentFeeInfo).toEqual(expectedFeeInfo)

    await act(async () => {
      resolveRegular({ success: true, data: '365.00' })
    })
    await expect(regularRequest).resolves.toBeNull()
    expect(result.current.amount).toBe(108)
    expect(result.current.paymentFeeInfo).toEqual(expectedFeeInfo)
    expect(result.current.calculating).toBe(false)
  })
})
