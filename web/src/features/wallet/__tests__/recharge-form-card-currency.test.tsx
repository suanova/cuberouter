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
import { beforeEach, describe, expect, test, vi } from 'vitest'

import {
  DEFAULT_CURRENCY_CONFIG,
  useSystemConfigStore,
} from '@/stores/system-config-store'

import { RechargeFormCard } from '../components/recharge-form-card'
import type { TopupInfo } from '../types'

const topupInfo: TopupInfo = {
  enable_online_topup: true,
  enable_stripe_topup: false,
  min_topup: 1,
  stripe_min_topup: 1,
  pay_methods: [],
  amount_options: [10],
  discount: {},
}

function renderCard(paymentCurrency?: string) {
  return render(
    <RechargeFormCard
      topupInfo={topupInfo}
      presetAmounts={[{ value: 10 }]}
      selectedPreset={null}
      onSelectPreset={vi.fn()}
      topupAmount={10}
      onTopupAmountChange={vi.fn()}
      paymentAmount={10.8}
      calculating={false}
      onPaymentMethodSelect={vi.fn()}
      paymentLoading={null}
      paymentCurrency={paymentCurrency}
      redemptionCode=''
      onRedemptionCodeChange={vi.fn()}
      onRedeem={vi.fn()}
      redeeming={false}
    />
  )
}

beforeEach(() => {
  useSystemConfigStore.setState((state) => ({
    config: { ...state.config, currency: { ...DEFAULT_CURRENCY_CONFIG } },
  }))
})

describe('RechargeFormCard currency symbols', () => {
  // The preset "Pay" estimate is local-currency pricing (Price/Epay), while the
  // live "Amount to pay" quote comes from the selected gateway. A HKD
  // PaymentAsia channel must not relabel the preset estimate as HK$.
  test('keeps the local symbol on preset estimates and uses the gateway symbol on the live quote', () => {
    renderCard('HKD')

    const presetButton = screen.getByText('$10').closest('button')
    expect(presetButton).toHaveTextContent('Pay ¥10')
    expect(presetButton).not.toHaveTextContent('HK$')

    const amountToPay = screen.getByText('Amount to pay:').parentElement
    expect(amountToPay).toHaveTextContent('HK$10.8')
  })

  test('falls back to the local symbol on the live quote without a gateway currency', () => {
    renderCard()

    const amountToPay = screen.getByText('Amount to pay:').parentElement
    expect(amountToPay).toHaveTextContent('¥10.8')
  })

  test('gives multi-character custom currency symbols enough input padding', () => {
    useSystemConfigStore.setState((state) => ({
      config: {
        ...state.config,
        currency: {
          ...DEFAULT_CURRENCY_CONFIG,
          quotaDisplayType: 'CUSTOM',
          customCurrencySymbol: 'HKD',
        },
      },
    }))

    renderCard()

    const input = screen.getByLabelText('Custom Amount')
    expect(input.className).toContain('pl-12')
    expect(input.className).not.toContain('pl-7')
  })
})
