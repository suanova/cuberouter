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
import { render, screen, within } from '@testing-library/react'
import { beforeEach, describe, expect, test } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { PaymentConfirmDialog } from '../components/dialogs/payment-confirm-dialog'
import type { PaymentAsiaFeeInfo, PaymentMethod } from '../types'

const paymentAsiaMethod: PaymentMethod = {
  name: 'AlipayHK',
  type: 'paymentasia_alipayhk',
  currency: 'HKD',
}

const feeInfo: PaymentAsiaFeeInfo = {
  fee: 1.6,
  feeRate: 0.016,
  feeFixed: 0,
  netAmount: 98.4,
  netQuota: 49_200_000,
}

function renderDialog(props: {
  paymentMethod?: PaymentMethod
  feeInfo?: PaymentAsiaFeeInfo
  calculating?: boolean
}): Promise<HTMLElement> {
  render(
    <PaymentConfirmDialog
      open
      onOpenChange={() => undefined}
      onConfirm={() => undefined}
      topupAmount={100}
      paymentAmount={100}
      paymentMethod={props.paymentMethod}
      calculating={props.calculating ?? false}
      processing={false}
      feeInfo={props.feeInfo}
    />
  )
  return screen.findByRole('alertdialog')
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: {
      ...state.config,
      currency: { ...DEFAULT_CURRENCY_CONFIG, quotaDisplayType: 'CNY' },
    },
  }))
})

describe('PaymentAsia service fee notice', () => {
  test('shows the fee rate, fee and credited amount for a PaymentAsia quote', async () => {
    const dialog = await renderDialog({
      paymentMethod: paymentAsiaMethod,
      feeInfo,
    })

    expect(
      within(dialog).getByText(
        'This payment includes a 1.60% service fee (¥1.6); the credited amount will be ¥98.4.'
      )
    ).toBeInTheDocument()
  })

  test('hides the notice for a non-PaymentAsia gateway', async () => {
    const dialog = await renderDialog({
      paymentMethod: { name: 'Alipay', type: 'alipay_official' },
      feeInfo,
    })

    expect(within(dialog).queryByText(/service fee/)).not.toBeInTheDocument()
  })

  test('hides the notice when the PaymentAsia quote carries no fee', async () => {
    const dialog = await renderDialog({ paymentMethod: paymentAsiaMethod })

    expect(within(dialog).queryByText(/service fee/)).not.toBeInTheDocument()
  })

  test('hides the notice while the quote is being recalculated', async () => {
    const dialog = await renderDialog({
      paymentMethod: paymentAsiaMethod,
      feeInfo,
      calculating: true,
    })

    expect(within(dialog).queryByText(/service fee/)).not.toBeInTheDocument()
  })
})
