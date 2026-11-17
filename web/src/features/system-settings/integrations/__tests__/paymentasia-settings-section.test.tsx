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
import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, test, vi } from 'vitest'

import {
  PaymentAsiaSettingsSection,
  type PaymentAsiaSettingsValues,
} from '../paymentasia-settings-section'

vi.mock('../../hooks/use-system-options', () => ({
  useSystemOptions: () => ({
    data: {
      data: [{ key: 'ServerAddress', value: 'https://api.example.com' }],
    },
    isLoading: false,
  }),
}))

const baseValues: PaymentAsiaSettingsValues = {
  PaymentAsiaSandbox: false,
  PaymentAsiaMerchantToken: 'live-token',
  PaymentAsiaSecret: 'live-secret',
  PaymentAsiaSandboxMerchantToken: 'sandbox-token',
  PaymentAsiaSandboxSecret: 'sandbox-secret',
  PaymentAsiaCurrency: 'HKD',
  PaymentAsiaUnitPrice: 1,
  PaymentAsiaMinTopUp: 1,
  PaymentAsiaNotifyUrl: '',
  PaymentAsiaReturnUrl: '',
}

function renderSection() {
  const onValueChange = vi.fn()
  render(
    <PaymentAsiaSettingsSection
      values={baseValues}
      onValueChange={onValueChange}
    />
  )
  return onValueChange
}

describe('PaymentAsiaSettingsSection', () => {
  test('associates every settings label with its control', () => {
    renderSection()

    for (const label of [
      'Sandbox mode',
      'Merchant Token',
      'Merchant Secret',
      'Sandbox Merchant Token',
      'Sandbox Merchant Secret',
      'Currency',
      'Unit price',
      'Minimum top-up quantity',
      'Callback notification URL',
      'Payment return URL',
    ]) {
      expect(screen.getByLabelText(label)).toBeInTheDocument()
    }
  })

  test('toggles the sandbox switch through onValueChange', () => {
    const onValueChange = renderSection()

    fireEvent.click(screen.getByRole('switch', { name: 'Sandbox mode' }))

    expect(onValueChange).toHaveBeenCalledWith('PaymentAsiaSandbox', true)
  })

  test('emits the edited merchant token through onValueChange', () => {
    const onValueChange = renderSection()

    fireEvent.change(screen.getByLabelText('Merchant Token'), {
      target: { value: 'new-token' },
    })

    expect(onValueChange).toHaveBeenCalledWith(
      'PaymentAsiaMerchantToken',
      'new-token'
    )
  })

  test('emits the minimum top-up quantity as a number', () => {
    const onValueChange = renderSection()

    fireEvent.change(screen.getByLabelText('Minimum top-up quantity'), {
      target: { value: '5' },
    })

    expect(onValueChange).toHaveBeenCalledWith('PaymentAsiaMinTopUp', 5)
  })

  test('emits the unit price as a number', () => {
    const onValueChange = renderSection()

    fireEvent.change(screen.getByLabelText('Unit price'), {
      target: { value: '0.13' },
    })

    expect(onValueChange).toHaveBeenCalledWith('PaymentAsiaUnitPrice', 0.13)
  })

  test('emits the selected currency when another currency is picked', async () => {
    const onValueChange = renderSection()

    fireEvent.click(screen.getByRole('combobox', { name: 'Currency' }))
    const usdOption = await screen.findByRole('option', { name: 'USD' })
    // Base UI commits an option only when the press started on it.
    fireEvent.pointerDown(usdOption)
    fireEvent.click(usdOption)

    expect(onValueChange).toHaveBeenCalledWith('PaymentAsiaCurrency', 'USD')
  })
})
